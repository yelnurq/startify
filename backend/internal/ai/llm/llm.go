// Package llm is the provider-neutral interface to language models. No module
// talks to a model API directly: requests go through ai.Gateway to a Provider
// (Anthropic, an OpenAI-compatible server, or the deterministic mock).
package llm

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Feature is what the platform uses a model for; providers and limits are
// chosen per feature.
type Feature string

// Features.
const (
	FeatureMentor     Feature = "mentor"
	FeatureEvaluation Feature = "evaluation"
	FeatureGeneration Feature = "generation"
	FeatureSummary    Feature = "summary"
	FeatureEmbedding  Feature = "embedding"
)

// Features lists every feature.
var Features = []Feature{FeatureMentor, FeatureEvaluation, FeatureGeneration, FeatureSummary, FeatureEmbedding}

// Valid reports whether f is a known feature.
func (f Feature) Valid() bool {
	for _, k := range Features {
		if f == k {
			return true
		}
	}
	return false
}

// Role is the author of a message.
type Role string

// Roles.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn of a conversation.
type Message struct {
	Role    Role
	Content string
}

// Effort trades depth of reasoning for latency and cost on models that think.
type Effort string

// Efforts.
const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
)

// Schema asks for output that is one JSON value matching the schema.
type Schema struct {
	// Name identifies the schema (OpenAI requires one).
	Name string
	// JSON is the JSON Schema (draft 2020-12) as a decoded object.
	JSON map[string]any
}

// Tool is a function the model may ask to call.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolCall is a call the model asked for.
type ToolCall struct {
	ID    string
	Name  string
	Input []byte
}

// RequestMeta identifies who a request is made for; it is used for limits,
// accounting and provider-side abuse monitoring, never sent as content.
type RequestMeta struct {
	TenantID      uuid.UUID
	UserID        uuid.UUID
	Feature       Feature
	PromptVersion string
	// Locale is the language the answer is expected in (kk, ru, en).
	Locale string
}

// CompletionRequest is a request for one model response.
type CompletionRequest struct {
	Model     string
	System    string
	Messages  []Message
	MaxTokens int
	// Temperature is ignored by models that reject sampling parameters
	// (current Claude models); nil keeps the provider default.
	Temperature *float32
	Effort      Effort
	JSONSchema  *Schema
	Tools       []Tool
	Metadata    RequestMeta
}

// Usage counts tokens of a response.
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
}

// Add sums usages.
func (u Usage) Add(o Usage) Usage {
	return Usage{
		InputTokens:      u.InputTokens + o.InputTokens,
		OutputTokens:     u.OutputTokens + o.OutputTokens,
		CacheReadTokens:  u.CacheReadTokens + o.CacheReadTokens,
		CacheWriteTokens: u.CacheWriteTokens + o.CacheWriteTokens,
	}
}

// StopReason says why generation ended.
type StopReason string

// Stop reasons.
const (
	StopEnd       StopReason = "end"
	StopMaxTokens StopReason = "max_tokens"
	StopToolUse   StopReason = "tool_use"
	StopRefusal   StopReason = "refusal"
)

// CompletionResponse is a model response.
type CompletionResponse struct {
	Text       string
	ToolCalls  []ToolCall
	Provider   string
	Model      string
	StopReason StopReason
	Usage      Usage
}

// StreamEvent is a piece of a streamed response: a text delta, or the final
// response (Done), or an error. The channel closes after Done or Err.
type StreamEvent struct {
	Delta string
	Done  *CompletionResponse
	Err   error
}

// Provider is a language-model backend.
type Provider interface {
	// Name identifies the provider in configuration: anthropic, openai, mock.
	Name() string
	// Local reports whether the provider runs inside the university network;
	// data sent elsewhere is redacted and users are warned.
	Local() bool
	// DefaultModel is used when a route names no model; it is empty for a
	// feature the provider cannot serve (embeddings on Anthropic).
	DefaultModel(f Feature) string
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error)
	// Embed returns one vector per text and the model that produced them.
	Embed(ctx context.Context, texts []string) ([][]float32, string, error)
}

// Errors returned by providers.
var (
	// ErrNotSupported: the provider cannot do this (e.g. embeddings on Anthropic).
	ErrNotSupported = errors.New("llm: not supported by the provider")
	// ErrRefused: the model declined the request.
	ErrRefused = errors.New("llm: the model declined the request")
	// ErrUnavailable: the provider could not be reached or is overloaded; another may serve.
	ErrUnavailable = errors.New("llm: provider unavailable")
	// ErrBadRequest: the provider rejected the request itself; retrying elsewhere may help.
	ErrBadRequest = errors.New("llm: request rejected by the provider")
)
