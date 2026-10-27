// Package apiconv converts stored values into the generated API types.
package apiconv

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
)

// Text is a value in several languages, stored as JSON {"kk": "...", "ru": "...", "en": "..."}.
type Text map[string]string

// ParseText decodes stored JSON; malformed values yield an empty text.
func ParseText(raw json.RawMessage) Text {
	t := Text{}
	_ = json.Unmarshal(raw, &t)
	return t
}

// JSON encodes the text for storage.
func (t Text) JSON() json.RawMessage {
	raw, err := json.Marshal(t)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

// API converts to the generated type, omitting empty languages.
func (t Text) API() apigen.LocalizedText {
	var out apigen.LocalizedText
	if v := t["kk"]; v != "" {
		out.Kk = &v
	}
	if v := t["ru"]; v != "" {
		out.Ru = &v
	}
	if v := t["en"]; v != "" {
		out.En = &v
	}
	return out
}

// TextFromAPI converts from the generated type.
func TextFromAPI(in apigen.LocalizedText) Text {
	t := Text{}
	if in.Kk != nil {
		t["kk"] = *in.Kk
	}
	if in.Ru != nil {
		t["ru"] = *in.Ru
	}
	if in.En != nil {
		t["en"] = *in.En
	}
	return t
}

// Best returns the text in the locale, falling back to Russian, Kazakh, English.
func (t Text) Best(locale string) string {
	for _, l := range []string{locale, "ru", "kk", "en"} {
		if v := t[l]; v != "" {
			return v
		}
	}
	return ""
}

// FileURL is the public path of an uploaded file, or nil.
func FileURL(id *uuid.UUID) *string {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	u := "/api/v1/files/" + id.String() + "/content"
	return &u
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }
