package ai

import (
	"time"

	"github.com/yelnurq/startify/backend/internal/ai/anthropic"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/ai/openai"
	"github.com/yelnurq/startify/backend/internal/platform/config"
)

// ConfiguredProviders builds the providers the configuration enables. The
// mock is always available for the offline demo; mockDelay paces its stream.
func ConfiguredProviders(cfg config.AI, mockDelay time.Duration) []llm.Provider {
	providers := []llm.Provider{&mock.Provider{ChunkDelay: mockDelay}}
	if cfg.AnthropicAPIKey != "" {
		providers = append(providers, anthropic.New(anthropic.Options{
			APIKey: cfg.AnthropicAPIKey, BaseURL: cfg.AnthropicBaseURL,
			ServerFallbacks: cfg.AnthropicServerFallbacks, Timeout: cfg.RequestTimeout,
		}))
	}
	if cfg.OpenAIBaseURL != "" {
		providers = append(providers, openai.New(openai.Options{
			BaseURL: cfg.OpenAIBaseURL, APIKey: cfg.OpenAIAPIKey, Model: cfg.OpenAIModel,
			EmbeddingModel: cfg.OpenAIEmbeddingModel, Local: cfg.OpenAIIsLocal(), Timeout: cfg.RequestTimeout,
		}))
	}
	return providers
}
