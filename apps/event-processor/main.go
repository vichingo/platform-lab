package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		slog.Error("failed to connect to NATS", "url", natsURL, "err", err)
		os.Exit(1)
	}
	defer nc.Close()
	slog.Info("connected to NATS", "url", natsURL)

	js, err := nc.JetStream()
	if err != nil {
		slog.Error("failed to get JetStream context", "err", err)
		os.Exit(1)
	}

	// Create stream if it doesn't exist
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "WEBHOOKS",
		Subjects: []string{"webhooks.events"},
		MaxAge:   24 * time.Hour,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		slog.Error("failed to create stream", "err", err)
		os.Exit(1)
	}
	slog.Info("WEBHOOKS stream ready")

	// Durable pull consumer — KEDA watches this consumer's pending count
	sub, err := js.PullSubscribe("webhooks.events", "webhook-processor")
	if err != nil {
		slog.Error("failed to subscribe", "err", err)
		os.Exit(1)
	}
	slog.Info("event-processor ready, waiting for events")

	go func() {
		for {
			msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				slog.Error("fetch error", "err", err)
				continue
			}
			for _, msg := range msgs {
				var payload map[string]any
				json.Unmarshal(msg.Data, &payload)
				slog.Info("processing event", "payload", payload)
				msg.Ack()
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	slog.Info("event-processor shutting down")
}
