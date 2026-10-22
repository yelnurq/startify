package files

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/gif" // registers the GIF decoder
	"image/jpeg"
	_ "image/png" // registers the PNG decoder

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// Avatar limits.
const (
	AvatarSize      = 512
	avatarMinSide   = 64
	avatarMaxPixels = 40_000_000
)

// ProcessAvatar turns an uploaded photo into a square JPEG of at most
// AvatarSize pixels: it applies the EXIF orientation of phone photos, crops
// the centre, scales it and re-encodes it, which also drops all metadata
// (location, camera) from the published file.
func ProcessAvatar(data []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.New(apperr.KindValidation, "files.unsupported_type",
			"upload a JPEG, PNG, WebP or GIF image")
	}
	if cfg.Width*cfg.Height > avatarMaxPixels {
		return nil, apperr.New(apperr.KindValidation, "files.image_too_large", "the image has too many pixels").
			WithParams(map[string]any{"max_megapixels": avatarMaxPixels / 1_000_000})
	}
	if min(cfg.Width, cfg.Height) < avatarMinSide {
		return nil, apperr.New(apperr.KindValidation, "files.image_too_small", "the image is too small").
			WithParams(map[string]any{"min_pixels": avatarMinSide})
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.New(apperr.KindValidation, "files.image_invalid", "the image is damaged")
	}

	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	out := min(side, AvatarSize)

	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	xdraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, image.Rect(x0, y0, x0+side, y0+side), xdraw.Over, nil)

	orientation := 1
	if format == "jpeg" {
		orientation = jpegOrientation(data)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, orient(dst, orientation), &jpeg.Options{Quality: 86}); err != nil {
		return nil, apperr.Internal(err)
	}
	return buf.Bytes(), nil
}

// orient applies an EXIF orientation (1-8) to a square image.
func orient(src *image.RGBA, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	n := src.Bounds().Dx()
	dst := image.NewRGBA(src.Bounds())
	last := n - 1
	for y := range n {
		for x := range n {
			var sx, sy int
			switch orientation {
			case 2:
				sx, sy = last-x, y
			case 3:
				sx, sy = last-x, last-y
			case 4:
				sx, sy = x, last-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, last-x
			case 7:
				sx, sy = last-y, last-x
			case 8:
				sx, sy = last-y, x
			}
			dst.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst
}

// jpegOrientation reads the EXIF orientation tag of a JPEG; 1 when absent.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF:
			i++
			continue
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8):
			i += 2
			continue
		case marker == 0xDA || marker == 0xD9:
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		if size < 2 || i+2+size > len(data) {
			return 1
		}
		segment := data[i+4 : i+2+size]
		if marker == 0xE1 && len(segment) >= 6 && string(segment[:6]) == "Exif\x00\x00" {
			return tiffOrientation(segment[6:])
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	var header [8]byte
	copy(header[:], t)
	if order.Uint16(header[2:4]) != 42 {
		return 1
	}
	ifd := int(order.Uint32(header[4:8]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	entries := int(order.Uint16(t[ifd:]))
	for k := range entries {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if order.Uint16(t[e:]) == 0x0112 {
			if v := int(order.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// Cover limits: covers are 16:9.
const (
	CoverWidth    = 1600
	CoverHeight   = 900
	coverMinWidth = 480
)

// ProcessCover turns an uploaded picture into a 16:9 JPEG of at most
// CoverWidth×CoverHeight: it applies the EXIF orientation, crops the centre,
// scales it and re-encodes it without metadata.
func ProcessCover(data []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.New(apperr.KindValidation, "files.unsupported_type",
			"upload a JPEG, PNG, WebP or GIF image")
	}
	if cfg.Width*cfg.Height > avatarMaxPixels {
		return nil, apperr.New(apperr.KindValidation, "files.image_too_large", "the image has too many pixels").
			WithParams(map[string]any{"max_megapixels": avatarMaxPixels / 1_000_000})
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.New(apperr.KindValidation, "files.image_invalid", "the image is damaged")
	}
	src := image.NewRGBA(img.Bounds())
	xdraw.Draw(src, src.Bounds(), img, img.Bounds().Min, xdraw.Src)
	oriented := src
	if format == "jpeg" {
		oriented = orientRect(src, jpegOrientation(data))
	}
	b := oriented.Bounds()
	if max(b.Dx(), b.Dy()) < coverMinWidth {
		return nil, apperr.New(apperr.KindValidation, "files.image_too_small", "the image is too small").
			WithParams(map[string]any{"min_pixels": coverMinWidth})
	}
	// The largest 16:9 rectangle in the centre.
	w, h := b.Dx(), b.Dx()*CoverHeight/CoverWidth
	if h > b.Dy() {
		w, h = b.Dy()*CoverWidth/CoverHeight, b.Dy()
	}
	x0, y0 := b.Min.X+(b.Dx()-w)/2, b.Min.Y+(b.Dy()-h)/2
	outW := min(w, CoverWidth)
	outH := outW * CoverHeight / CoverWidth
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	xdraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), oriented, image.Rect(x0, y0, x0+w, y0+h), xdraw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, apperr.Internal(err)
	}
	return buf.Bytes(), nil
}

// orientRect applies an EXIF orientation (1-8) to an image of any shape;
// orientations 5-8 swap width and height.
func orientRect(src *image.RGBA, orientation int) *image.RGBA {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		for x := range dw {
			var sx, sy int
			switch orientation {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			dst.SetRGBA(x, y, src.RGBAAt(src.Bounds().Min.X+sx, src.Bounds().Min.Y+sy))
		}
	}
	return dst
}
