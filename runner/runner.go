// Implementation of agent runner
// NOTE:: If we have to deal with multiple processes i.e., moving to K8S
// then we go for channel router implementation,
// which runs on a separate controller process.
package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/honganh1206/tinker/channel"
	"github.com/honganh1206/tinker/eventbus"
	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/session"
	"github.com/honganh1206/tinker/store"
)

// NOTE:: This interface is declared in k8s sigs io package.
// Remove when we move to channel router.
type Runnable interface {
	Start(ctx context.Context) error
}

type Runner struct {
	EventBus      eventbus.EventBus
	Store         store.Store
	Log           *logger.Logger
	SessionConfig session.SessionConfig
}

func (r *Runner) Start(ctx context.Context) error {
	r.Log.Info("Starting runner...")

	inboundCh, err := r.EventBus.Subscribe(ctx, eventbus.TopicChannelMessageRecv)
	if err != nil {
		return fmt.Errorf("subscribing to %s: %w", eventbus.TopicChannelMessageRecv, err)
	}

	for {
		select {
		case <-ctx.Done():
			r.Log.Info("Runner shutting down")
			return nil
		case event := <-inboundCh:
			r.handleInbound(ctx, event)
		}
	}
}

func (r *Runner) handleInbound(ctx context.Context, event *eventbus.Event) {
	if event.Ctx != nil {
		ctx = event.Ctx
	}

	var msg channel.InboundMessage
	if err := json.Unmarshal(event.Data, &msg); err != nil {
		r.Log.Error("failed to unmarshal inbound message", "error", err)
		return
	}

	if msg.Text == "" {
		r.Log.Info("Skipping empty inbound message", "channel", msg.Channel)
		return
	}

	r.Log.Info("Received channel message", "channel", msg.Channel,
		"instance", msg.InstanceName,
		"sender", msg.SenderName,
		"text", truncateForLog(msg.Text, 80))

	go func() {
		cfg := r.SessionConfig
		cfg.Prompt = msg.Text

		result, err := session.RunSession(ctx, cfg)
		if err != nil {
			r.Log.Error("agent session failed", "error", err)
			return
		}

		if r.Store != nil {
			if err := r.Store.Save(result); err != nil {
				r.Log.Error("failed to save session", "error", err)
			}
		}

		outbound := channel.OutboundMessage{
			Channel: msg.Channel,
			ChatID:  msg.ChatID,
			Text:    result.FinalMessage,
			ReplyTo: msg.Metadata["messageId"],
		}

		outEvent, err := eventbus.NewEvent(eventbus.TopicChannelMessageSend, event.Metadata, outbound)
		if err != nil {
			r.Log.Error("failed to create outbound event", "error", err)
			return
		}

		if err := r.EventBus.Publish(ctx, eventbus.TopicChannelMessageSend, outEvent); err != nil {
			r.Log.Error("failed to publish outbound message", "error", err)
		}
	}()
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
