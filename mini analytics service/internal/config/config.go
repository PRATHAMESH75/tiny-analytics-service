package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds shared service configuration sourced from environment variables.
type Config struct {
	IngestAddr          string
	QueryAddr           string
	EnricherMetricsAddr string
	LoaderMetricsAddr   string
	KafkaBrokers        []string
	KafkaTopicRaw       string
	KafkaTopicEnriched  string
	ClickHouseDSN       string
	HMACSecret          string
	IPHashSalt          string
	CORSAllowOrigins    []string
	BotUserAgents       []string
	BatchSize           int
	BatchInterval       time.Duration
	Sites               map[string]SiteCredential
	SitesConfigPath     string
}

// SiteCredential defines API key / HMAC secrets for a tenant site.
type SiteCredential struct {
	APIKey     string `yaml:"api_key"`
	HMACSecret string `yaml:"hmac_secret"`
}

type sitesFile struct {
	Sites map[string]SiteCredential `yaml:"sites"`
}

// Load parses process environment variables into a Config struct, applying defaults when unset.
func Load() (Config, error) {
	path := getenv("SITES_CONFIG_PATH", "config/sites.dev.yml")
	sites, err := loadSitesConfig(path)
	if err != nil {
		return Config{}, fmt.Errorf("load sites config: %w", err)
	}

	cfg := Config{
		IngestAddr:          getenv("INGEST_ADDR", ":8080"),
		QueryAddr:           getenv("QUERY_ADDR", ":8081"),
		EnricherMetricsAddr: getenv("ENRICHER_METRICS_ADDR", ":9100"),
		LoaderMetricsAddr:   getenv("LOADER_METRICS_ADDR", ":9101"),
		KafkaBrokers:        splitAndTrim(getenv("KAFKA_BROKERS", "localhost:9092")),
		KafkaTopicRaw:       getenv("KAFKA_TOPIC_RAW", "events.raw"),
		KafkaTopicEnriched:  getenv("KAFKA_TOPIC_ENRICHED", "events.enriched"),
		ClickHouseDSN:       getenv("CLICKHOUSE_DSN", "clickhouse://default:@localhost:9000?database=default&dial_timeout=5s&compress=true&allow_experimental_object_type=1"),
		HMACSecret:          os.Getenv("HMAC_SECRET"),
		IPHashSalt:          getenv("IP_HASH_SALT", "dev-salt"),
		CORSAllowOrigins:    splitAndTrimAllowEmpty(getenv("CORS_ALLOW_ORIGINS", "*")),
		BotUserAgents:       splitAndTrimAllowEmpty(getenv("BOT_UA_DENYLIST", "bot,crawler,spider")),
		BatchSize:           atoiDefault("LOADER_BATCH_SIZE", 1000),
		BatchInterval:       durationDefault("LOADER_BATCH_INTERVAL_MS", 800),
		Sites:               sites,
		SitesConfigPath:     path,
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return def
}

func splitAndTrim(v string) []string {
	return splitAndTrimAllowEmpty(v)
}

func splitAndTrimAllowEmpty(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func atoiDefault(key string, def int) int {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return def
}

func durationDefault(key string, defMS int) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(val); err == nil {
			return time.Duration(parsed) * time.Millisecond
		}
	}
	return time.Duration(defMS) * time.Millisecond
}

func loadSitesConfig(path string) (map[string]SiteCredential, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file sitesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if len(file.Sites) == 0 {
		return nil, fmt.Errorf("no sites configured in %s", path)
	}
	out := make(map[string]SiteCredential, len(file.Sites))
	for id, cred := range file.Sites {
		if strings.TrimSpace(id) == "" {
			continue
		}
		if cred.APIKey == "" {
			return nil, fmt.Errorf("site %s missing api_key in %s", id, path)
		}
		out[id] = cred
	}
	return out, nil
}
