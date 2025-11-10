package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kafkago "github.com/segmentio/kafka-go"

	"tiny-analytics/internal/config"
	ikafka "tiny-analytics/internal/kafka"
	"tiny-analytics/internal/model"
	"tiny-analytics/internal/pipeline"
)

var (
	msgsConsumed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "enricher_msgs_consumed_total",
		Help: "Total messages consumed from events.raw",
	})
	msgsProduced = promauto.NewCounter(prometheus.CounterOpts{
		Name: "enricher_msgs_produced_total",
		Help: "Total messages produced to events.enriched",
	})
	errorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "enricher_errors_total",
		Help: "Number of enrichment failures",
	})
	consumerLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "enricher_consumer_lag",
		Help: "Current consumer lag reported by kafka-go",
	})
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := ikafka.NewReader(cfg.KafkaBrokers, cfg.KafkaTopicRaw, "enricher-group")
	writer := ikafka.NewWriter(cfg.KafkaBrokers, cfg.KafkaTopicEnriched)
	defer reader.Close()
	defer writer.Close()

	go serveMetrics(cfg.EnricherMetricsAddr)
	go handleSignals(cancel)

	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			errorsTotal.Inc()
			log.Printf("read kafka: %v", err)
			time.Sleep(time.Second)
			continue
		}
		msgsConsumed.Inc()
		stats := reader.Stats()
		consumerLag.Set(float64(stats.Lag))

		var raw model.RawEvent
		if err := json.Unmarshal(m.Value, &raw); err != nil {
			errorsTotal.Inc()
			log.Printf("decode raw event: %v", err)
			continue
		}

		enriched, err := pipeline.Enrich(raw, cfg.IPHashSalt)
		if err != nil {
			errorsTotal.Inc()
			log.Printf("enrich event: %v", err)
			continue
		}
		payload, err := json.Marshal(enriched)
		if err != nil {
			errorsTotal.Inc()
			log.Printf("marshal enriched event: %v", err)
			continue
		}
		writeCtx, cancelWrite := context.WithTimeout(ctx, 10*time.Second)
		err = writer.WriteMessages(writeCtx, kafkago.Message{
			Key:   m.Key,
			Value: payload,
		})
		cancelWrite()
		if err != nil {
			errorsTotal.Inc()
			log.Printf("produce enriched event: %v", err)
			continue
		}
		msgsProduced.Inc()
	}
	log.Println("enricher shutdown complete")
}

func handleSignals(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()
}

func serveMetrics(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("metrics server failed: %v", err)
	}
}
