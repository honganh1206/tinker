package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/honganh1206/tinker/agent"
	"github.com/honganh1206/tinker/inference"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/tools"
)

const MaxRetries = 1

type SessionConfig struct {
	LLMBase    inference.BaseLLMClient
	MCPConfigs []mcp.ServerConfig
	Prompt     string
	VerifyCmd  string
	Verbose    bool
}

func RunSession(ctx context.Context, cfg SessionConfig) (*SessionResult, error) {
	startedAt := time.Now()
	logger := NewLogger(os.Stderr, cfg.Verbose)

	llm, err := inference.Init(ctx, cfg.LLMBase)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM: %w", err)
	}

	conv := message.NewConversation()

	toolBox := &tools.ToolBox{
		Tools: []*tools.ToolDefinition{
			&tools.ReadFileDefinition,
			&tools.ListFilesDefinition,
			&tools.EditFileDefinition,
			&tools.GrepSearchDefinition,
			&tools.FinderDefinition,
			&tools.BashDefinition,
		},
	}

	a := agent.New(&agent.Config{
		LLM:          llm,
		Conversation: conv,
		ToolBox:      toolBox,
		MCPConfigs:   cfg.MCPConfigs,
		Logger:       logger,
	})

	a.RegisterMCPServers()
	defer a.ShutdownMCPServers()

	onDelta := func(delta string) {
		logger.Debug("agent", "delta", delta)
	}

	logger.Info("running agent", "prompt", cfg.Prompt, "model", llm.ModelName(), "provider", llm.ProviderName())
	err = a.Run(ctx, cfg.Prompt, onDelta)
	if err != nil {
		return resultFromError(conv, cfg.Prompt, startedAt, err, llm), nil
	}

	retryCount := 0
	if cfg.VerifyCmd != "" {
		for attempt := 0; attempt <= MaxRetries; attempt++ {
			logger.Info("running verification", "cmd", cfg.VerifyCmd, "attempt", attempt+1)
			output, verifyErr := runVerifyCmd(ctx, cfg.VerifyCmd)

			if verifyErr == nil {
				logger.Info("verification passed")
				break
			}

			logger.Warn("verification failed", "error", verifyErr, "output_len", len(output))
			retryCount++

			if attempt < MaxRetries {
				retryPrompt := fmt.Sprintf(
					"The verification command `%s` failed. Output:\n\n```\n%s\n```\n\nPlease fix the issues and try again.",
					cfg.VerifyCmd, output,
				)
				logger.Info("retrying agent with failure context", "attempt", attempt+2)
				err = a.Run(ctx, retryPrompt, onDelta)
				if err != nil {
					return resultFromError(conv, cfg.Prompt, startedAt, err, llm), nil
				}
			} else {
				return buildResult(conv, cfg.Prompt, startedAt, StatusPartial, retryCount, "verification failed after max retries", llm), nil
			}
		}
	}

	return buildResult(conv, cfg.Prompt, startedAt, StatusSuccess, retryCount, "", llm), nil
}

func runVerifyCmd(ctx context.Context, cmdStr string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func resultFromError(conv *message.Conversation, prompt string, startedAt time.Time, err error, llm inference.LLMClient) *SessionResult {
	return buildResult(conv, prompt, startedAt, StatusFailed, 0, err.Error(), llm)
}

func buildResult(conv *message.Conversation, prompt string, startedAt time.Time, status Status, retryCount int, errMsg string, llm inference.LLMClient) *SessionResult {
	completedAt := time.Now()
	finalMessage := extractFinalMessage(conv)

	return &SessionResult{
		SessionID:    conv.ID,
		Status:       status,
		Prompt:       prompt,
		StartedAt:    startedAt,
		CompletedAt:  completedAt,
		DurationMs:   completedAt.Sub(startedAt).Milliseconds(),
		TokensUsed:   conv.TokenCount,
		RetryCount:   retryCount,
		FinalMessage: finalMessage,
		Error:        errMsg,
		Model:        llm.ModelName(),
		Provider:     llm.ProviderName(),
		Messages:     conv.Messages,
	}
}

func extractFinalMessage(conv *message.Conversation) string {
	for i := len(conv.Messages) - 1; i >= 0; i-- {
		msg := conv.Messages[i]
		if msg.Role == message.AssistantRole || msg.Role == message.ModelRole {
			for _, block := range msg.Content {
				if tb, ok := block.(message.TextBlock); ok && tb.Text != "" {
					return tb.Text
				}
			}
		}
	}
	return ""
}

func OutputResult(result *SessionResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
