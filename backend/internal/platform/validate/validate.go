// Package validate checks user input field by field. Error codes and
// parameters match the validation messages of the web client
// (common:validation.<code>), so a server-side rejection reads exactly like a
// client-side one.
package validate

import (
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// Field error codes.
const (
	CodeRequired  = "required"
	CodeMinLength = "minLength"
	CodeMaxLength = "maxLength"
	CodeMaxItems  = "maxItems"
	CodeMinItems  = "minItems"
	CodeInvalid   = "invalid"
	CodeURL       = "url"
	CodeFormat    = "format"
	CodeTaken     = "taken"
	CodeNotFound  = "not_found"
	CodeMin       = "min"
	CodeMax       = "max"
)

// Errors collects field errors.
type Errors struct {
	fields []apperr.FieldError
}

// Add records an error of a field.
func (e *Errors) Add(field, code string, params map[string]any) {
	e.fields = append(e.fields, apperr.FieldError{Field: field, Code: code, Params: params})
}

// Has reports whether the field already has an error.
func (e *Errors) Has(field string) bool {
	return slices.ContainsFunc(e.fields, func(f apperr.FieldError) bool { return f.Field == field })
}

// Err returns a validation error, or nil when every field is valid.
func (e *Errors) Err() error {
	if len(e.fields) == 0 {
		return nil
	}
	return apperr.Validation(e.fields...)
}

// Clean trims the value and removes control characters (keeping line breaks
// when multiline).
func Clean(s string, multiline bool) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' && multiline {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || r == 0x200B || r == 0xFEFF {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// Text checks a required single-line value.
func (e *Errors) Text(field, value string, minLen, maxLen int) string {
	v := Clean(value, false)
	e.length(field, v, minLen, maxLen)
	return v
}

// Paragraph checks a required multi-line value.
func (e *Errors) Paragraph(field, value string, minLen, maxLen int) string {
	v := Clean(strings.ReplaceAll(value, "\r\n", "\n"), true)
	e.length(field, v, minLen, maxLen)
	return v
}

func (e *Errors) length(field, v string, minLen, maxLen int) {
	n := utf8.RuneCountInString(v)
	switch {
	case n == 0 && minLen > 0:
		e.Add(field, CodeRequired, nil)
	case n < minLen:
		e.Add(field, CodeMinLength, map[string]any{"count": minLen})
	case n > maxLen:
		e.Add(field, CodeMaxLength, map[string]any{"count": maxLen})
	}
}

// Optional checks an optional value; blank becomes nil.
func (e *Errors) Optional(field string, value *string, maxLen int, multiline bool) *string {
	if value == nil {
		return nil
	}
	v := Clean(strings.ReplaceAll(*value, "\r\n", "\n"), multiline)
	if v == "" {
		return nil
	}
	if utf8.RuneCountInString(v) > maxLen {
		e.Add(field, CodeMaxLength, map[string]any{"count": maxLen})
	}
	return &v
}

// Tags checks a list of short labels (skills, interests): trims them, drops
// blanks and case-insensitive duplicates, keeping the first spelling.
func (e *Errors) Tags(field string, values []string, maxItems, maxLen int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		v := strings.Join(strings.Fields(Clean(raw, false)), " ")
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if seen[key] {
			continue
		}
		seen[key] = true
		if utf8.RuneCountInString(v) > maxLen {
			e.Add(field, CodeMaxLength, map[string]any{"count": maxLen})
			return out
		}
		out = append(out, v)
	}
	if len(out) > maxItems {
		e.Add(field, CodeMaxItems, map[string]any{"count": maxItems})
	}
	return out
}

// URL checks an optional http(s) link; hosts, when given, restrict the domain
// (subdomains included).
func (e *Errors) URL(field string, value *string, maxLen int, hosts ...string) *string {
	v := e.Optional(field, value, maxLen, false)
	if v == nil || e.Has(field) {
		return v
	}
	s := *v
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		e.Add(field, CodeURL, nil)
		return v
	}
	if len(hosts) > 0 {
		host := strings.ToLower(u.Hostname())
		if !slices.ContainsFunc(hosts, func(h string) bool { return host == h || strings.HasSuffix(host, "."+h) }) {
			e.Add(field, CodeURL, nil)
			return v
		}
	}
	return &s
}

// IntRange checks an optional integer.
func (e *Errors) IntRange(field string, value *int, minV, maxV int) {
	if value == nil {
		return
	}
	if *value < minV {
		e.Add(field, CodeMin, map[string]any{"value": minV})
	} else if *value > maxV {
		e.Add(field, CodeMax, map[string]any{"value": maxV})
	}
}
