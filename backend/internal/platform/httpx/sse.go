package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// sseWriteTimeout bounds how long one event may take to reach the client.
const sseWriteTimeout = time.Minute

// SSE writes a server-sent event stream.
type SSE struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

// NewSSE starts an event stream. Proxies must not buffer it (X-Accel-Buffering).
//
// The request body has been read by now, and the stream outlives the
// server's timeouts on purpose: the read deadline is lifted (when it passes,
// net/http cancels the request context), and every event gets its own write
// deadline instead of one for the whole response.
func NewSSE(w http.ResponseWriter) *SSE {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	s := &SSE{w: w, rc: http.NewResponseController(w)}
	_ = s.rc.SetReadDeadline(time.Time{})
	s.extend()
	w.WriteHeader(http.StatusOK)
	_ = s.rc.Flush()
	return s
}

func (s *SSE) extend() { _ = s.rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout)) }

// Event sends one event with a JSON payload and flushes it to the client.
func (s *SSE) Event(name string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse: encode %s: %w", name, err)
	}
	s.extend()
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", name, raw); err != nil {
		return err
	}
	return s.rc.Flush()
}

// Comment sends a keep-alive comment line.
func (s *SSE) Comment(text string) error {
	s.extend()
	if _, err := fmt.Fprintf(s.w, ": %s\n\n", strings.ReplaceAll(text, "\n", " ")); err != nil {
		return err
	}
	return s.rc.Flush()
}
