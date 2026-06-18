package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_requests_total",
		Help: "Total number of webhook requests",
	}, []string{"status"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webhook_request_duration_seconds",
		Help:    "Webhook request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"status"})
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
			semconv.ServiceName("webhook-service"),
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

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		slog.Error("API_KEY env var not set")
		os.Exit(1)
	}

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

	tracer := otel.Tracer("webhook-service")

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.Handle("GET /metrics", promhttp.Handler())

	mux.HandleFunc("POST /webhook", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		if r.Header.Get("X-API-Key") != apiKey {
			slog.Warn("unauthorized webhook request", "remote", r.RemoteAddr)
			requestsTotal.WithLabelValues("unauthorized").Inc()
			requestDuration.WithLabelValues("unauthorized").Observe(time.Since(start).Seconds())
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(r.Context(), "webhook.receive")
		defer span.End()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			requestsTotal.WithLabelValues("error").Inc()
			requestDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Inject trace context into NATS headers so event-processor can continue the trace
		carrier := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(ctx, carrier)
		msg := &nats.Msg{
			Subject: "webhooks.events",
			Header:  make(nats.Header),
		}
		for k, v := range carrier {
			msg.Header.Set(k, v)
		}
		data, _ := json.Marshal(payload)
		msg.Data = data

		if err := nc.PublishMsg(msg); err != nil {
			slog.Error("failed to publish to NATS", "err", err)
			requestsTotal.WithLabelValues("error").Inc()
			requestDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		slog.InfoContext(ctx, "webhook published", "subject", "webhooks.events", "payload", payload)
		requestsTotal.WithLabelValues("success").Inc()
		requestDuration.WithLabelValues("success").Observe(time.Since(start).Seconds())

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	slog.Info("webhook-service starting", "addr", ":8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
