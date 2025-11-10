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

	"tiny-analytics/internal/ch"
	"tiny-analytics/internal/config"
	ikafka "tiny-analytics/internal/kafka"
	"tiny-analytics/internal/model"
	"tiny-analytics/pkg/batcher"
)

var (
	batchSizeHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "loader_batch_size",
		Help:    "Histogram of ClickHouse batch sizes",
		Buckets: []float64{1, 10, 50, 100, 250, 500, 1000, 2000},
	})
	insertDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "loader_insert_duration_seconds",
		Help:    "Duration of ClickHouse insert operations",
		Buckets: prometheus.DefBuckets,
	})
	insertErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "loader_insert_errors_total",
		Help: "Total ClickHouse insert failures",
	})
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := ch.New(ctx, cfg.ClickHouseDSN)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	defer client.Close()
	if err := client.EnsureSchema(ctx); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}

	reader := ikafka.NewReader(cfg.KafkaBrokers, cfg.KafkaTopicEnriched, "loader-group")
	defer reader.Close()

	flusher := func(events []model.EnrichedEvent) error {
		return insertWithRetry(ctx, client, events)
	}
	b := batcher.New[model.EnrichedEvent](cfg.BatchSize, cfg.BatchInterval, flusher)
	defer b.Close()

	go serveMetrics(cfg.LoaderMetricsAddr)
	go handleSignals(cancel)

	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Printf("read enriched message: %v", err)
			time.Sleep(time.Second)
			continue
		}
		var evt model.EnrichedEvent
		if err := json.Unmarshal(m.Value, &evt); err != nil {
			log.Printf("decode enriched event: %v", err)
			continue
		}
		if err := b.Add(evt); err != nil {
			log.Printf("batch add failed: %v", err)
		}
	}
	log.Println("loader shutdown complete")
}

func insertWithRetry(ctx context.Context, client *ch.Client, events []model.EnrichedEvent) error {
	const maxAttempts = 5
	backoff := 200 * time.Millisecond
	start := time.Now()
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		insertCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := client.InsertBatch(insertCtx, events)
		cancel()
		if err == nil {
			insertDuration.Observe(time.Since(start).Seconds())
			batchSizeHistogram.Observe(float64(len(events)))
			return nil
		}
		insertErrors.Inc()
		if attempt == maxAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
	return nil
}

func serveMetrics(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{Addr: addr, Handler: mux}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("loader metrics server failed: %v", err)
	}
}

func handleSignals(cancel context.CancelFunc) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	cancel()
}
