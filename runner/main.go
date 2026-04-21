package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/honganh1206/tinker/agent"
	"github.com/honganh1206/tinker/channel"
	"github.com/honganh1206/tinker/eventbus"
	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/model"
	"github.com/honganh1206/tinker/storage"
	"github.com/honganh1206/tinker/tools"
)

func main() {
	var provider string
	var modelName string
	var eventBusURL string

	flag.StringVar(&provider, "provider", "anthropic", "LLM provider (anthropic, gemini)")
	flag.StringVar(&modelName, "model", string(model.Claude46Sonnet), "LLM model name")
	flag.StringVar(&eventBusURL, "event-bus-url", os.Getenv("NATS_LOCAL_PORT"), "Event bus URL")
	flag.Parse()

	log := logger.NewLogger(os.Stderr, true)
	log.Info("runner starting...")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	bus, err := eventbus.NewNATSEventBus(eventBusURL)
	if err != nil {
		log.Error("failed to connect to event bus", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := bus.Close(); err != nil {
			log.Error("failed to close event bus", "error", err)
		}
	}()

	inboundCh, err := bus.Subscribe(ctx, eventbus.TopicChannelMessageRecv)
	if err != nil {
		log.Error("failed to subscribe to inbound messages", "error", err)
		os.Exit(1)
	}

	log.Info("runner listening for messages", "provider", provider, "model", modelName)

	for {
		select {
		case <-ctx.Done():
			log.Info("runner shutting down")
			return
		case event := <-inboundCh:
			eventCtx := ctx
			if event.Ctx != nil {
				eventCtx = event.Ctx
			}

			var msg channel.InboundMessage
			if err := json.Unmarshal(event.Data, &msg); err != nil {
				log.Error("failed to unmarshal inbound message", "error", err)
				continue
			}

			if msg.Text == "" {
				log.Info("skipping empty inbound message", "channel", msg.Channel)
				continue
			}

			log.Info("received channel message",
				"channel", msg.Channel,
				"sender", msg.SenderName,
				"text", truncateForLog(msg.Text, 80))

			finalMessage, err := handleMessage(eventCtx, model.ModelVersion(modelName), msg.Text, log)
			if err != nil {
				log.Error("agent run failed", "error", err)
				continue
			}

			completed := channel.AgentRunCompleted{
				Channel:      msg.Channel,
				ChatID:       msg.ChatID,
				ReplyTo:      msg.Metadata["messageId"],
				FinalMessage: finalMessage,
				Status:       "success",
			}

			doneEvent, err := eventbus.NewEvent(eventbus.TopicAgentRunCompleted, event.Metadata, completed)
			if err != nil {
				log.Error("failed to create completed event", "error", err)
				continue
			}

			if err := bus.Publish(eventCtx, eventbus.TopicAgentRunCompleted, doneEvent); err != nil {
				log.Error("failed to publish completed event", "error", err)
			}
		}
	}
}

func handleMessage(ctx context.Context, modelVersion model.ModelVersion, prompt string, log *logger.Logger) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dbPath := filepath.Join(home, ".tinker", "test.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return "", err
	}
	db, err := storage.NewContextDB(dbPath)
	if err != nil {
		return "", err
	}

	llm, err := model.NewClaudeModel(modelVersion)
	if err != nil {
		if err = db.Close(); err != nil {
			return "", fmt.Errorf("closing db: %w", err)
		}
		return "", err
	}

	cw, err := model.NewContextWindow(db, llm, "")
	if err != nil {
		if err = db.Close(); err != nil {
			return "", fmt.Errorf("closing db: %w", err)
		}
		return "", err
	}
	defer cw.Close()

	builtinTools := []tools.ToolDefinition{
		tools.ReadFileDefinition,
		tools.ListFilesDefinition,
		tools.EditFileDefinition,
		tools.GrepSearchDefinition,
		tools.FinderDefinition,
		tools.BashDefinition,
		tools.WebSearchDefinition,
		tools.ReadWebPageDefinition,
	}

	for _, t := range builtinTools {
		if err := cw.RegisterTool(t); err != nil {
			return "", err
		}
	}

	a := agent.New(&agent.Config{
		ContextWindow: cw,
		Logger:        log,
	})

	a.CW.SetSystemPrompt(model.SystemPrompt())

	return a.Run(ctx, prompt)
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
