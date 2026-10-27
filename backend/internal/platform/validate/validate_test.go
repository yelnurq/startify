package validate

import (
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

func codes(t *testing.T, v *Errors) map[string]string {
	t.Helper()
	out := map[string]string{}
	e, ok := apperr.As(v.Err())
	if !ok {
		return out
	}
	for _, f := range e.Fields {
		out[f.Field] = f.Code
	}
	return out
}

func TestText(t *testing.T) {
	var v Errors
	if got := v.Text("name", "  Айгерім\u0000 Сәрсенова\t ", 2, 120); got != "Айгерім Сәрсенова" {
		t.Fatalf("cleaned %q", got)
	}
	v.Text("empty", "   ", 2, 10)
	v.Text("short", "a", 2, 10)
	v.Text("long", strings.Repeat("ә", 11), 2, 10)
	if got := v.Paragraph("bio", "line one\r\n\r\nline two", 1, 100); got != "line one\n\nline two" {
		t.Fatalf("paragraph %q", got)
	}
	got := codes(t, &v)
	if got["empty"] != CodeRequired || got["short"] != CodeMinLength || got["long"] != CodeMaxLength || len(got) != 3 {
		t.Fatalf("codes %v", got)
	}
	e, _ := apperr.As(v.Err())
	if e.Fields[2].Params["count"] != 10 {
		t.Fatalf("params %v", e.Fields[2].Params)
	}
}

func TestOptionalAndTags(t *testing.T) {
	var v Errors
	blank := "   "
	if v.Optional("headline", &blank, 10, false) != nil || v.Optional("headline", nil, 10, false) != nil {
		t.Fatal("blank must become nil")
	}
	tags := v.Tags("skills", []string{" Go ", "go", "", "Machine   Learning", "PostgreSQL"}, 3, 40)
	if strings.Join(tags, "|") != "Go|Machine Learning|PostgreSQL" {
		t.Fatalf("tags %v", tags)
	}
	v.Tags("interests", []string{"a", "b", "c"}, 2, 40)
	v.Tags("long", []string{strings.Repeat("x", 41)}, 2, 40)
	got := codes(t, &v)
	if got["interests"] != CodeMaxItems || got["long"] != CodeMaxLength {
		t.Fatalf("codes %v", got)
	}
}

func TestURL(t *testing.T) {
	var v Errors
	s := func(x string) *string { return &x }
	if u := v.URL("site", s("qadam.kz/team"), 300); *u != "https://qadam.kz/team" {
		t.Fatalf("scheme not added: %s", *u)
	}
	if u := v.URL("gh", s("https://github.com/aru"), 300, "github.com"); *u != "https://github.com/aru" {
		t.Fatalf("github: %s", *u)
	}
	v.URL("js", s("javascript:alert(1)"), 300)
	v.URL("foreign", s("https://github.com.evil.kz/aru"), 300, "github.com")
	v.URL("creds", s("https://user:pass@qadam.kz"), 300)
	got := codes(t, &v)
	for _, f := range []string{"js", "foreign", "creds"} {
		if got[f] != CodeURL {
			t.Errorf("%s: %v", f, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("unexpected errors %v", got)
	}
}

func TestIntRange(t *testing.T) {
	var v Errors
	low, high, ok := 0, 9, 3
	v.IntRange("low", &low, 1, 7)
	v.IntRange("high", &high, 1, 7)
	v.IntRange("ok", &ok, 1, 7)
	v.IntRange("nil", nil, 1, 7)
	got := codes(t, &v)
	if got["low"] != CodeMin || got["high"] != CodeMax || len(got) != 2 {
		t.Fatalf("codes %v", got)
	}
	var none Errors
	if none.Err() != nil {
		t.Fatal("no errors expected")
	}
}
