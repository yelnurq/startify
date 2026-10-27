package incubator

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
)

// While the model is thinking, the stream sends keep-alive comments so that a
// proxy with an idle timeout does not close it (Н-22).
func TestRelayKeepsAQuietStreamAlive(t *testing.T) {
	rec := httptest.NewRecorder()
	sse := httpx.NewSSE(rec)
	events := make(chan ai.Event)
	go func() {
		defer close(events)
		time.Sleep(80 * time.Millisecond) // the model thinks
		events <- ai.Event{Delta: "Сәлем"}
		events <- ai.Event{Result: &ai.Result{Text: "Сәлем"}}
	}()
	stopped := false
	out := relay(sse, events, 15*time.Millisecond, false, func() { stopped = true })
	body := rec.Body.String()
	if out.text != "Сәлем" || out.result == nil || out.gone || stopped {
		t.Fatalf("relayed %+v, stopped %v", out, stopped)
	}
	keepAlive := strings.Index(body, ": keep-alive\n\n")
	delta := strings.Index(body, "event: delta\n")
	if keepAlive < 0 || delta < keepAlive {
		t.Fatalf("no keep-alive before the first words:\n%s", body)
	}
}
