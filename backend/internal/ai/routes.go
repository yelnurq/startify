package ai

import (
	"fmt"
	"strings"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
)

// Route is a provider, and optionally a model, serving a feature.
type Route struct {
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`
}

// Routes lists the providers of each feature in the order they are tried.
type Routes map[llm.Feature][]Route

// ParseRoutes reads routes from configuration:
// "mentor=anthropic:claude-sonnet-5-5,openai;embedding=openai".
func ParseRoutes(s string) (Routes, error) {
	out := Routes{}
	for entry := range strings.SplitSeq(s, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, list, ok := strings.Cut(entry, "=")
		f := llm.Feature(strings.TrimSpace(name))
		if !ok || !f.Valid() {
			return nil, fmt.Errorf("ai: route %q: unknown feature", entry)
		}
		for item := range strings.SplitSeq(list, ",") {
			provider, model, _ := strings.Cut(strings.TrimSpace(item), ":")
			if provider == "" {
				return nil, fmt.Errorf("ai: route %q: empty provider", entry)
			}
			out[f] = append(out[f], Route{Provider: provider, Model: strings.TrimSpace(model)})
		}
	}
	return out, nil
}

// price is the cost of a million tokens in US dollars.
type price struct {
	input, output, cacheRead, cacheWrite float64
}

// prices of Claude models for cost accounting; other providers (local models)
// cost nothing per token. Cache writes are the 5-minute rate.
var prices = map[string]price{
	"claude-fable-5-1":  {input: 10, output: 50, cacheRead: 0.25, cacheWrite: 12.5},
	"claude-opus-5-5":   {input: 4, output: 20, cacheRead: 0.20, cacheWrite: 5},
	"claude-opus-5":     {input: 5, output: 25, cacheRead: 0.50, cacheWrite: 6.25},
	"claude-opus-4-8":   {input: 5, output: 25, cacheRead: 0.50, cacheWrite: 6.25},
	"claude-sonnet-5-5": {input: 2, output: 10, cacheRead: 0.20, cacheWrite: 2.5},
	"claude-haiku-5-5":  {input: 0.10, output: 0.50, cacheRead: 0.01, cacheWrite: 0.125},
}

// costMicros estimates the cost of a call in millionths of a dollar.
func costMicros(model string, u llm.Usage) int64 {
	p, ok := prices[model]
	if !ok {
		return 0
	}
	// A price per million tokens is the price in micro-dollars per token.
	total := float64(u.InputTokens)*p.input + float64(u.OutputTokens)*p.output +
		float64(u.CacheReadTokens)*p.cacheRead + float64(u.CacheWriteTokens)*p.cacheWrite
	return int64(total + 0.5)
}
