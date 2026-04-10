package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/honganh1206/tinker/channel"
	"github.com/honganh1206/tinker/eventbus"
	"github.com/honganh1206/tinker/inference"
	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/session"
	"github.com/honganh1206/tinker/store"
)

func main() {
	var provider string
	var model string
	var eventBusURL string

	flag.StringVar(&provider, "provider", string(inference.AnthropicProvider), "LLM provider (anthropic, gemini)")
	flag.StringVar(&model, "model", "", "LLM model name")

	flag.StringVar(&eventBusURL, "event-bus-url", os.Getenv("NATS_LOCAL_PORT"), "Event bus URL")
	flag.Parse()

	if model == "" {
		model = string(inference.GetDefaultModel(inference.ProviderName(provider)))
	}

	log := logger.NewLogger(os.Stderr, true)
	log.Info("runner starting...")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	bus, err := eventbus.NewNATSEventBus(eventBusURL)
	if err != nil {
		log.Error("failed to connect to event bus", "error", err)
		os.Exit(1)
	}
	defer bus.Close()

	ss, err := store.NewFileStore("")
	if err != nil {
		log.Error("failed to create file store", "error", err)
		os.Exit(1)
	}

	inboundCh, err := bus.Subscribe(ctx, eventbus.TopicChannelMessageRecv)
	if err != nil {
		log.Error("failed to subscribe to inbound messages", "error", err)
		os.Exit(1)
	}

	log.Info("runner listening for messages", "provider", provider, "model", model)

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

			cfg := session.SessionConfig{
				LLMBase: inference.ClientConfig{
					ProviderName: provider,
					ModelName:    model,
					TokenLimit:   8192,
				},
				Prompt:  msg.Text,
				Verbose: true,
			}

			result, err := session.RunSession(eventCtx, cfg)
			if err != nil {
				log.Error("agent session failed", "error", err)
				continue
			}

			if err := ss.Save(result); err != nil {
				log.Error("failed to save session", "error", err)
			}

			completed := channel.AgentRunCompleted{
				Channel:      msg.Channel,
				ChatID:       msg.ChatID,
				ReplyTo:      msg.Metadata["messageId"],
				FinalMessage: result.FinalMessage,
				Status:       string(result.Status),
				SessionID:    result.SessionID,
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

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
