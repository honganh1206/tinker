// Entry point for the Discord channel instance
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/honganh1206/tinker/channel"
	"github.com/honganh1206/tinker/eventbus"
	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/router"
)

type DiscordChannel struct {
	channel.BaseChannel
	// Connection to Discord API
	session *discordgo.Session
	healthy bool
}

func main() {
	var instanceName string
	var botToken string
	var listenAddr string
	var eventBusURL string

	flag.StringVar(&instanceName, "instance", os.Getenv("INSTANCE_NAME"), "Tinker instance name")
	flag.StringVar(&botToken, "bot-token", os.Getenv("DISCORD_BOT_TOKEN"), "Discord bot token")
	flag.StringVar(&listenAddr, "addr", ":8080", "Listen address for health endpoint")
	flag.StringVar(&eventBusURL, "event-bus-url", os.Getenv("NATS_LOCAL_PORT"), "Event bus URL")
	flag.Parse()

	if botToken == "" {
		fmt.Fprintln(os.Stderr, "DISCORD_BOT_TOKEN is required")
		os.Exit(1)
	}

	log := logger.NewLogger(os.Stderr, true)

	bus, err := eventbus.NewNATSEventBus(eventBusURL)
	if err != nil {
		log.Error("failed to connect to event bus", "error", err)
		os.Exit(1)
	}
	defer bus.Close()

	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Error("failed to create discord session", "error", err)
		os.Exit(1)
	}

	dc := &DiscordChannel{
		BaseChannel: channel.BaseChannel{
			ChannelType:  "discord",
			InstanceName: instanceName,
			EventBus:     bus,
		},
		session: dg,
	}

	// Guild messages and DMs
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentDirectMessages | discordgo.IntentMessageContent

	// Register message handler
	dg.AddHandler(dc.messageCreate)

	// Open WebSocket connection
	if err := dg.Open(); err != nil {
		log.Error("failed to open discord gateway", "error", err)
		os.Exit(1)
	}
	defer dg.Close()

	dc.healthy = true
	log.Info("Discord channel connected", "instance", instanceName, "user", dg.State.User.Username)

	_ = dc.PublishHealth(context.Background(), channel.HealthStatus{Connected: true})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go dc.handleOutbound(ctx)

	r := &router.Router{
		EventBus: bus,
		Log:      log,
	}
	go r.Start(ctx)

	// Health server
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if dc.healthy {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("health server failed", "err", err)
	}
}

// discordgo handler for MESSAGE_CREATE events.
func (dc *DiscordChannel) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messsages from the bot
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Skip empty messages
	if m.Content == "" {
		return
	}

	msg := channel.InboundMessage{
		SenderID:   m.Author.ID,
		SenderName: m.Author.Username,
		ChatID:     m.ChannelID,
		Text:       m.Content,
		Metadata: map[string]string{
			"messageId": m.ID,
			"guildId":   m.GuildID,
		},
	}

	if err := dc.PublishInbound(context.Background(), msg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to publish inbound: %v\n", err)
	}
}

// Subcribe to outbound messages and sends them via Discord
func (dc *DiscordChannel) handleOutbound(ctx context.Context) {
	events, err := dc.SubscribeOutbound(ctx)
	if err != nil {
		// Format and write to stderr
		fmt.Fprintf(os.Stderr, "failed to subscribe to outbound: %v\n", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			var msg channel.OutboundMessage
			if err := json.Unmarshal(event.Data, &msg); err != nil {
				continue
			}
			if msg.Channel != "discord" {
				continue
			}
			if err := dc.sendMessage(msg); err != nil {
				fmt.Fprintf(os.Stderr, "failed to send discord message: %v\n", err)
			}
		}
	}
}

// Send a message to a Discord channel
func (dc *DiscordChannel) sendMessage(msg channel.OutboundMessage) error {
	_, err := dc.session.ChannelMessageSend(msg.ChatID, msg.Text)
	return err
}
