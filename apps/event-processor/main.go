package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func initTracer(ctx context.Context) func() {
	exp, err := otlptracegrpc.New(ctx)
	if err != nil {
		slog.Error("failed to create OTLP exporter", "err", err)
		os.Exit(1)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("event-processor"),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return func() { tp.Shutdown(ctx) }
}

func main() {
	ctx := context.Background()
	shutdown := initTracer(ctx)
	defer shutdown()

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

	sub, err := js.PullSubscribe("webhooks.events", "webhook-processor")
	if err != nil {
		slog.Error("failed to subscribe", "err", err)
		os.Exit(1)
	}
	slog.Info("event-processor ready, waiting for events")

	tracer := otel.Tracer("event-processor")

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
				// Extract trace context from NATS headers to continue the trace from webhook-service.
				// NATS canonicalizes header keys (e.g. "traceparent" → "Traceparent"),
				// so we normalize back to lowercase for the OTel propagator.
				carrier := propagation.MapCarrier{}
				for k := range msg.Header {
					carrier[strings.ToLower(k)] = msg.Header.Get(k)
				}
				ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

				_, span := tracer.Start(ctx, "event.process")

				var payload map[string]any
				json.Unmarshal(msg.Data, &payload)
				slog.InfoContext(ctx, "processing event", "payload", payload)
				msg.Ack()

				span.End()
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	slog.Info("event-processor shutting down")
}
