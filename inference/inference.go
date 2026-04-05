package inference

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/prompts"
	"github.com/honganh1206/tinker/tools"
	"google.golang.org/genai"
)

type Request struct {
	Messages     []*message.Message
	Tools        []*tools.ToolDefinition
	SystemPrompt string
	MaxTokens    int64
}

type LLMClient interface {
	Generate(ctx context.Context, req Request) (*message.Message, error)
	CountTokens(ctx context.Context, req Request) (int, error)
	Provider() string
	Model() string
}

type ClientConfig struct {
	ProviderName string
	ModelName    string
	TokenLimit   int64
}

func Init(ctx context.Context, cfg ClientConfig) (LLMClient, error) {
	switch cfg.ProviderName {
	case AnthropicProvider:
		client := anthropic.NewClient()
		sysPrompt := prompts.SystemPrompt()
		return NewAnthropicClient(&client, ModelVersion(cfg.ModelName), cfg.TokenLimit, sysPrompt), nil
	case GoogleProvider:
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  os.Getenv("GOOGLE_API_KEY"),
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}
		sysPrompt := prompts.SystemPrompt()
		return NewGeminiClient(client, ModelVersion(cfg.ModelName), cfg.TokenLimit, sysPrompt), nil
	default:
		return nil, fmt.Errorf("unknown model provider: %s", cfg.ProviderName)
	}
}

func ListAvailableModels(provider ProviderName) []ModelVersion {
	switch provider {
	case AnthropicProvider:
		return []ModelVersion{
			Claude4Opus,
			Claude4Sonnet,
			Claude35Sonnet,
			Claude35Haiku,
			Claude3Opus,
			Claude3Sonnet, // FIXME: Deprecated soon
			Claude3Haiku,
		}
	case GoogleProvider:
		return []ModelVersion{
			Gemini3Pro,
			Gemini25Pro,
			Gemini25Flash,
			Gemini20Flash,
			Gemini20FlashLite,
			Gemini15Pro,
			Gemini15Flash,
		}
	default:
		return []ModelVersion{}
	}
}

func GetDefaultModel(provider ProviderName) ModelVersion {
	switch provider {
	case AnthropicProvider:
		return Claude46Opus
	case GoogleProvider:
		return Gemini3Pro
	default:
		return ""
	}
}


