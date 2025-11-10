//go:build e2e

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tiny-analytics/internal/ch"
)

func TestEndToEndPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}
	root := projectRoot(t)
	composeFile := filepath.Join(root, "deployments", "docker-compose.yml")
	runCommand(t, "docker", "compose", "-f", composeFile, "up", "-d", "--wait")
	defer runCommand(t, "docker", "compose", "-f", composeFile, "down", "-v")

	env := map[string]string{
		"KAFKA_BROKERS":     "localhost:9092",
		"CLICKHOUSE_DSN":    "clickhouse://default:@localhost:9000?database=default&dial_timeout=5s&compress=true",
		"SITES_CONFIG_PATH": filepath.Join(root, "config", "sites.dev.yml"),
	}

	ingest := startService(t, env, root, "go", "run", "./cmd/ingest-api")
	defer stopService(ingest)
	enricher := startService(t, env, root, "go", "run", "./cmd/consumer-enricher")
	defer stopService(enricher)
	loader := startService(t, env, root, "go", "run", "./cmd/loader")
	defer stopService(loader)
	query := startService(t, env, root, "go", "run", "./cmd/query-api")
	defer stopService(query)

	waitForHealth(t, "http://localhost:8080/healthz", time.Minute)
	waitForHealth(t, "http://localhost:8081/healthz", time.Minute)

	for i := 0; i < 5; i++ {
		event := map[string]any{
			"site_id":    "site-1",
			"event_name": "pageview",
			"url":        fmt.Sprintf("https://example.com/%d", i),
			"user_id":    fmt.Sprintf("user-%d", i),
			"session_id": fmt.Sprintf("sess-%d", i),
			"ts":         time.Now().Add(time.Duration(i) * time.Millisecond).UnixMilli(),
		}
		postJSON(t, "http://localhost:8080/v1/collect", event, map[string]string{
			"X-TA-API-Key": "dev-site-1-key",
		})
	}

	chClient, err := ch.New(context.Background(), env["CLICKHOUSE_DSN"])
	require.NoError(t, err)
	defer chClient.Close()

	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		count, err := chClient.CountEvents(ctx)
		if err != nil {
			return false
		}
		return count >= 5
	}, 2*time.Minute, 2*time.Second, "expected ClickHouse to receive events")

	today := time.Now().Format("2006-01-02")
	resp := getJSON(t, fmt.Sprintf("http://localhost:8081/v1/metrics/pageviews?site_id=site-1&from=%s&to=%s", today, today))
	series, ok := resp["series"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, series)
}

func runCommand(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())
}

func startService(t *testing.T, env map[string]string, workdir string, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	cmd.Dir = workdir
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	require.NoError(t, cmd.Start())
	return cmd
}

func projectRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}

func stopService(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
	case <-done:
	}
}

func waitForHealth(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	client := http.Client{Timeout: 2 * time.Second}
	require.Eventually(t, func() bool {
		resp, err := client.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, timeout, 2*time.Second, "endpoint %s never became healthy", url)
}

func postJSON(t *testing.T, url string, payload any, headers map[string]string) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, data)
	}
}

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}
