package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SSE writes a server-sent event stream.
type SSE struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

// NewSSE starts an event stream. Proxies must not buffer it (X-Accel-Buffering).
func NewSSE(w http.ResponseWriter) *SSE {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	s := &SSE{w: w, rc: http.NewResponseController(w)}
	_ = s.rc.Flush()
	return s
}

// Event sends one event with a JSON payload and flushes it to the client.
func (s *SSE) Event(name string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse: encode %s: %w", name, err)
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", name, raw); err != nil {
		return err
	}
	return s.rc.Flush()
}

// Comment sends a keep-alive comment line.
func (s *SSE) Comment(text string) error {
	if _, err := fmt.Fprintf(s.w, ": %s\n\n", strings.ReplaceAll(text, "\n", " ")); err != nil {
		return err
	}
	return s.rc.Flush()
}
