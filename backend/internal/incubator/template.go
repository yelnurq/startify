package incubator

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
)

//go:embed template.yaml
var templateYAML []byte

// Field types.
const (
	TypeText     = "text"
	TypeTextarea = "textarea"
	TypeNumber   = "number"
	TypeSelect   = "select"
	TypeList     = "list"
	TypeTable    = "table"
	TypeURL      = "url"
)

// Default maximum lengths when the template sets none.
const (
	defaultTextMax     = 200
	defaultTextareaMax = 4000
	defaultItemsMax    = 20
	urlMax             = 500
	maxNumber          = 1e15
)

// Template is the sequence of incubator steps.
type Template struct {
	Version int     `yaml:"version"`
	Stages  []Stage `yaml:"stages"`

	steps map[string]*Step
	order []string
}

// Stage groups steps.
type Stage struct {
	Key   string       `yaml:"key"`
	Title apiconv.Text `yaml:"title"`
	Steps []Step       `yaml:"steps"`
}

// Step is one form of the incubator.
type Step struct {
	Key      string       `yaml:"key"`
	Title    apiconv.Text `yaml:"title"`
	Goal     apiconv.Text `yaml:"goal"`
	Method   apiconv.Text `yaml:"method"`
	Criteria []Criterion  `yaml:"criteria"`
	Fields   []Field      `yaml:"fields"`

	stage  string
	number int
}

// Stage returns the key of the step's stage.
func (s *Step) Stage() string { return s.stage }

// Number is the 1-based position of the step.
func (s *Step) Number() int { return s.number }

// Criterion describes a strong answer; Fields are the inputs it judges.
type Criterion struct {
	Fields []string     `yaml:"fields"`
	Text   apiconv.Text `yaml:"text"`
}

// Field is an input of a step, or a column of a table.
type Field struct {
	Key         string       `yaml:"key"`
	Type        string       `yaml:"type"`
	Required    bool         `yaml:"required"`
	MinLength   int          `yaml:"min_length"`
	MaxLength   int          `yaml:"max_length"`
	MinItems    int          `yaml:"min_items"`
	MaxItems    int          `yaml:"max_items"`
	Unit        string       `yaml:"unit"`
	Label       apiconv.Text `yaml:"label"`
	Hint        apiconv.Text `yaml:"hint"`
	Placeholder apiconv.Text `yaml:"placeholder"`
	Options     []Option     `yaml:"options"`
	Columns     []Field      `yaml:"columns"`
}

// Option is a choice of a select field.
type Option struct {
	Value string       `yaml:"value"`
	Label apiconv.Text `yaml:"label"`
}

var loadDefault = sync.OnceValues(func() (*Template, error) { return parseTemplate(templateYAML) })

// Default returns the built-in template.
func Default() *Template {
	t, err := loadDefault()
	if err != nil {
		panic(err) // the embedded template is checked by tests
	}
	return t
}

func parseTemplate(raw []byte) (*Template, error) {
	var t Template
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("incubator: template: %w", err)
	}
	t.steps = map[string]*Step{}
	for si := range t.Stages {
		stage := &t.Stages[si]
		for i := range stage.Steps {
			s := &stage.Steps[i]
			if _, dup := t.steps[s.Key]; dup || s.Key == "" {
				return nil, fmt.Errorf("incubator: template: duplicate or empty step %q", s.Key)
			}
			s.stage = stage.Key
			s.number = len(t.order) + 1
			t.steps[s.Key] = s
			t.order = append(t.order, s.Key)
			if err := checkFields(s.Key, s.Fields, true); err != nil {
				return nil, err
			}
			if err := checkCriteria(s); err != nil {
				return nil, err
			}
		}
	}
	if len(t.order) == 0 {
		return nil, errors.New("incubator: template has no steps")
	}
	return &t, nil
}

func checkCriteria(s *Step) error {
	for _, c := range s.Criteria {
		for _, l := range []string{"kk", "ru", "en"} {
			if c.Text[l] == "" {
				return fmt.Errorf("incubator: template: a criterion of %s has no %s text", s.Key, l)
			}
		}
		for _, key := range c.Fields {
			if s.Field(key) == nil {
				return fmt.Errorf("incubator: template: a criterion of %s names unknown field %q", s.Key, key)
			}
		}
	}
	return nil
}

func checkFields(path string, fields []Field, top bool) error {
	seen := map[string]bool{}
	for i := range fields {
		f := &fields[i]
		where := path + "." + f.Key
		if f.Key == "" || seen[f.Key] {
			return fmt.Errorf("incubator: template: duplicate or empty field %q", where)
		}
		seen[f.Key] = true
		for _, l := range []string{"kk", "ru", "en"} {
			if f.Label[l] == "" {
				return fmt.Errorf("incubator: template: %s has no %s label", where, l)
			}
		}
		switch f.Type {
		case TypeText, TypeTextarea, TypeNumber, TypeURL:
		case TypeSelect:
			if len(f.Options) == 0 {
				return fmt.Errorf("incubator: template: %s has no options", where)
			}
		case TypeList:
			if !top {
				return fmt.Errorf("incubator: template: %s: lists cannot be table columns", where)
			}
		case TypeTable:
			if !top || len(f.Columns) == 0 {
				return fmt.Errorf("incubator: template: %s: tables need columns and cannot nest", where)
			}
			if err := checkFields(where, f.Columns, false); err != nil {
				return err
			}
		default:
			return fmt.Errorf("incubator: template: %s has unknown type %q", where, f.Type)
		}
	}
	return nil
}

// Field finds a field of the step.
func (s *Step) Field(key string) *Field {
	for i := range s.Fields {
		if s.Fields[i].Key == key {
			return &s.Fields[i]
		}
	}
	return nil
}

// Step finds a step by key.
func (t *Template) Step(key string) (*Step, bool) {
	s, ok := t.steps[key]
	return s, ok
}

// Keys are the step keys in order.
func (t *Template) Keys() []string { return t.order }

// Next is the step after key, or "" for the last one.
func (t *Template) Next(key string) string {
	for i, k := range t.order {
		if k == key && i+1 < len(t.order) {
			return t.order[i+1]
		}
	}
	return ""
}

func (f *Field) maxLength() int {
	switch {
	case f.MaxLength > 0:
		return f.MaxLength
	case f.Type == TypeTextarea:
		return defaultTextareaMax
	case f.Type == TypeURL:
		return urlMax
	default:
		return defaultTextMax
	}
}

func (f *Field) maxItems() int {
	if f.MaxItems > 0 {
		return f.MaxItems
	}
	return defaultItemsMax
}

func (f *Field) option(v string) bool {
	for _, o := range f.Options {
		if o.Value == v {
			return true
		}
	}
	return false
}

// Values are the stored answers of a step, keyed by field.
type Values map[string]any

// Sanitize checks values sent while editing: types, lengths and options, but
// not the minimum a completed step needs (drafts are saved as typed). It
// returns cleaned values; unknown fields are errors.
func (s *Step) Sanitize(in map[string]json.RawMessage) (Values, error) {
	var v validate.Errors
	out := Values{}
	for key, raw := range in {
		f := s.Field(key)
		if f == nil {
			v.Add("fields."+key, validate.CodeInvalid, nil)
			continue
		}
		out[key] = f.sanitize(&v, "fields."+key, raw)
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (f *Field) sanitize(v *validate.Errors, path string, raw json.RawMessage) any {
	if string(raw) == "null" {
		if f.Type == TypeList || f.Type == TypeTable {
			return []any{}
		}
		return nil
	}
	switch f.Type {
	case TypeNumber:
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			v.Add(path, validate.CodeInvalid, nil)
			return nil
		}
		if n < 0 {
			v.Add(path, validate.CodeMin, map[string]any{"value": 0})
		} else if n > maxNumber {
			v.Add(path, validate.CodeMax, map[string]any{"value": maxNumber})
		}
		return n
	case TypeList:
		var items []string
		if err := json.Unmarshal(raw, &items); err != nil {
			v.Add(path, validate.CodeInvalid, nil)
			return []any{}
		}
		if len(items) > f.maxItems() {
			v.Add(path, validate.CodeMaxItems, map[string]any{"count": f.maxItems()})
		}
		out := make([]any, len(items))
		for i, it := range items {
			out[i] = f.text(v, path+"."+strconv.Itoa(i), it, false)
		}
		return out
	case TypeTable:
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil {
			v.Add(path, validate.CodeInvalid, nil)
			return []any{}
		}
		if len(rows) > f.maxItems() {
			v.Add(path, validate.CodeMaxItems, map[string]any{"count": f.maxItems()})
		}
		cols := map[string]*Field{}
		for i := range f.Columns {
			cols[f.Columns[i].Key] = &f.Columns[i]
		}
		out := make([]any, len(rows))
		for i, row := range rows {
			clean := map[string]any{}
			for key, cell := range row {
				c, ok := cols[key]
				if !ok {
					v.Add(path+"."+strconv.Itoa(i)+"."+key, validate.CodeInvalid, nil)
					continue
				}
				clean[key] = c.sanitize(v, path+"."+strconv.Itoa(i)+"."+key, cell)
			}
			out[i] = clean
		}
		return out
	default:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			v.Add(path, validate.CodeInvalid, nil)
			return ""
		}
		return f.text(v, path, s, f.Type == TypeTextarea)
	}
}

func (f *Field) text(v *validate.Errors, path, s string, multiline bool) string {
	s = validate.Clean(strings.ReplaceAll(s, "\r\n", "\n"), multiline)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > f.maxLength() {
		v.Add(path, validate.CodeMaxLength, map[string]any{"count": f.maxLength()})
	}
	switch f.Type {
	case TypeSelect:
		if !f.option(s) {
			v.Add(path, validate.CodeInvalid, nil)
		}
	case TypeURL:
		if u := v.URL(path, &s, urlMax); u != nil {
			return *u
		}
	}
	return s
}

// Gap is a requirement a step does not meet yet.
type Gap struct {
	Field string
	Code  string
	Count int
}

// Missing lists what keeps a step from being completed.
func (s *Step) Missing(values Values) []Gap {
	var gaps []Gap
	for i := range s.Fields {
		f := &s.Fields[i]
		if !f.Required {
			continue
		}
		switch f.Type {
		case TypeList, TypeTable:
			n := f.filled(values[f.Key])
			minItems := max(f.MinItems, 1)
			if n < minItems {
				gaps = append(gaps, Gap{Field: f.Key, Code: validate.CodeMinItems, Count: minItems})
			}
		case TypeNumber:
			if _, ok := values[f.Key].(float64); !ok {
				gaps = append(gaps, Gap{Field: f.Key, Code: validate.CodeRequired})
			}
		default:
			text, _ := values[f.Key].(string)
			n := utf8.RuneCountInString(strings.TrimSpace(text))
			switch {
			case n == 0:
				gaps = append(gaps, Gap{Field: f.Key, Code: validate.CodeRequired})
			case n < f.MinLength:
				gaps = append(gaps, Gap{Field: f.Key, Code: validate.CodeMinLength, Count: f.MinLength})
			}
		}
	}
	return gaps
}

// filled counts list items and table rows that carry content: a row counts
// when its required columns (or, without any, one column) are filled.
func (f *Field) filled(v any) int {
	items, _ := v.([]any)
	n := 0
	for _, it := range items {
		switch x := it.(type) {
		case string:
			if strings.TrimSpace(x) != "" {
				n++
			}
		case map[string]any:
			if f.rowFilled(x) {
				n++
			}
		}
	}
	return n
}

func (f *Field) rowFilled(row map[string]any) bool {
	required, some := 0, false
	for i := range f.Columns {
		c := &f.Columns[i]
		has := present(row[c.Key])
		some = some || has
		if c.Required {
			if !has {
				return false
			}
			required++
		}
	}
	return required > 0 || some
}

func present(v any) bool {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) != ""
	case float64:
		return true
	}
	return false
}

// Grade rates the answers of a step from 0 (empty) to 1 (every field
// answered in substance).
func (s *Step) Grade(values Values) float64 {
	if len(s.Fields) == 0 {
		return 0
	}
	sum := 0
	for i := range s.Fields {
		sum += fieldLevel(&s.Fields[i], values[s.Fields[i].Key])
	}
	return float64(sum) / float64(2*len(s.Fields))
}

// Filled reports how many fields of a step have content, for progress.
func (s *Step) Filled(values Values) (filled, total int) {
	for i := range s.Fields {
		f := &s.Fields[i]
		total++
		switch f.Type {
		case TypeList, TypeTable:
			if f.filled(values[f.Key]) > 0 {
				filled++
			}
		default:
			if present(values[f.Key]) {
				filled++
			}
		}
	}
	return filled, total
}
