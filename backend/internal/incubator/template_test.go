package incubator

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

func TestDefaultTemplate(t *testing.T) {
	tmpl := Default()
	if len(tmpl.Stages) != 4 || len(tmpl.Keys()) != 14 {
		t.Fatalf("%d stages, %d steps", len(tmpl.Stages), len(tmpl.Keys()))
	}
	for _, key := range tmpl.Keys() {
		s, _ := tmpl.Step(key)
		for _, l := range []string{"kk", "ru", "en"} {
			if s.Title[l] == "" || s.Goal[l] == "" {
				t.Errorf("%s: no %s title or goal", key, l)
			}
		}
		if len(s.Criteria) == 0 {
			t.Errorf("%s: no criteria", key)
		}
		required := false
		for _, f := range s.Fields {
			required = required || f.Required
		}
		if !required {
			t.Errorf("%s: nothing is required to complete it", key)
		}
	}
	if s, _ := tmpl.Step("pitch"); s.Number() != 14 || s.Stage() != "launch" || tmpl.Next("pitch") != "" {
		t.Fatal("pitch is not the last step")
	}
	if tmpl.Next("problem") != "audience" {
		t.Fatal(tmpl.Next("problem"))
	}
}

func raw(t *testing.T, v map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for k, val := range v {
		b, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		out[k] = b
	}
	return out
}

func fieldErrors(t *testing.T, err error) map[string]string {
	t.Helper()
	var e *apperr.Error
	if !errors.As(err, &e) {
		t.Fatalf("not a validation error: %v", err)
	}
	out := map[string]string{}
	for _, f := range e.Fields {
		out[f.Field] = f.Code
	}
	return out
}

func TestSanitizeAcceptsDraftsAndRejectsWrongShapes(t *testing.T) {
	audience, _ := Default().Step("audience")
	values, err := audience.Sanitize(raw(t, map[string]any{
		"segments": "  Студенты\r\nпервого курса\u200b ",
		"personas": []map[string]any{{"name": "Айгерім, 18", "pains": "Нет свободных аудиторий"}, {}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if values["segments"] != "Студенты\nпервого курса" {
		t.Fatalf("%q", values["segments"])
	}
	if rows := values["personas"].([]any); len(rows) != 2 {
		t.Fatalf("draft rows dropped: %v", rows)
	}

	_, err = audience.Sanitize(raw(t, map[string]any{
		"unknown":  "x",
		"segments": 5,
		"personas": []map[string]any{{"name": strings.Repeat("а", 121)}, {"age": "18"}},
	}))
	got := fieldErrors(t, err)
	want := map[string]string{
		"fields.unknown": "invalid", "fields.segments": "invalid",
		"fields.personas.0.name": "maxLength", "fields.personas.1.age": "invalid",
	}
	for k, code := range want {
		if got[k] != code {
			t.Errorf("%s: got %q, want %q (all %v)", k, got[k], code, got)
		}
	}

	market, _ := Default().Step("market_size")
	_, err = market.Sanitize(raw(t, map[string]any{"tam": -1, "sources": make([]string, 11)}))
	got = fieldErrors(t, err)
	if got["fields.tam"] != "min" || got["fields.sources"] != "maxItems" {
		t.Fatal(got)
	}
	competitors, _ := Default().Step("competitors")
	_, err = competitors.Sanitize(raw(t, map[string]any{"list": []map[string]any{{"name": "Kaspi", "kind": "partner"}}}))
	if got = fieldErrors(t, err); got["fields.list.0.kind"] != "invalid" {
		t.Fatal(got)
	}
	pitch, _ := Default().Step("pitch")
	_, err = pitch.Sanitize(raw(t, map[string]any{"video_url": "javascript:alert(1)"}))
	if got = fieldErrors(t, err); got["fields.video_url"] != "url" {
		t.Fatal(got)
	}
	values, err = pitch.Sanitize(raw(t, map[string]any{"video_url": "youtu.be/abc", "contact": nil}))
	if err != nil || values["video_url"] != "https://youtu.be/abc" || values["contact"] != nil {
		t.Fatal(values, err)
	}
}

func TestMissingRequirements(t *testing.T) {
	problem, _ := Default().Step("problem")
	gaps := problem.Missing(Values{"statement": "Слишком коротко", "who": ""})
	codes := map[string]string{}
	for _, g := range gaps {
		codes[g.Field] = g.Code
	}
	if codes["statement"] != "minLength" || codes["who"] != "required" || codes["alternatives"] != "required" || len(codes) != 3 {
		t.Fatal(gaps)
	}

	audience, _ := Default().Step("audience")
	values := Values{
		"segments":       "Первокурсники, которые живут в общежитии, и магистранты, совмещающие учёбу с работой.",
		"early_adopters": "Старосты групп первого курса: они каждый день ищут аудитории для своей группы.",
		"personas":       []any{map[string]any{"context": "без имени"}},
	}
	if gaps := audience.Missing(values); len(gaps) != 1 || gaps[0].Field != "personas" || gaps[0].Count != 1 {
		t.Fatalf("a row without its required column must not count: %v", gaps)
	}
	values["personas"] = []any{map[string]any{"name": "Айгерім, 18 жас"}}
	if gaps := audience.Missing(values); len(gaps) != 0 {
		t.Fatal(gaps)
	}
	if filled, total := audience.Filled(values); filled != 3 || total != 3 {
		t.Fatal(filled, total)
	}

	unit, _ := Default().Step("unit_economics")
	if gaps := unit.Missing(Values{"price": 0.0}); len(gaps) != 5 {
		t.Fatalf("zero is a value, the rest is missing: %v", gaps)
	}
}

func TestTemplateChecks(t *testing.T) {
	bad := map[string]string{
		"duplicate step": "version: 1\nstages: [{key: a, steps: [{key: s, fields: []}, {key: s, fields: []}]}]",
		"no label":       "version: 1\nstages: [{key: a, steps: [{key: s, fields: [{key: f, type: text}]}]}]",
		"unknown type": "version: 1\nstages: [{key: a, steps: [{key: s, fields: [{key: f, type: date, " +
			"label: {kk: a, ru: a, en: a}}]}]}]",
		"no steps": "version: 1\nstages: []",
	}
	for name, doc := range bad {
		if _, err := parseTemplate([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
