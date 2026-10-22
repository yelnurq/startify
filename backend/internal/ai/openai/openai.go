// Package openai serves llm requests with any server speaking the OpenAI chat
// completions protocol: OpenAI itself, or models inside the university
// network behind vLLM, Ollama or LM Studio.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/schema"
)

// Options configure the provider.
type Options struct {
	BaseURL        string
	APIKey         string
	Model          string
	EmbeddingModel string
	// Local marks a server inside the university network.
	Local   bool
	Timeout time.Duration
	// HTTPClient replaces the default client (tests).
	HTTPClient *http.Client
}

// Provider is an OpenAI-compatible server.
type Provider struct {
	o    Options
	http *http.Client
}

var _ llm.Provider = (*Provider)(nil)

// New returns the provider.
func New(o Options) *Provider {
	c := o.HTTPClient
	if c == nil {
		c = &http.Client{Timeout: o.Timeout}
	}
	o.BaseURL = strings.TrimSuffix(o.BaseURL, "/")
	return &Provider{o: o, http: c}
}

// Name implements llm.Provider.
func (p *Provider) Name() string { return "openai" }

// Local implements llm.Provider.
func (p *Provider) Local() bool { return p.o.Local }

// DefaultModel implements llm.Provider.
func (p *Provider) DefaultModel(f llm.Feature) string {
	if f == llm.FeatureEmbedding {
		return p.o.EmbeddingModel
	}
	return p.o.Model
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float32        `json:"temperature,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	StreamOptions  *streamOptions  `json:"stream_options,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Tools          []chatTool      `json:"tools,omitempty"`
	User           string          `json:"user,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

func (u *chatUsage) usage() llm.Usage {
	if u == nil {
		return llm.Usage{}
	}
	return llm.Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens, CacheReadTokens: u.PromptTokensDetails.CachedTokens}
}

type toolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			Refusal   string     `json:"refusal"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *chatUsage `json:"usage"`
}

func (p *Provider) request(req llm.CompletionRequest, stream bool) chatRequest {
	body := chatRequest{Model: req.Model, MaxTokens: req.MaxTokens, Temperature: req.Temperature, Stream: stream}
	if stream {
		body.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	if req.System != "" {
		body.Messages = append(body.Messages, chatMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, chatMessage{Role: string(m.Role), Content: m.Content})
	}
	if req.JSONSchema != nil {
		name := req.JSONSchema.Name
		if name == "" {
			name = "response"
		}
		body.ResponseFormat = &responseFormat{Type: "json_schema", JSONSchema: jsonSchema{
			Name: name, Schema: schema.ForDecoding(req.JSONSchema.JSON), Strict: true,
		}}
	}
	for _, t := range req.Tools {
		body.Tools = append(body.Tools, chatTool{Type: "function", Function: chatFunction{Name: t.Name, Description: t.Description, Parameters: t.InputSchema}})
	}
	if req.Metadata.UserID != uuid.Nil {
		sum := sha256.Sum256([]byte(req.Metadata.TenantID.String() + "/" + req.Metadata.UserID.String()))
		body.User = hex.EncodeToString(sum[:16])
	}
	return body
}

type statusError struct {
	status int
	body   string
}

func (e *statusError) Error() string { return fmt.Sprintf("openai: HTTP %d: %s", e.status, e.body) }

func (p *Provider) post(ctx context.Context, path string, body any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: encode: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.o.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("openai: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.o.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.o.APIKey)
	}
	resp, err := p.http.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %w", llm.ErrUnavailable, err)
	}
	if resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		se := &statusError{status: resp.StatusCode, body: strings.TrimSpace(string(msg))}
		switch resp.StatusCode {
		case http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
			return nil, fmt.Errorf("%w: %w", llm.ErrBadRequest, se)
		default:
			return nil, fmt.Errorf("%w: %w", llm.ErrUnavailable, se)
		}
	}
	return resp, nil
}

// chat sends a request; servers that reject the structured-output format get
// it once more without (the answer is still validated against the schema).
func (p *Provider) chat(ctx context.Context, body chatRequest) (*http.Response, error) {
	resp, err := p.post(ctx, "/chat/completions", body)
	var se *statusError
	if err != nil && body.ResponseFormat != nil && errors.As(err, &se) && se.status == http.StatusBadRequest {
		body.ResponseFormat = nil
		return p.post(ctx, "/chat/completions", body)
	}
	return resp, err
}

// Complete implements llm.Provider.
func (p *Provider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	resp, err := p.chat(ctx, p.request(req, false))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("%w: decode: %w", llm.ErrUnavailable, err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("%w: no choices", llm.ErrUnavailable)
	}
	c := out.Choices[0]
	res := &llm.CompletionResponse{Text: c.Message.Content, Provider: p.Name(), Model: modelOr(out.Model, req.Model), Usage: out.Usage.usage()}
	for _, tc := range c.Message.ToolCalls {
		res.ToolCalls = append(res.ToolCalls, llm.ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: []byte(tc.Function.Arguments)})
	}
	res.StopReason = stopReason(c.FinishReason)
	if c.Message.Refusal != "" || res.StopReason == llm.StopRefusal {
		res.StopReason = llm.StopRefusal
		return res, llm.ErrRefused
	}
	return res, nil
}

func modelOr(got, asked string) string {
	if got != "" {
		return got
	}
	return asked
}

func stopReason(finish string) llm.StopReason {
	switch finish {
	case "length":
		return llm.StopMaxTokens
	case "tool_calls", "function_call":
		return llm.StopToolUse
	case "content_filter":
		return llm.StopRefusal
	default:
		return llm.StopEnd
	}
}

// Stream implements llm.Provider.
func (p *Provider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
	resp, err := p.chat(ctx, p.request(req, true)) //nolint:bodyclose // closed by the stream goroutine
	if err != nil {
		return nil, err
	}
	ch := make(chan llm.StreamEvent, 16)
	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()
		send := func(ev llm.StreamEvent) bool {
			select {
			case ch <- ev:
				return true
			case <-ctx.Done():
				return false
			}
		}
		final := &llm.CompletionResponse{Provider: p.Name(), Model: req.Model, StopReason: llm.StopEnd}
		var text strings.Builder
		refused := false
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64<<10), 4<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			data, ok := strings.CutPrefix(line, "data:")
			if !ok {
				continue
			}
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				break
			}
			var chunk chatResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				send(llm.StreamEvent{Err: fmt.Errorf("%w: stream chunk: %w", llm.ErrUnavailable, err)})
				return
			}
			if chunk.Model != "" {
				final.Model = chunk.Model
			}
			if chunk.Usage != nil {
				final.Usage = chunk.Usage.usage()
			}
			for _, c := range chunk.Choices {
				if c.Delta.Refusal != "" {
					refused = true
				}
				if c.Delta.Content != "" {
					text.WriteString(c.Delta.Content)
					if !send(llm.StreamEvent{Delta: c.Delta.Content}) {
						return
					}
				}
				if c.FinishReason != "" {
					final.StopReason = stopReason(c.FinishReason)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			if ctx.Err() != nil {
				return
			}
			send(llm.StreamEvent{Err: fmt.Errorf("%w: stream: %w", llm.ErrUnavailable, err)})
			return
		}
		final.Text = text.String()
		if refused || final.StopReason == llm.StopRefusal {
			final.StopReason = llm.StopRefusal
			send(llm.StreamEvent{Err: llm.ErrRefused})
			return
		}
		send(llm.StreamEvent{Done: final})
	}()
	return ch, nil
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Embed implements llm.Provider.
func (p *Provider) Embed(ctx context.Context, texts []string) ([][]float32, string, error) {
	if p.o.EmbeddingModel == "" {
		return nil, "", llm.ErrNotSupported
	}
	resp, err := p.post(ctx, "/embeddings", embeddingRequest{Model: p.o.EmbeddingModel, Input: texts})
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	var out embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", fmt.Errorf("%w: decode embeddings: %w", llm.ErrUnavailable, err)
	}
	if len(out.Data) != len(texts) {
		return nil, "", fmt.Errorf("%w: %d embeddings for %d texts", llm.ErrUnavailable, len(out.Data), len(texts))
	}
	vectors := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(vectors) {
			return nil, "", fmt.Errorf("%w: embedding index %d", llm.ErrUnavailable, d.Index)
		}
		vectors[d.Index] = d.Embedding
	}
	// The configured name identifies the vectors: servers may report aliases.
	return vectors, p.o.EmbeddingModel, nil
}
