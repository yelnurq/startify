package anthropic

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

type capture struct {
	body map[string]any
	beta string
}

func fake(t *testing.T, respond func(w http.ResponseWriter)) (*Provider, *capture) {
	t.Helper()
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("unexpected request %s %v", r.URL.Path, r.Header)
		}
		c.beta = r.Header.Get("Anthropic-Beta")
		_ = json.NewDecoder(r.Body).Decode(&c.body)
		w.Header().Set("Content-Type", "application/json")
		respond(w)
	}))
	t.Cleanup(srv.Close)
	return New(Options{APIKey: "test-key", BaseURL: srv.URL, ServerFallbacks: true}), c
}

func TestCompleteBuildsTheRequest(t *testing.T) {
	p, c := fake(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"msg_01","type":"message","role":"assistant","model":"claude-opus-5-5",
			"content":[{"type":"thinking","thinking":"","signature":"sig"},{"type":"text","text":"{\"score\": 71}"}],
			"stop_reason":"end_turn","stop_sequence":null,
			"usage":{"input_tokens":1200,"output_tokens":40,"cache_creation_input_tokens":1000,"cache_read_input_tokens":0,
			  "iterations":[{"type":"message","model":"claude-opus-5-5","input_tokens":1200,"output_tokens":0},
			                {"type":"fallback_message","model":"claude-opus-4-8","input_tokens":1150,"output_tokens":40}]}}`)
	})
	temp := float32(0.2)
	res, err := p.Complete(context.Background(), llm.CompletionRequest{
		Model: "claude-opus-5-5", System: "You evaluate projects.", MaxTokens: 2000, Effort: llm.EffortHigh,
		Temperature: &temp,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Part one"},
			{Role: llm.RoleUser, Content: "Part two"},
			{Role: llm.RoleAssistant, Content: "Earlier answer"},
			{Role: llm.RoleUser, Content: "Evaluate"},
		},
		JSONSchema: &llm.Schema{Name: "evaluation", JSON: map[string]any{
			"type": "object", "required": []any{"score"},
			"properties": map[string]any{"score": map[string]any{"type": "integer", "minimum": 0, "maximum": 100}},
		}},
		Metadata: llm.RequestMeta{TenantID: uuid.New(), UserID: uuid.New()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != `{"score": 71}` || res.Model != "claude-opus-5-5" || res.StopReason != llm.StopEnd {
		t.Fatalf("result %+v", res)
	}
	// Every attempt of the fallback chain is billed.
	if res.Usage.InputTokens != 2350 || res.Usage.OutputTokens != 40 {
		t.Fatalf("usage %+v", res.Usage)
	}

	b := c.body
	if _, ok := b["temperature"]; ok {
		t.Fatal("current Claude models reject sampling parameters")
	}
	if b["fallbacks"] != "default" || !strings.Contains(c.beta, "server-side-fallback-2026-07-01") {
		t.Fatalf("server fallbacks: %v / %q", b["fallbacks"], c.beta)
	}
	system := b["system"].([]any)[0].(map[string]any)
	if system["text"] != "You evaluate projects." || system["cache_control"].(map[string]any)["type"] != "ephemeral" {
		t.Fatalf("system %+v", system)
	}
	msgs := b["messages"].([]any)
	if len(msgs) != 3 || len(msgs[0].(map[string]any)["content"].([]any)) != 2 {
		t.Fatalf("consecutive user turns must merge: %+v", msgs)
	}
	out := b["output_config"].(map[string]any)
	score := out["format"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["score"].(map[string]any)
	if out["effort"] != "high" || score["minimum"] != nil || score["type"] != "integer" {
		t.Fatalf("output_config %+v", out)
	}
	if b["metadata"].(map[string]any)["user_id"] == "" {
		t.Fatal("no opaque user id")
	}
}

func TestSonnetGetsNoServerFallback(t *testing.T) {
	p, c := fake(t, func(w http.ResponseWriter) {
		_, _ = fmt.Fprint(w, `{"id":"m","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"ok"}],
			"stop_reason":"max_tokens","usage":{"input_tokens":5,"output_tokens":7}}`)
	})
	res, err := p.Complete(context.Background(), llm.CompletionRequest{
		Model: "claude-sonnet-5-5", MaxTokens: 10,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}},
	})
	if err != nil || res.StopReason != llm.StopMaxTokens {
		t.Fatalf("%+v %v", res, err)
	}
	if _, ok := c.body["fallbacks"]; ok || c.body["system"] != nil {
		t.Fatalf("unexpected fields: %+v", c.body)
	}
}

func TestRefusalAndErrors(t *testing.T) {
	status := http.StatusOK
	p, _ := fake(t, func(w http.ResponseWriter) {
		if status != http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],
			"stop_reason":"refusal","stop_details":{"type":"refusal","category":"bio","explanation":"declined"},
			"usage":{"input_tokens":30,"output_tokens":0}}`)
	})
	req := llm.CompletionRequest{Model: "claude-opus-5-5", MaxTokens: 10, Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}}
	res, err := p.Complete(context.Background(), req)
	if !errors.Is(err, llm.ErrRefused) || res == nil || res.Usage.InputTokens != 30 {
		t.Fatalf("refusal: %+v %v", res, err)
	}
	status = http.StatusBadRequest
	if _, err := p.Complete(context.Background(), req); !errors.Is(err, llm.ErrBadRequest) {
		t.Fatalf("400: %v", err)
	}
	if _, _, err := p.Embed(context.Background(), []string{"x"}); !errors.Is(err, llm.ErrNotSupported) {
		t.Fatal(err)
	}
}

func TestStream(t *testing.T) {
	p, c := fake(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range []struct{ name, data string }{
			{"message_start", `{"type":"message_start","message":{"id":"m","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[],"stop_reason":null,"usage":{"input_tokens":25,"output_tokens":1}}}`},
			{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
			{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Қош "}}`},
			{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"келдіңіз!"}}`},
			{"content_block_stop", `{"type":"content_block_stop","index":0}`},
			{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":9}}`},
			{"message_stop", `{"type":"message_stop"}`},
		} {
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.name, ev.data)
		}
	})
	events, err := p.Stream(context.Background(), llm.CompletionRequest{
		Model: "claude-sonnet-5-5", MaxTokens: 100,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Сәлем"}},
	})
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
	if text.String() != "Қош келдіңіз!" || done == nil || done.Text != text.String() || done.Usage.OutputTokens != 9 || done.Usage.InputTokens != 25 {
		t.Fatalf("stream %q %+v", text.String(), done)
	}
	if c.body["stream"] != true {
		t.Fatal("not streamed")
	}
}
