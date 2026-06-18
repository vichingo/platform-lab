package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

func main() {
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

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			requestsTotal.WithLabelValues("error").Inc()
			requestDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		data, _ := json.Marshal(payload)
		if err := nc.Publish("webhooks.events", data); err != nil {
			slog.Error("failed to publish to NATS", "err", err)
			requestsTotal.WithLabelValues("error").Inc()
			requestDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		slog.Info("webhook published", "subject", "webhooks.events", "payload", payload)
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
