package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kafkago "github.com/segmentio/kafka-go"

	"tiny-analytics/internal/auth"
	"tiny-analytics/internal/config"
	"tiny-analytics/internal/httpx"
	ikafka "tiny-analytics/internal/kafka"
	"tiny-analytics/internal/model"
	"tiny-analytics/internal/util"
)

const (
	apiKeyHeader      = "X-TA-API-Key"
	signatureHeader   = "X-TA-Signature"
	unknownSiteError  = "unknown site"
	missingKeyMessage = "missing or invalid api key"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("starting ingest API on %s", cfg.IngestAddr)
	writer := ikafka.NewWriter(cfg.KafkaBrokers, cfg.KafkaTopicRaw)
	defer writer.Close()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(httpx.NewHTTPMetrics("ingest_api").Handler())
	router.Use(httpx.CORSMiddleware(cfg.CORSAllowOrigins))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.POST("/v1/collect", func(c *gin.Context) {
		handleCollect(c, cfg, writer)
	})

	server := &http.Server{
		Addr:    cfg.IngestAddr,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ingest server failed: %v", err)
		}
	}()

	graceful(server)
}

func handleCollect(c *gin.Context, cfg config.Config, writer *kafkago.Writer) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var evt model.Event
	if err := json.Unmarshal(body, &evt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if evt.SiteID == "" || evt.EventName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site_id and event_name are required"})
		return
	}
	siteCred, ok := cfg.Sites[evt.SiteID]
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": unknownSiteError})
		return
	}
	apiKey := c.GetHeader(apiKeyHeader)
	if apiKey == "" || apiKey != siteCred.APIKey {
		c.JSON(http.StatusUnauthorized, gin.H{"error": missingKeyMessage})
		return
	}
	secret := siteCred.HMACSecret
	if secret == "" {
		secret = cfg.HMACSecret
	}
	if secret != "" {
		sig := c.GetHeader(signatureHeader)
		if sig == "" || !auth.VerifySignature(secret, body, sig) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
	}

	if evt.Props == nil {
		evt.Props = map[string]any{}
	}
	if evt.TS == 0 {
		evt.TS = time.Now().UnixMilli()
	}
	evt.IP = c.ClientIP()
	evt.UA = c.GetHeader("User-Agent")

	if util.IsBot(evt.UA, cfg.BotUserAgents) {
		c.JSON(http.StatusAccepted, gin.H{"status": "ignored"})
		return
	}

	raw := model.NewRawEvent(evt, evt.IP, evt.UA)
	payload, err := json.Marshal(raw)
	if err != nil {
		log.Printf("marshal raw event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode event"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(evt.SiteID),
		Value: payload,
	}); err != nil {
		log.Printf("write kafka: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "queue unavailable"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}

func graceful(server *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down ingest API...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
