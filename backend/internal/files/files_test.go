package files

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// exifJPEG wraps a JPEG with an APP1 segment carrying the orientation.
func exifJPEG(t *testing.T, img image.Image, orientation uint16, order binary.ByteOrder) []byte {
	t.Helper()
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, img, nil); err != nil {
		t.Fatal(err)
	}
	tiff := make([]byte, 26)
	if order == binary.LittleEndian {
		copy(tiff, "II")
	} else {
		copy(tiff, "MM")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)
	order.PutUint16(tiff[8:], 1)
	order.PutUint16(tiff[10:], 0x0112)
	order.PutUint16(tiff[12:], 3)
	order.PutUint32(tiff[14:], 1)
	order.PutUint16(tiff[18:], orientation)
	segment := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(segment)+2))
	out := append([]byte{0xFF, 0xD8}, app1...)
	out = append(out, segment...)
	return append(out, plain.Bytes()[2:]...)
}

// marked is a square image with a red top-left quadrant.
func marked(n int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			c := color.RGBA{B: 255, A: 255}
			if x < n/2 && y < n/2 {
				c = color.RGBA{R: 255, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func redAt(img image.Image, x, y int) bool {
	r, _, b, _ := img.At(x, y).RGBA()
	return r > 0xA000 && b < 0x6000
}

func TestOrientationIsReadAndApplied(t *testing.T) {
	src := marked(200)
	for orientation, corner := range map[uint16][2]int{1: {20, 20}, 3: {180, 180}, 6: {180, 20}, 8: {20, 180}, 2: {180, 20}} {
		for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
			data := exifJPEG(t, src, orientation, order)
			if got := jpegOrientation(data); got != int(orientation) {
				t.Fatalf("orientation %d (%v): read %d", orientation, order, got)
			}
			out, err := ProcessAvatar(context.Background(), data)
			if err != nil {
				t.Fatal(err)
			}
			img, err := jpeg.Decode(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			if !redAt(img, corner[0], corner[1]) {
				t.Errorf("orientation %d: red quadrant not at %v", orientation, corner)
			}
		}
	}
	if jpegOrientation([]byte("not a jpeg")) != 1 || jpegOrientation([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF}) != 1 {
		t.Fatal("garbage must yield orientation 1")
	}
}

func TestProcessAvatar(t *testing.T) {
	wide := image.NewRGBA(image.Rect(0, 0, 1200, 400))
	var buf bytes.Buffer
	if err := png.Encode(&buf, wide); err != nil {
		t.Fatal(err)
	}
	out, err := ProcessAvatar(context.Background(), buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "jpeg" || cfg.Width != 400 || cfg.Height != 400 {
		t.Fatalf("got %s %dx%d %v", format, cfg.Width, cfg.Height, err)
	}

	buf.Reset()
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 40)))
	if _, err := ProcessAvatar(context.Background(), buf.Bytes()); !apperr.Is(err, "files.image_too_small") {
		t.Fatalf("small image: %v", err)
	}
	if _, err := ProcessAvatar(context.Background(), []byte("<svg onload=alert(1)>")); !apperr.Is(err, "files.unsupported_type") {
		t.Fatalf("svg: %v", err)
	}
	// A header promising a gigantic image is refused before decoding.
	ihdr := []byte("IHDR\x00\x01\x00\x00\x00\x01\x00\x00\x08\x02\x00\x00\x00")
	bomb := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d"), ihdr...)
	bomb = binary.BigEndian.AppendUint32(bomb, crc32.ChecksumIEEE(ihdr))
	if _, err := ProcessAvatar(context.Background(), bomb); !apperr.Is(err, "files.image_too_large") {
		t.Fatalf("bomb: %v", err)
	}
}

// fakeClamd answers one INSTREAM command with reply after reading the stream.
func fakeClamd(t *testing.T, reply string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		r := bufio.NewReader(conn)
		if cmd, err := r.ReadString(0); err != nil || cmd != "zINSTREAM\x00" {
			return
		}
		for {
			var size [4]byte
			if _, err := io.ReadFull(r, size[:]); err != nil {
				return
			}
			n := binary.BigEndian.Uint32(size[:])
			if n == 0 {
				break
			}
			if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
				return
			}
		}
		_, _ = conn.Write([]byte(reply + "\x00"))
	}()
	return ln.Addr().String()
}

func TestClamAV(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 200_000)
	v, err := (&ClamAV{Addr: fakeClamd(t, "stream: OK"), Timeout: 5 * time.Second}).Scan(context.Background(), data)
	if err != nil || !v.Clean {
		t.Fatalf("clean: %+v %v", v, err)
	}
	v, err = (&ClamAV{Addr: fakeClamd(t, "stream: Win.Test.EICAR_HDB-1 FOUND"), Timeout: 5 * time.Second}).Scan(context.Background(), data)
	if err != nil || v.Clean || v.Signature != "Win.Test.EICAR_HDB-1" {
		t.Fatalf("infected: %+v %v", v, err)
	}
	if _, err := (&ClamAV{Addr: fakeClamd(t, "INSTREAM size limit exceeded. ERROR"), Timeout: 5 * time.Second}).Scan(context.Background(), data); err == nil {
		t.Fatal("error reply must fail")
	}
	if _, err := (&ClamAV{Addr: "127.0.0.1:1", Timeout: time.Second}).Scan(context.Background(), data); err == nil {
		t.Fatal("unreachable scanner must fail")
	}
}

type stubScanner struct {
	v   Verdict
	err error
}

func (s stubScanner) Scan(context.Context, []byte) (Verdict, error) { return s.v, s.err }

func TestScanModes(t *testing.T) {
	ctx := context.Background()
	s := &Service{scanMode: ScanRequired}
	if _, err := s.scan(ctx, nil); !apperr.Is(err, "files.scanner_unavailable") {
		t.Fatalf("required without scanner: %v", err)
	}
	s = &Service{scanMode: ScanOptional, scanner: stubScanner{err: io.ErrUnexpectedEOF}, log: discardLogger()}
	if status, err := s.scan(ctx, nil); err != nil || status != "not_scanned" {
		t.Fatalf("optional with failing scanner: %s %v", status, err)
	}
	s.scanner = stubScanner{v: Verdict{Signature: "Eicar"}}
	if _, err := s.scan(ctx, nil); !apperr.Is(err, "files.infected") {
		t.Fatalf("infected: %v", err)
	}
	s.scanner = stubScanner{v: Verdict{Clean: true}}
	if status, _ := s.scan(ctx, nil); status != "clean" {
		t.Fatalf("clean: %s", status)
	}
}

func TestReadForm(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("kind", " students ")
	part, _ := w.CreateFormFile("file", `C:\Users\admin\Список "студентов".csv`)
	_, _ = part.Write([]byte("email,full_name\n"))
	_ = w.Close()
	fields, file, err := ReadForm(multipart.NewReader(bytes.NewReader(buf.Bytes()), w.Boundary()), "file", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if fields["kind"] != "students" || file.FileName != "Список студентов.csv" || !strings.HasPrefix(file.ContentType, "text/plain") {
		t.Fatalf("fields %v file %q %q", fields, file.FileName, file.ContentType)
	}
	_, _, err = ReadForm(multipart.NewReader(bytes.NewReader(buf.Bytes()), w.Boundary()), "file", 4)
	if !apperr.Is(err, "files.too_large") {
		t.Fatalf("limit: %v", err)
	}
	_, _, err = ReadForm(multipart.NewReader(bytes.NewReader(buf.Bytes()), w.Boundary()), "missing", 1<<20)
	if !apperr.Is(err, "validation.failed") {
		t.Fatalf("missing file: %v", err)
	}
}

func TestETagMatching(t *testing.T) {
	tag := `"abc"`
	for header, want := range map[string]bool{`"abc"`: true, `W/"abc"`: true, `"x", "abc"`: true, "*": true, `"x"`: false, "": false} {
		if etagMatches(header, tag) != want {
			t.Errorf("%q: want %v", header, want)
		}
	}
}

func TestMemoryStorage(t *testing.T) {
	m := NewMemoryStorage()
	ctx := context.Background()
	if err := m.Put(ctx, "k", []byte("v"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	r, err := m.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := io.ReadAll(r); string(b) != "v" {
		t.Fatalf("got %q", b)
	}
	_ = m.Delete(ctx, "k")
	if _, err := m.Get(ctx, "k"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("deleted object: %v", err)
	}
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestProcessCover(t *testing.T) {
	encode := func(w, h int) []byte {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	for _, tc := range []struct{ w, h, ow, oh int }{
		{3200, 1200, 1600, 900}, // wide: the centre is cropped to 16:9
		{1000, 2000, 1000, 562}, // tall: full width, cropped height
		{800, 450, 800, 450},    // small 16:9 stays as is
	} {
		out, err := ProcessCover(context.Background(), encode(tc.w, tc.h))
		if err != nil {
			t.Fatal(err)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
		if err != nil || format != "jpeg" || cfg.Width != tc.ow || cfg.Height != tc.oh {
			t.Fatalf("%dx%d: got %s %dx%d %v", tc.w, tc.h, format, cfg.Width, cfg.Height, err)
		}
	}
	if _, err := ProcessCover(context.Background(), encode(200, 100)); !apperr.Is(err, "files.image_too_small") {
		t.Fatalf("small: %v", err)
	}
	// Orientations 5-8 swap the sides.
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	src.SetRGBA(3, 0, color.RGBA{R: 255, A: 255})
	rotated := orientRect(src, 6)
	if rotated.Bounds().Dx() != 2 || rotated.Bounds().Dy() != 4 || rotated.RGBAAt(1, 3).R != 255 {
		t.Fatalf("rotated %v", rotated.Bounds())
	}
}

// pngHeader is the start of a PNG that claims a size; nothing follows it.
func pngHeader(w, h uint32) []byte {
	ihdr := append([]byte("IHDR"), binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, w), h)...)
	ihdr = append(ihdr, 8, 2, 0, 0, 0)
	out := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d"), ihdr...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(ihdr))
}

func TestPicturesAreCheckedBeforeDecoding(t *testing.T) {
	ctx := context.Background()
	for _, size := range [][2]uint32{{5000, 5000}, {9000, 100}, {100, 9000}} {
		for name, process := range map[string]func(context.Context, []byte) ([]byte, error){"avatar": ProcessAvatar, "cover": ProcessCover} {
			if _, err := process(ctx, pngHeader(size[0], size[1])); !apperr.Is(err, "files.image_too_large") {
				t.Errorf("%s %dx%d: %v", name, size[0], size[1], err)
			}
		}
	}
	// A 12-megapixel phone photo is within the limit.
	if _, err := ProcessCover(ctx, pngHeader(4032, 3024)); !apperr.Is(err, "files.image_invalid") {
		t.Fatalf("phone photo header: %v", err)
	}
}

func TestCoverOrientationWithoutAFullCopy(t *testing.T) {
	// A 960×540 photo taken with the phone turned: shown upright it is
	// 540×960, and its red corner is the top right.
	img := image.NewRGBA(image.Rect(0, 0, 960, 540))
	for y := range 540 {
		for x := range 960 {
			c := color.RGBA{B: 255, A: 255}
			if x < 480 && y < 270 {
				c = color.RGBA{R: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	out, err := ProcessCover(context.Background(), exifJPEG(t, img, 6, binary.BigEndian))
	if err != nil {
		t.Fatal(err)
	}
	cover, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if b := cover.Bounds(); b.Dx() != 540 || b.Dy() != 303 {
		t.Fatalf("cover %v", b)
	}
	if !redAt(cover, 500, 50) || redAt(cover, 100, 50) || redAt(cover, 500, 250) {
		t.Fatal("the cover is not the upright centre of the photo")
	}
}

func TestDecodingWaitsForASlot(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 100, 100))); err != nil {
		t.Fatal(err)
	}
	for range decodeSlots {
		decoding <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := ProcessAvatar(ctx, buf.Bytes())
	for range decodeSlots {
		<-decoding
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("busy decoder: %v", err)
	}
	if _, err := ProcessAvatar(context.Background(), buf.Bytes()); err != nil {
		t.Fatalf("free decoder: %v", err)
	}
}
