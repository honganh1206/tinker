// Package router is an implementation of channel router.
// Translates agent.run.completed events into channel.message.send events.
package router

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/honganh1206/tinker/internal/channel"
	"github.com/honganh1206/tinker/internal/eventbus"
	"github.com/honganh1206/tinker/internal/logger"
)

type Router struct {
	EventBus eventbus.EventBus
	Log      *logger.Logger
}

func NewRouter(eb eventbus.EventBus, logger *logger.Logger) *Router {
	return &Router{EventBus: eb, Log: logger}
}

func (r *Router) Start(ctx context.Context) error {
	r.Log.Info("starting router...")

	completedCh, err := r.EventBus.Subscribe(ctx, eventbus.TopicAgentRunCompleted)
	if err != nil {
		return fmt.Errorf("subscribing to %s: %w", eventbus.TopicAgentRunCompleted, err)
	}

	for {
		select {
		case <-ctx.Done():
			r.Log.Info("souter shutting down...")
			return nil
		case event := <-completedCh:
			r.handleCompleted(ctx, event)
		}
	}
}

func (r *Router) handleCompleted(ctx context.Context, event *eventbus.Event) {
	if event.Ctx != nil {
		ctx = event.Ctx
	}

	var completed channel.AgentRunCompleted
	if err := json.Unmarshal(event.Data, &completed); err != nil {
		r.Log.Error("failed to unmarshal agent run completed", "error", err)
		return
	}

	r.Log.Info("agent run completed",
		"channel", completed.Channel,
		"status", completed.Status,
		"sessionId", completed.SessionID,
		"text", truncateForLog(completed.FinalMessage, 80))

	outbound := channel.OutboundMessage{
		Channel: completed.Channel,
		ChatID:  completed.ChatID,
		ThreadID: completed.ThreadID,
		Text:    completed.FinalMessage,
		ReplyTo: completed.ReplyTo,
	}

	outEvent, err := eventbus.NewEvent(eventbus.TopicChannelMessageSend, event.Metadata, outbound)
	if err != nil {
		r.Log.Error("failed to create outbound event", "error", err)
		return
	}

	if err := r.EventBus.Publish(ctx, eventbus.TopicChannelMessageSend, outEvent); err != nil {
		r.Log.Error("failed to publish outbound message", "error", err)
	}
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
