// Package mock is a deterministic llm provider: the same request always gets
// the same answer. It runs the demo without any model and makes tests of
// AI features reproducible. Features register generators for realistic
// answers; structured requests without one get a value built from the schema.
package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
)

// EmbeddingModel names the vectors of the mock.
const EmbeddingModel = "mock-hash-256"

const dims = 256

// Generator writes the answer to a request.
type Generator func(req llm.CompletionRequest) string

var (
	mu         sync.RWMutex
	generators = map[llm.Feature]Generator{}
)

// Register sets the generator of a feature.
func Register(f llm.Feature, g Generator) {
	mu.Lock()
	defer mu.Unlock()
	generators[f] = g
}

// Provider is the mock.
type Provider struct {
	// ChunkDelay paces streamed words so the demo reads like a live model.
	ChunkDelay time.Duration
}

var _ llm.Provider = (*Provider)(nil)

// Name implements llm.Provider.
func (p *Provider) Name() string { return "mock" }

// Local implements llm.Provider: nothing leaves the server.
func (p *Provider) Local() bool { return true }

// DefaultModel implements llm.Provider.
func (p *Provider) DefaultModel(f llm.Feature) string {
	if f == llm.FeatureEmbedding {
		return EmbeddingModel
	}
	return "mock-" + string(f)
}

func (p *Provider) answer(req llm.CompletionRequest) string {
	mu.RLock()
	g := generators[req.Metadata.Feature]
	mu.RUnlock()
	switch {
	case g != nil:
		return g(req)
	case req.JSONSchema != nil:
		raw, err := json.Marshal(Fake(req.JSONSchema.JSON, seed(req)))
		if err != nil {
			return "{}"
		}
		return string(raw)
	default:
		return Reply(req)
	}
}

// Complete implements llm.Provider.
func (p *Provider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	text := p.answer(req)
	return &llm.CompletionResponse{
		Text: text, Provider: p.Name(), Model: modelOf(p, req), StopReason: llm.StopEnd, Usage: usage(req, text),
	}, nil
}

// Stream implements llm.Provider.
func (p *Provider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
	text := p.answer(req)
	ch := make(chan llm.StreamEvent, 8)
	go func() {
		defer close(ch)
		for _, chunk := range chunks(text) {
			if p.ChunkDelay > 0 {
				select {
				case <-time.After(p.ChunkDelay):
				case <-ctx.Done():
					return
				}
			}
			select {
			case ch <- llm.StreamEvent{Delta: chunk}:
			case <-ctx.Done():
				return
			}
		}
		select {
		case ch <- llm.StreamEvent{Done: &llm.CompletionResponse{
			Text: text, Provider: p.Name(), Model: modelOf(p, req), StopReason: llm.StopEnd, Usage: usage(req, text),
		}}:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

func modelOf(p *Provider, req llm.CompletionRequest) string {
	if req.Model != "" {
		return req.Model
	}
	return p.DefaultModel(req.Metadata.Feature)
}

// chunks splits text into word-sized pieces that rebuild it exactly.
func chunks(text string) []string {
	var out []string
	start := 0
	for i, r := range text {
		if r == ' ' && i > start {
			out = append(out, text[start:i+1])
			start = i + 1
		}
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

// tokens estimates the token count of a text (about four characters each).
func tokens(s string) int { return (utf8.RuneCountInString(s) + 3) / 4 }

func usage(req llm.CompletionRequest, answer string) llm.Usage {
	in := tokens(req.System)
	for _, m := range req.Messages {
		in += tokens(m.Content)
	}
	return llm.Usage{InputTokens: in, OutputTokens: tokens(answer)}
}

func seed(req llm.CompletionRequest) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(req.System))
	for _, m := range req.Messages {
		_, _ = h.Write([]byte(m.Content))
	}
	return h.Sum64()
}

// LastUserMessage returns the latest user turn.
func LastUserMessage(req llm.CompletionRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == llm.RoleUser {
			return req.Messages[i].Content
		}
	}
	return ""
}

var replies = map[string][]string{
	"kk": {
		"Жақсы бастама! Нақтылау үшін екі сұрақ: бұл мәселе кімге ең қатты әсер етеді — оларды бір сөйлеммен сипаттап көріңізші? Және олар бүгін бұл мәселені қалай шешіп жүр, оның қандай кемшілігі бар?",
		"Ойыңызды түсіндім. Енді оны тексеріп көрейік: осы болжамды растайтын немесе жоққа шығаратын қандай дерек бар? Келесі аптада әлеуетті 5 пайдаланушымен сөйлесіп, не сұрар едіңіз?",
		"Тұжырымыңыз күштірек болуы үшін нақты сан қосып көріңіз: қанша адам, қаншалықты жиі, қанша уақыт немесе ақша жоғалтады? Сандарды қайдан алғаныңызды да белгілеңіз.",
	},
	"ru": {
		"Хорошее начало! Чтобы уточнить, два вопроса: кого эта проблема задевает сильнее всего — сможете описать их одним предложением? И как они решают её сегодня, чем их это решение не устраивает?",
		"Понимаю вашу мысль. Давайте её проверим: какие факты подтверждают или опровергают эту гипотезу? Что бы вы спросили у пяти потенциальных пользователей на следующей неделе?",
		"Чтобы формулировка звучала сильнее, добавьте конкретику: сколько людей, как часто, сколько времени или денег они теряют? И отметьте, откуда взяты цифры — это ваши данные или допущение.",
	},
	"en": {
		"Good start! Two questions to sharpen it: who feels this problem the most — can you describe them in one sentence? And how do they solve it today, and what is wrong with that?",
		"I see your point. Let us test it: what evidence supports or contradicts this assumption? What would you ask five potential users next week?",
		"To make this stronger, add numbers: how many people, how often, how much time or money do they lose? Note where each number comes from — your data or an assumption.",
	},
}

// Reply is the default conversational answer in the request's language.
func Reply(req llm.CompletionRequest) string {
	locale := req.Metadata.Locale
	options, ok := replies[locale]
	if !ok {
		options = replies["ru"]
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(LastUserMessage(req)))
	return options[int(h.Sum32()%uint32(len(options)))] //nolint:gosec // len is a small constant
}

// Embed implements llm.Provider with the hashing trick: words and word pairs
// land in fixed buckets, so texts sharing words get close vectors.
func (p *Provider) Embed(ctx context.Context, texts []string) ([][]float32, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = Vector(t)
	}
	return out, EmbeddingModel, nil
}

// Vector embeds one text.
func Vector(text string) []float32 {
	v := make([]float32, dims)
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	add := func(term string, weight float32) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(term))
		sum := h.Sum32()
		sign := float32(1)
		if sum&1 == 1 {
			sign = -1
		}
		v[(sum>>1)%dims] += sign * weight
	}
	prev := ""
	for _, w := range words {
		// Short words ("до", "на", "и") carry no topic and only add noise.
		if utf8.RuneCountInString(w) < 3 {
			continue
		}
		s := stem(w)
		add(s, 1)
		if prev != "" {
			add(prev+" "+s, 0.5)
		}
		prev = s
	}
	var norm float64
	for _, x := range v {
		norm += float64(x * x)
	}
	if norm == 0 {
		v[0] = 1
		return v
	}
	scale := float32(1 / math.Sqrt(norm))
	for i := range v {
		v[i] *= scale
	}
	return v
}

// stem cuts word endings so that inflected Russian and Kazakh words meet.
func stem(w string) string {
	r := []rune(w)
	if len(r) > 6 {
		return string(r[:6])
	}
	return w
}

// Fake builds a value matching a JSON schema, varied by seed.
func Fake(s map[string]any, seed uint64) any {
	return fake(s, seed, "value", s)
}

func fake(s map[string]any, seed uint64, name string, root map[string]any) any {
	if ref, ok := s["$ref"].(string); ok {
		if def := resolve(root, ref); def != nil {
			return fake(def, seed, name, root)
		}
	}
	if c, ok := s["const"]; ok {
		return c
	}
	if e, ok := s["enum"].([]any); ok && len(e) > 0 {
		return e[int(seed%uint64(len(e)))] //nolint:gosec // bounded by len
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		if opts, ok := s[key].([]any); ok && len(opts) > 0 {
			for _, o := range opts {
				if m, ok := o.(map[string]any); ok && m["type"] != "null" {
					return fake(m, seed, name, root)
				}
			}
		}
	}
	switch typeOf(s) {
	case "object":
		out := map[string]any{}
		props, _ := s["properties"].(map[string]any)
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if ps, ok := props[k].(map[string]any); ok {
				out[k] = fake(ps, seed+uint64(i)*7919, k, root)
			}
		}
		return out
	case "array":
		n := 2
		if m, ok := number(s["minItems"]); ok && int(m) > n {
			n = int(m)
		}
		if m, ok := number(s["maxItems"]); ok && int(m) < n {
			n = int(m)
		}
		items, _ := s["items"].(map[string]any)
		out := make([]any, n)
		for i := range out {
			out[i] = fake(items, seed+uint64(i)+1, name, root)
		}
		return out
	case "integer", "number":
		lo, hasLo := number(s["minimum"])
		hi, hasHi := number(s["maximum"])
		switch {
		case hasLo && hasHi:
			v := lo + float64(seed%uint64(hi-lo+1))
			if typeOf(s) == "integer" {
				return int(v)
			}
			return v
		case hasLo:
			return int(lo)
		default:
			return int(seed % 10)
		}
	case "boolean":
		return seed%2 == 0
	case "null":
		return nil
	default:
		if f, _ := s["format"].(string); f == "date-time" {
			return "2026-10-22T10:00:00Z"
		}
		text := fmt.Sprintf("%s %d", strings.ReplaceAll(name, "_", " "), seed%97)
		if m, ok := number(s["minLength"]); ok {
			for utf8.RuneCountInString(text) < int(m) {
				text += " " + text
			}
		}
		return text
	}
}

func typeOf(s map[string]any) string {
	switch t := s["type"].(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if str, ok := v.(string); ok && str != "null" {
				return str
			}
		}
	}
	if _, ok := s["properties"]; ok {
		return "object"
	}
	return "string"
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func resolve(root map[string]any, ref string) map[string]any {
	path, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return nil
	}
	var cur any = root
	for _, part := range strings.Split(path, "/") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	m, _ := cur.(map[string]any)
	return m
}
