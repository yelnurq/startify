// Package anthropic serves llm requests with Claude through the official SDK.
package anthropic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/schema"
)

// Models chosen per feature when a route names none: the interactive mentor
// favours latency and cost, evaluations and generated materials favour depth,
// summaries are high-volume (D-031).
var defaultModels = map[llm.Feature]string{
	llm.FeatureMentor:     "claude-sonnet-5-5",
	llm.FeatureEvaluation: "claude-opus-5-5",
	llm.FeatureGeneration: "claude-opus-5-5",
	llm.FeatureSummary:    "claude-haiku-5-5",
}

// Options configure the provider.
type Options struct {
	APIKey  string
	BaseURL string
	// ServerFallbacks re-runs requests declined by safety classifiers on a
	// recommended model (Opus models).
	ServerFallbacks bool
	Timeout         time.Duration
}

// Provider is Claude.
type Provider struct {
	client          sdk.Client
	serverFallbacks bool
}

var _ llm.Provider = (*Provider)(nil)

// New returns the provider.
func New(o Options) *Provider {
	opts := []option.RequestOption{option.WithAPIKey(o.APIKey), option.WithMaxRetries(2)}
	if o.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(o.BaseURL))
	}
	if o.Timeout > 0 {
		opts = append(opts, option.WithRequestTimeout(o.Timeout))
	}
	return &Provider{client: sdk.NewClient(opts...), serverFallbacks: o.ServerFallbacks}
}

// Name implements llm.Provider.
func (p *Provider) Name() string { return "anthropic" }

// Local implements llm.Provider.
func (p *Provider) Local() bool { return false }

// DefaultModel implements llm.Provider: none for embeddings, which Anthropic
// does not offer.
func (p *Provider) DefaultModel(f llm.Feature) string {
	if f == llm.FeatureEmbedding {
		return ""
	}
	if m, ok := defaultModels[f]; ok {
		return m
	}
	return "claude-opus-5-5"
}

// Embed implements llm.Provider: Anthropic has no embeddings endpoint.
func (p *Provider) Embed(context.Context, []string) ([][]float32, string, error) {
	return nil, "", llm.ErrNotSupported
}

func (p *Provider) params(req llm.CompletionRequest) sdk.BetaMessageNewParams {
	params := sdk.BetaMessageNewParams{
		Model:     req.Model,
		MaxTokens: int64(req.MaxTokens),
		Messages:  messages(req.Messages),
	}
	if req.System != "" {
		// The system prompt carries the instructions and the project context
		// that repeat across turns: cache it.
		params.System = []sdk.BetaTextBlockParam{{Text: req.System, CacheControl: sdk.NewBetaCacheControlEphemeralParam()}}
	}
	if req.Effort != "" {
		params.OutputConfig.Effort = sdk.BetaOutputConfigEffort(req.Effort)
	}
	if req.JSONSchema != nil {
		params.OutputConfig.Format = sdk.BetaJSONOutputFormatParam{Schema: schema.ForDecoding(req.JSONSchema.JSON)}
	}
	for _, t := range req.Tools {
		props := t.InputSchema["properties"]
		var required []string
		if r, ok := t.InputSchema["required"].([]any); ok {
			for _, v := range r {
				if s, ok := v.(string); ok {
					required = append(required, s)
				}
			}
		}
		params.Tools = append(params.Tools, sdk.BetaToolUnionParam{OfTool: &sdk.BetaToolParam{
			Name:        t.Name,
			Description: sdk.String(t.Description),
			InputSchema: sdk.BetaToolInputSchemaParam{Properties: props, Required: required},
		}})
	}
	if req.Metadata.UserID != uuid.Nil {
		// An opaque id lets Anthropic tell users apart for abuse monitoring.
		sum := sha256.Sum256([]byte(req.Metadata.TenantID.String() + "/" + req.Metadata.UserID.String()))
		params.Metadata = sdk.BetaMetadataParam{UserID: sdk.String(hex.EncodeToString(sum[:16]))}
	}
	if p.serverFallbacks && strings.HasPrefix(req.Model, "claude-opus") {
		params.Fallbacks = sdk.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()}
		params.Betas = []sdk.AnthropicBeta{sdk.AnthropicBetaServerSideFallback2026_07_01}
	}
	return params
}

// messages converts a conversation, merging consecutive turns of one role.
func messages(in []llm.Message) []sdk.BetaMessageParam {
	var out []sdk.BetaMessageParam
	for _, m := range in {
		role := sdk.BetaMessageParamRoleUser
		if m.Role == llm.RoleAssistant {
			role = sdk.BetaMessageParamRoleAssistant
		}
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Content = append(out[n-1].Content, sdk.NewBetaTextBlock(m.Content))
			continue
		}
		out = append(out, sdk.BetaMessageParam{Role: role, Content: []sdk.BetaContentBlockParamUnion{sdk.NewBetaTextBlock(m.Content)}})
	}
	return out
}

// Complete implements llm.Provider.
func (p *Provider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	msg, err := p.client.Beta.Messages.New(ctx, p.params(req))
	if err != nil {
		return nil, mapError(err)
	}
	return convert(msg)
}

// Stream implements llm.Provider. A request that failed before the answer
// started (401, 429, 529, network) is returned as an error, so that the
// gateway can try the next provider of the route (С-15).
func (p *Provider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
	stream := p.client.Beta.Messages.NewStreaming(ctx, p.params(req))
	if err := stream.Err(); err != nil {
		_ = stream.Close()
		return nil, mapError(err)
	}
	ch := make(chan llm.StreamEvent, 16)
	go func() {
		defer close(ch)
		defer func() { _ = stream.Close() }()
		send := func(ev llm.StreamEvent) bool {
			select {
			case ch <- ev:
				return true
			case <-ctx.Done():
				return false
			}
		}
		var msg sdk.BetaMessage
		for stream.Next() {
			ev := stream.Current()
			if err := msg.Accumulate(ev); err != nil {
				send(llm.StreamEvent{Err: fmt.Errorf("anthropic: stream: %w", err)})
				return
			}
			if d, ok := ev.AsAny().(sdk.BetaRawContentBlockDeltaEvent); ok {
				if t, ok := d.Delta.AsAny().(sdk.BetaTextDelta); ok && t.Text != "" {
					if !send(llm.StreamEvent{Delta: t.Text}) {
						return
					}
				}
			}
		}
		if err := stream.Err(); err != nil {
			send(llm.StreamEvent{Err: mapError(err)})
			return
		}
		resp, err := convert(&msg)
		if err != nil {
			send(llm.StreamEvent{Err: err})
			return
		}
		send(llm.StreamEvent{Done: resp})
	}()
	return ch, nil
}

func convert(msg *sdk.BetaMessage) (*llm.CompletionResponse, error) {
	usage := usageOf(msg.Usage)
	if msg.StopReason == sdk.BetaStopReasonRefusal {
		return &llm.CompletionResponse{Provider: "anthropic", Model: msg.Model, StopReason: llm.StopRefusal, Usage: usage},
			fmt.Errorf("%w (category %q)", llm.ErrRefused, msg.StopDetails.Category)
	}
	resp := &llm.CompletionResponse{Provider: "anthropic", Model: msg.Model, Usage: usage}
	var text strings.Builder
	for i := range msg.Content {
		switch b := msg.Content[i].AsAny().(type) {
		case sdk.BetaTextBlock:
			text.WriteString(b.Text)
		case sdk.BetaToolUseBlock:
			input, err := json.Marshal(b.Input)
			if err != nil {
				return nil, fmt.Errorf("anthropic: tool input: %w", err)
			}
			resp.ToolCalls = append(resp.ToolCalls, llm.ToolCall{ID: b.ID, Name: b.Name, Input: input})
		}
	}
	resp.Text = text.String()
	switch msg.StopReason {
	case sdk.BetaStopReasonMaxTokens, sdk.BetaStopReasonModelContextWindowExceeded:
		resp.StopReason = llm.StopMaxTokens
	case sdk.BetaStopReasonToolUse:
		resp.StopReason = llm.StopToolUse
	default:
		resp.StopReason = llm.StopEnd
	}
	return resp, nil
}

// usageOf counts every attempt of a request: with server-side fallbacks the
// top-level usage covers only the attempt that answered.
func usageOf(u sdk.BetaUsage) llm.Usage {
	if len(u.Iterations) > 0 {
		var total llm.Usage
		for i := range u.Iterations {
			it := &u.Iterations[i]
			total = total.Add(llm.Usage{
				InputTokens: int(it.InputTokens), OutputTokens: int(it.OutputTokens),
				CacheReadTokens: int(it.CacheReadInputTokens), CacheWriteTokens: int(it.CacheCreationInputTokens),
			})
		}
		return total
	}
	return llm.Usage{
		InputTokens: int(u.InputTokens), OutputTokens: int(u.OutputTokens),
		CacheReadTokens: int(u.CacheReadInputTokens), CacheWriteTokens: int(u.CacheCreationInputTokens),
	}
}

func mapError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest, http.StatusNotFound, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
			return fmt.Errorf("%w: %w", llm.ErrBadRequest, err)
		}
	}
	return fmt.Errorf("%w: %w", llm.ErrUnavailable, err)
}
