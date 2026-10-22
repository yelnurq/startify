package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
)

type recorded struct {
	path string
	auth string
	body map[string]any
}

func server(t *testing.T, handle func(w http.ResponseWriter, rec recorded)) (*Provider, *[]recorded) {
	t.Helper()
	var calls []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		rec := recorded{path: r.URL.Path, auth: r.Header.Get("Authorization"), body: body}
		calls = append(calls, rec)
		handle(w, rec)
	}))
	t.Cleanup(srv.Close)
	p := New(Options{BaseURL: srv.URL + "/v1/", APIKey: "local-key", Model: "qwen3:14b", EmbeddingModel: "bge-m3", Local: true})
	return p, &calls
}

func TestCompleteWithSchema(t *testing.T) {
	rejected := false
	p, calls := server(t, func(w http.ResponseWriter, rec recorded) {
		// The first request carries response_format, which this server rejects.
		if rec.body["response_format"] != nil && !rejected {
			rejected = true
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"response_format is not supported"}}`))
			return
		}
		_, _ = fmt.Fprint(w, `{"model":"qwen3:14b","choices":[{"message":{"content":"{\"ok\":true}"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":120,"completion_tokens":8,"prompt_tokens_details":{"cached_tokens":64}}}`)
	})
	res, err := p.Complete(context.Background(), llm.CompletionRequest{
		Model: "qwen3:14b", System: "Answer in JSON", MaxTokens: 300,
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: "Проверка"}},
		JSONSchema: &llm.Schema{Name: "check", JSON: map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}}}},
		Metadata:   llm.RequestMeta{TenantID: uuid.New(), UserID: uuid.New()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != `{"ok":true}` || res.Usage.InputTokens != 120 || res.Usage.CacheReadTokens != 64 || res.StopReason != llm.StopEnd {
		t.Fatalf("result %+v", res)
	}
	if len(*calls) != 2 || (*calls)[1].body["response_format"] != nil {
		t.Fatalf("expected a retry without response_format: %+v", *calls)
	}
	first := (*calls)[0]
	msgs := first.body["messages"].([]any)
	if first.path != "/v1/chat/completions" || first.auth != "Bearer local-key" || len(msgs) != 2 ||
		msgs[0].(map[string]any)["role"] != "system" || first.body["user"] == "" {
		t.Fatalf("request %+v", first)
	}
	format := first.body["response_format"].(map[string]any)["json_schema"].(map[string]any)
	if format["name"] != "check" || format["schema"].(map[string]any)["additionalProperties"] != false {
		t.Fatalf("format %+v", format)
	}
}

func TestStream(t *testing.T) {
	p, _ := server(t, func(w http.ResponseWriter, rec recorded) {
		if rec.body["stream"] != true || rec.body["stream_options"] == nil {
			t.Errorf("not a streaming request: %+v", rec.body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{
			`{"model":"qwen3:14b","choices":[{"delta":{"role":"assistant","content":""}}]}`,
			`{"choices":[{"delta":{"content":"Сәлем, "}}]}`,
			`{"choices":[{"delta":{"content":"әлем!"},"finish_reason":"stop"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":9,"completion_tokens":4}}`,
		} {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	})
	events, err := p.Stream(context.Background(), llm.CompletionRequest{Model: "qwen3:14b", Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	var done *llm.CompletionResponse
	for ev := range events {
		if ev.Err != nil {
			t.Fatal(ev.Err)
		}
		text.WriteString(ev.Delta)
		if ev.Done != nil {
			done = ev.Done
		}
	}
	if text.String() != "Сәлем, әлем!" || done == nil || done.Usage.OutputTokens != 4 || done.Text != text.String() {
		t.Fatalf("got %q %+v", text.String(), done)
	}
}

func TestErrorsAndRefusals(t *testing.T) {
	status := http.StatusTooManyRequests
	p, _ := server(t, func(w http.ResponseWriter, _ recorded) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"","refusal":"I can't help with that."},"finish_reason":"stop"}]}`)
	})
	req := llm.CompletionRequest{Model: "m", Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}}
	if _, err := p.Complete(context.Background(), req); !errors.Is(err, llm.ErrUnavailable) {
		t.Fatalf("429: %v", err)
	}
	status = http.StatusNotFound
	if _, err := p.Complete(context.Background(), req); !errors.Is(err, llm.ErrBadRequest) {
		t.Fatalf("404: %v", err)
	}
	status = http.StatusOK
	if _, err := p.Complete(context.Background(), req); !errors.Is(err, llm.ErrRefused) {
		t.Fatalf("refusal: %v", err)
	}
	unreachable := New(Options{BaseURL: "http://127.0.0.1:1/v1", Model: "m"})
	if _, err := unreachable.Complete(context.Background(), req); !errors.Is(err, llm.ErrUnavailable) {
		t.Fatalf("unreachable: %v", err)
	}
}

func TestEmbed(t *testing.T) {
	p, calls := server(t, func(w http.ResponseWriter, _ recorded) {
		// Out of order on purpose: the index field decides.
		_, _ = fmt.Fprint(w, `{"model":"bge-m3:latest","data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}]}`)
	})
	vectors, model, err := p.Embed(context.Background(), []string{"a", "b"})
	if err != nil || model != "bge-m3" || vectors[0][0] != 1 || vectors[1][1] != 1 {
		t.Fatalf("%v %s %v", err, model, vectors)
	}
	if (*calls)[0].path != "/v1/embeddings" || (*calls)[0].body["model"] != "bge-m3" {
		t.Fatalf("%+v", (*calls)[0])
	}
	if _, _, err := New(Options{BaseURL: "http://x", Model: "m"}).Embed(context.Background(), []string{"a"}); !errors.Is(err, llm.ErrNotSupported) {
		t.Fatalf("no embedding model: %v", err)
	}
}
