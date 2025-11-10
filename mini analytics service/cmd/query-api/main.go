package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"tiny-analytics/internal/ch"
	"tiny-analytics/internal/config"
	"tiny-analytics/internal/httpx"
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

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(httpx.NewHTTPMetrics("query_api").Handler())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/v1/metrics/pageviews", func(c *gin.Context) {
		handleTimeseries(c, client, "pageviews")
	})
	router.GET("/v1/metrics/unique-users", func(c *gin.Context) {
		handleTimeseries(c, client, "unique-users")
	})
	router.GET("/v1/metrics/top-pages", func(c *gin.Context) {
		handleTopPages(c, client)
	})

	server := &http.Server{
		Addr:    cfg.QueryAddr,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("query api failed: %v", err)
		}
	}()

	waitForSignal()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func handleTimeseries(c *gin.Context, client *ch.Client, metric string) {
	siteID := c.Query("site_id")
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if siteID == "" || fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site_id, from, and to are required"})
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var series []ch.MetricPoint
	switch metric {
	case "pageviews":
		series, err = client.Pageviews(ctx, siteID, from, to)
	case "unique-users":
		series, err = client.UniqueUsers(ctx, siteID, from, to)
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown metric"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	resp := gin.H{
		"site_id": siteID,
		"from":    fromStr,
		"to":      toStr,
		"series":  toAPIseries(series),
	}
	c.Header("Cache-Control", "public, max-age=30")
	c.JSON(http.StatusOK, resp)
}

func handleTopPages(c *gin.Context, client *ch.Client) {
	siteID := c.Query("site_id")
	fromStr := c.Query("from")
	toStr := c.Query("to")
	limitStr := c.DefaultQuery("limit", "20")
	if siteID == "" || fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site_id, from, and to are required"})
		return
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be positive"})
		return
	}
	if limit > 100 {
		limit = 100
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	pages, err := client.TopPages(ctx, siteID, from, to, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	type page struct {
		URL   string `json:"url"`
		Views int64  `json:"views"`
	}
	resp := gin.H{
		"site_id": siteID,
		"from":    fromStr,
		"to":      toStr,
		"pages":   pages,
	}
	c.Header("Cache-Control", "public, max-age=30")
	c.JSON(http.StatusOK, resp)
}

func toAPIseries(points []ch.MetricPoint) []gin.H {
	result := make([]gin.H, 0, len(points))
	for _, p := range points {
		result = append(result, gin.H{
			"date":  p.Date.Format("2006-01-02"),
			"value": p.Value,
		})
	}
	return result
}

func waitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
