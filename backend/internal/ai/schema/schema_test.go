package schema

import (
	"errors"
	"strings"
	"testing"
)

var evaluation = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []any{"score", "level", "flags"},
	"properties": map[string]any{
		"score": map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
		"level": map[string]any{"type": "string", "enum": []any{"idea", "mvp_ready"}},
		"flags": map[string]any{
			"type": "array", "minItems": 1,
			"items": map[string]any{
				"type": "object", "required": []any{"text"},
				"properties": map[string]any{"text": map[string]any{"type": "string", "minLength": 3}},
			},
		},
	},
}

func TestValidate(t *testing.T) {
	v, err := Compile("test-evaluation", evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Validate(`{"score": 72, "level": "mvp_ready", "flags": [{"text": "Нет данных о рынке"}]}`); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}
	fenced := "Вот оценка:\n```json\n{\"score\": 10, \"level\": \"idea\", \"flags\": [{\"text\": \"abc\"}]}\n```"
	if _, err := v.Validate(fenced); err != nil {
		t.Fatalf("fenced JSON rejected: %v", err)
	}
	for doc, want := range map[string]string{
		`{"score": 120, "level": "idea", "flags": [{"text": "abc"}]}`:  "/score",
		`{"score": 1, "level": "unicorn", "flags": [{"text": "abc"}]}`: "/level",
		`{"score": 1, "level": "idea", "flags": []}`:                   "/flags",
		`{"score": 1, "level": "idea", "flags": [{"text": "a"}]}`:      "/flags/0/text",
		`{"score": 1, "level": "idea", "flags": [], "extra": true}`:    "",
		`not json at all`: "not valid JSON",
	} {
		_, err := v.Validate(doc)
		if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", doc, err)
		}
	}
	again, _ := Compile("test-evaluation", nil)
	if again != v {
		t.Fatal("compiled schemas are cached by name")
	}
}

func TestForDecoding(t *testing.T) {
	out := ForDecoding(evaluation)
	raw := func(m any) map[string]any { return m.(map[string]any) }
	score := raw(raw(out["properties"])["score"])
	if _, ok := score["minimum"]; ok {
		t.Fatal("minimum must be dropped")
	}
	flags := raw(raw(out["properties"])["flags"])
	if _, ok := flags["minItems"]; ok {
		t.Fatal("minItems must be dropped")
	}
	item := raw(flags["items"])
	if item["additionalProperties"] != false {
		t.Fatal("nested objects must be closed")
	}
	if _, ok := raw(raw(item["properties"])["text"])["minLength"]; ok {
		t.Fatal("minLength must be dropped")
	}
	if _, ok := raw(raw(evaluation["properties"])["score"])["minimum"]; !ok {
		t.Fatal("the original schema must stay intact")
	}
}
