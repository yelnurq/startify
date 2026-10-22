package mock

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/schema"
)

func TestRepliesAreDeterministicAndLocalised(t *testing.T) {
	p := &Provider{}
	req := llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Моя проблема — очереди в столовой"}},
		Metadata: llm.RequestMeta{Feature: llm.FeatureMentor, Locale: "kk"},
	}
	a, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := p.Complete(context.Background(), req)
	if a.Text != b.Text || a.Text == "" || a.Usage.OutputTokens == 0 {
		t.Fatalf("not deterministic: %q vs %q", a.Text, b.Text)
	}
	if !strings.ContainsAny(a.Text, "әғқңөұүһі") {
		t.Fatalf("expected Kazakh: %q", a.Text)
	}

	events, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var streamed strings.Builder
	var done *llm.CompletionResponse
	for ev := range events {
		streamed.WriteString(ev.Delta)
		if ev.Done != nil {
			done = ev.Done
		}
	}
	if streamed.String() != a.Text || done == nil || done.Text != a.Text {
		t.Fatalf("stream %q differs from %q", streamed.String(), a.Text)
	}
}

func TestStreamStopsWhenCancelled(_ *testing.T) {
	p := &Provider{ChunkDelay: 50_000_000}
	ctx, cancel := context.WithCancel(context.Background())
	events, _ := p.Stream(ctx, llm.CompletionRequest{Metadata: llm.RequestMeta{Locale: "ru"}})
	<-events
	cancel()
	for range events {
	}
}

func TestStructuredAnswersMatchTheSchema(t *testing.T) {
	doc := map[string]any{
		"type": "object", "required": []any{"score", "level", "items", "note"},
		"properties": map[string]any{
			"score": map[string]any{"type": "integer", "minimum": 0, "maximum": 10},
			"level": map[string]any{"type": "string", "enum": []any{"idea", "mvp_ready"}},
			"items": map[string]any{"type": "array", "minItems": 3, "items": map[string]any{"$ref": "#/$defs/item"}},
			"note":  map[string]any{"type": []any{"string", "null"}, "minLength": 20},
		},
		"$defs": map[string]any{"item": map[string]any{
			"type": "object", "required": []any{"title", "done"},
			"properties": map[string]any{"title": map[string]any{"type": "string"}, "done": map[string]any{"type": "boolean"}},
		}},
	}
	v, err := schema.Compile("mock-test", doc)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{}
	for _, text := range []string{"a", "b", "c"} {
		res, err := p.Complete(context.Background(), llm.CompletionRequest{
			JSONSchema: &llm.Schema{Name: "mock-test", JSON: doc},
			Messages:   []llm.Message{{Role: llm.RoleUser, Content: text}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.Validate(res.Text); err != nil {
			t.Fatalf("%s: %v", res.Text, err)
		}
	}
}

func TestRegisteredGenerator(t *testing.T) {
	Register(llm.FeatureSummary, func(req llm.CompletionRequest) string { return "summary of " + LastUserMessage(req) })
	t.Cleanup(func() { Register(llm.FeatureSummary, nil) })
	res, _ := (&Provider{}).Complete(context.Background(), llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}, Metadata: llm.RequestMeta{Feature: llm.FeatureSummary},
	})
	if res.Text != "summary of x" {
		t.Fatal(res.Text)
	}
}

func cosine(a, b []float32) float64 {
	var dot float64
	for i := range a {
		dot += float64(a[i] * b[i])
	}
	return dot
}

func TestEmbeddingsPlaceRelatedTextsNearby(t *testing.T) {
	vectors, model, err := (&Provider{}).Embed(context.Background(), []string{
		"Заявки на конкурс стартап-проектов принимаются до 1 декабря",
		"До какого числа принимаются заявки на конкурс?",
		"Столовая работает с 8 утра до 6 вечера",
	})
	if err != nil || model != EmbeddingModel || len(vectors[0]) != dims {
		t.Fatalf("%v %s", err, model)
	}
	if cosine(vectors[0], vectors[1]) <= cosine(vectors[0], vectors[2]) {
		t.Fatal("related texts must be closer")
	}
	raw, _ := json.Marshal(vectors[0])
	if strings.Contains(string(raw), "NaN") {
		t.Fatal("NaN in vector")
	}
}
