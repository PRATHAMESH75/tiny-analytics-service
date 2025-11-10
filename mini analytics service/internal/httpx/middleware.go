package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPMetrics bundles common Prometheus collectors for HTTP services.
type HTTPMetrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
	Errors   *prometheus.CounterVec
	InFlight prometheus.Gauge
}

// NewHTTPMetrics registers the collectors for a specific service label.
func NewHTTPMetrics(service string) *HTTPMetrics {
	labels := prometheus.Labels{"service": service}
	return &HTTPMetrics{
		Requests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total HTTP requests received",
			ConstLabels: labels,
		}, []string{"method", "path", "status"}),
		Duration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "Latency distribution of HTTP requests",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}, []string{"method", "path"}),
		Errors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "http_errors_total",
			Help:        "Total HTTP errors returned",
			ConstLabels: labels,
		}, []string{"method", "path", "status"}),
		InFlight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "http_in_flight_requests",
			Help:        "Number of in-flight HTTP requests",
			ConstLabels: labels,
		}),
	}
}

// Handler returns a gin middleware that records metrics per request.
func (m *HTTPMetrics) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		m.InFlight.Inc()
		c.Next()
		elapsed := time.Since(start).Seconds()
		status := c.Writer.Status()
		method := c.Request.Method

		m.Requests.WithLabelValues(method, path, statusCode(status)).Inc()
		m.Duration.WithLabelValues(method, path).Observe(elapsed)
		if status >= 400 {
			m.Errors.WithLabelValues(method, path, statusCode(status)).Inc()
		}
		m.InFlight.Dec()
	}
}

func statusCode(code int) string {
	return strconv.Itoa(code)
}

// CORSMiddleware applies a simple allow-list policy.
func CORSMiddleware(allowed []string) gin.HandlerFunc {
	allowAll := len(allowed) == 0
	for _, o := range allowed {
		if o == "*" {
			allowAll = true
			break
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowAll {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && containsOrigin(allowed, origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-TA-Signature")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func containsOrigin(allowed []string, origin string) bool {
	for _, o := range allowed {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}
