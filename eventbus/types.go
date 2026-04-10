package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Event represents a message on the event bus
type Event struct {
	// Ctx carries trace context extracted from headers
	// used for continuing the tracing
	Ctx context.Context

	Topic     string            `json:"topic"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
	// Event payload
	Data json.RawMessage `json:"data"`
}

type EventBus interface {
	// Send an event to the bus/topic?
	Publish(ctx context.Context, topic string, event *Event) error
	// Receive a channel that receives events from (or for?) a topic
	Subscribe(ctx context.Context, topic string) (<-chan *Event, error)
	// Shut down the bus connection
	Close() error
}

// Topics consumed by components
const (
	TopicChannelHealthUpdate = "channel.health.update"
	TopicChannelMessageRecv  = "channel.message.received"
	TopicChannelMessageSend  = "channel.message.send"
	// NOTE: Only needed when running multiple processes (NATs JetStream for communication)
	TopicAgentRunCompleted = "agent.run.completed"
)

// NewEvent creates a new event with the current timestamp
func NewEvent(topic string, metadata map[string]string, data any) (*Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshalling event data failed: %w", err)
	}
	return &Event{
		Topic:     topic,
		Timestamp: time.Now(),
		Metadata:  metadata,
		Data:      raw,
	}, nil
}
