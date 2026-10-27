// Package schema validates model output against JSON Schema (draft 2020-12)
// and adapts schemas to what providers accept for constrained decoding.
package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator checks documents against one compiled schema.
type Validator struct {
	schema *jsonschema.Schema
}

var (
	mu       sync.Mutex
	compiled = map[string]*Validator{}
)

// Compile returns the validator of a schema, compiling it once per name.
func Compile(name string, doc map[string]any) (*Validator, error) {
	mu.Lock()
	defer mu.Unlock()
	if v, ok := compiled[name]; ok {
		return v, nil
	}
	// Round-trip through JSON so that numbers have the types the compiler expects.
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}
	c := jsonschema.NewCompiler()
	loc := "mem://qadam/" + name + ".json"
	if err := c.AddResource(loc, parsed); err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}
	s, err := c.Compile(loc)
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}
	v := &Validator{schema: s}
	compiled[name] = v
	return v, nil
}

// ErrInvalid wraps every validation failure.
var ErrInvalid = errors.New("output does not match the schema")

// Validate parses text as one JSON value and checks it against the schema.
// The error text is short enough to show the model in a retry.
func (v *Validator) Validate(text string) (any, error) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(Extract(text)))
	if err != nil {
		return nil, fmt.Errorf("%w: not valid JSON: %w", ErrInvalid, err)
	}
	if err := v.schema.Validate(doc); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return nil, fmt.Errorf("%w: %s", ErrInvalid, describe(ve))
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return doc, nil
}

func describe(ve *jsonschema.ValidationError) string {
	var parts []string
	var walk func(u *jsonschema.OutputUnit)
	walk = func(u *jsonschema.OutputUnit) {
		if u.Error != nil && len(u.Errors) == 0 {
			parts = append(parts, fmt.Sprintf("at %q: %s", "/"+strings.TrimPrefix(u.InstanceLocation, "/"), u.Error.String()))
		}
		for i := range u.Errors {
			walk(&u.Errors[i])
		}
	}
	walk(ve.BasicOutput())
	if len(parts) > 8 {
		parts = append(parts[:8], fmt.Sprintf("and %d more", len(parts)-8))
	}
	if len(parts) == 0 {
		return ve.Error()
	}
	return strings.Join(parts, "; ")
}

// Extract returns the JSON value in a reply: models that do not support
// constrained decoding sometimes wrap it in a Markdown fence or a sentence.
func Extract(text string) string {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "```") {
		if nl := strings.IndexByte(t, '\n'); nl >= 0 {
			t = t[nl+1:]
		}
		t = strings.TrimSuffix(strings.TrimSpace(t), "```")
	}
	start := strings.IndexAny(t, "{[")
	if start < 0 {
		return t
	}
	closer := byte('}')
	if t[start] == '[' {
		closer = ']'
	}
	end := strings.LastIndexByte(t, closer)
	if end < start {
		return t[start:]
	}
	return t[start : end+1]
}

// unsupported keywords are dropped from schemas sent for constrained decoding
// (Claude structured outputs); the full schema is still validated locally.
var unsupported = map[string]bool{
	"minimum": true, "maximum": true, "exclusiveMinimum": true, "exclusiveMaximum": true, "multipleOf": true,
	"minLength": true, "maxLength": true, "pattern": true,
	"minItems": true, "maxItems": true, "uniqueItems": true, "minProperties": true, "maxProperties": true,
	"$schema": true, "$id": true, "examples": true, "default": true,
}

// ForDecoding copies a schema without the keywords constrained decoding does
// not support, and closes every object (additionalProperties: false).
func ForDecoding(doc map[string]any) map[string]any {
	out, _ := strip(doc).(map[string]any)
	return out
}

func strip(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if unsupported[k] {
				continue
			}
			if k == "properties" || k == "$defs" || k == "definitions" {
				props, ok := val.(map[string]any)
				if !ok {
					continue
				}
				copied := make(map[string]any, len(props))
				for name, s := range props {
					copied[name] = strip(s)
				}
				out[k] = copied
				continue
			}
			out[k] = strip(val)
		}
		if out["type"] == "object" {
			out["additionalProperties"] = false
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = strip(item)
		}
		return out
	default:
		return v
	}
}
