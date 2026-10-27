package files

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

// Verdict is the outcome of a virus scan.
type Verdict struct {
	Clean     bool
	Signature string
}

// Scanner checks uploads for malware.
type Scanner interface {
	Scan(ctx context.Context, data []byte) (Verdict, error)
}

// ClamAV scans through clamd's INSTREAM command over TCP.
type ClamAV struct {
	Addr    string
	Timeout time.Duration
}

const clamChunk = 64 << 10

// Scan implements Scanner.
func (c *ClamAV) Scan(ctx context.Context, data []byte) (Verdict, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return Verdict{}, fmt.Errorf("clamav: dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(c.Timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return Verdict{}, fmt.Errorf("clamav: deadline: %w", err)
	}

	w := bufio.NewWriter(conn)
	if _, err := w.WriteString("zINSTREAM\x00"); err != nil {
		return Verdict{}, fmt.Errorf("clamav: command: %w", err)
	}
	var size [4]byte
	for start := 0; start < len(data); start += clamChunk {
		chunk := data[start:min(start+clamChunk, len(data))]
		binary.BigEndian.PutUint32(size[:], uint32(len(chunk))) //nolint:gosec // chunk is at most 64 KiB
		if _, err := w.Write(size[:]); err != nil {
			return Verdict{}, fmt.Errorf("clamav: stream: %w", err)
		}
		if _, err := w.Write(chunk); err != nil {
			return Verdict{}, fmt.Errorf("clamav: stream: %w", err)
		}
	}
	binary.BigEndian.PutUint32(size[:], 0)
	if _, err := w.Write(size[:]); err != nil {
		return Verdict{}, fmt.Errorf("clamav: stream end: %w", err)
	}
	if err := w.Flush(); err != nil {
		return Verdict{}, fmt.Errorf("clamav: flush: %w", err)
	}

	reply, err := bufio.NewReader(conn).ReadString(0)
	if err != nil {
		return Verdict{}, fmt.Errorf("clamav: reply: %w", err)
	}
	return parseClamReply(strings.TrimRight(reply, "\x00\n"))
}

func parseClamReply(reply string) (Verdict, error) {
	body := strings.TrimPrefix(reply, "stream: ")
	switch {
	case body == "OK":
		return Verdict{Clean: true}, nil
	case strings.HasSuffix(body, " FOUND"):
		return Verdict{Signature: strings.TrimSuffix(body, " FOUND")}, nil
	default:
		return Verdict{}, fmt.Errorf("clamav: unexpected reply %q", reply)
	}
}
