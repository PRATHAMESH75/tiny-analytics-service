package ch

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"

	"tiny-analytics/internal/model"
)

// Client wraps a ClickHouse connection.
type Client struct {
	db *sql.DB
}

// New creates a ClickHouse client from a DSN.
func New(ctx context.Context, dsn string) (*Client, error) {
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &Client{db: db}, nil
}

// Close releases database resources.
func (c *Client) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// EnsureSchema creates the events table if it does not exist.
func (c *Client) EnsureSchema(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS events
(
  event_time       DateTime64(3, 'UTC'),
  event_date       Date,
  event_name       LowCardinality(String),
  user_id          String,
  session_id       String,
  site_id          LowCardinality(String),
  url              String,
  referrer         String,
  utm_source       LowCardinality(String),
  utm_medium       LowCardinality(String),
  utm_campaign     LowCardinality(String),
  country          LowCardinality(String),
  device_type      LowCardinality(String),
  browser          LowCardinality(String),
  os               LowCardinality(String),
  ip_hash          FixedString(64),
  payload          JSON,
  _ingested_at     DateTime64(3, 'UTC')
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (site_id, event_date, user_id, event_name, event_time)`
	_, err := c.db.ExecContext(ctx, ddl)
	return err
}

// InsertBatch writes a batch of enriched events with a single prepared statement.
func (c *Client) InsertBatch(ctx context.Context, events []model.EnrichedEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO events (
	event_time, event_date, event_name, user_id, session_id, site_id,
	url, referrer, utm_source, utm_medium, utm_campaign,
	country, device_type, browser, os, ip_hash, payload, _ingested_at
) VALUES (
	?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, evt := range events {
		payload, err := json.Marshal(evt.Payload)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := stmt.ExecContext(
			ctx,
			evt.EventTime,
			evt.EventDate,
			evt.EventName,
			evt.UserID,
			evt.SessionID,
			evt.SiteID,
			evt.URL,
			evt.Referrer,
			evt.UTMSource,
			evt.UTMMedium,
			evt.UTMCampaign,
			evt.Country,
			evt.DeviceType,
			evt.Browser,
			evt.OS,
			evt.IPHash,
			string(payload),
			evt.IngestedAt,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// MetricPoint represents a time-series datapoint.
type MetricPoint struct {
	Date  time.Time `json:"date"`
	Value int64     `json:"value"`
}

// TopPage holds aggregated top URL data.
type TopPage struct {
	URL   string `json:"url"`
	Views int64  `json:"views"`
}

// Pageviews returns daily counts for a site.
func (c *Client) Pageviews(ctx context.Context, siteID string, from, to time.Time) ([]MetricPoint, error) {
	rows, err := c.db.QueryContext(ctx, `
SELECT event_date, count() AS views
FROM events
WHERE site_id = ? AND event_date BETWEEN ? AND ?
GROUP BY event_date
ORDER BY event_date ASC`, siteID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var series []MetricPoint
	for rows.Next() {
		var date time.Time
		var value int64
		if err := rows.Scan(&date, &value); err != nil {
			return nil, err
		}
		series = append(series, MetricPoint{Date: date, Value: value})
	}
	return series, rows.Err()
}

// UniqueUsers returns daily unique counts.
func (c *Client) UniqueUsers(ctx context.Context, siteID string, from, to time.Time) ([]MetricPoint, error) {
	rows, err := c.db.QueryContext(ctx, `
SELECT event_date, uniqExact(user_id) AS uniques
FROM events
WHERE site_id = ? AND event_date BETWEEN ? AND ?
GROUP BY event_date
ORDER BY event_date ASC`, siteID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var series []MetricPoint
	for rows.Next() {
		var date time.Time
		var value int64
		if err := rows.Scan(&date, &value); err != nil {
			return nil, err
		}
		series = append(series, MetricPoint{Date: date, Value: value})
	}
	return series, rows.Err()
}

// TopPages returns the top URLs for the timeframe.
func (c *Client) TopPages(ctx context.Context, siteID string, from, to time.Time, limit int) ([]TopPage, error) {
	rows, err := c.db.QueryContext(ctx, `
SELECT url, count() AS views
FROM events
WHERE site_id = ? AND event_date BETWEEN ? AND ?
GROUP BY url
ORDER BY views DESC
LIMIT ?`, siteID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopPage
	for rows.Next() {
		var record TopPage
		if err := rows.Scan(&record.URL, &record.Views); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// CountEvents returns the total rows, useful for tests.
func (c *Client) CountEvents(ctx context.Context) (int64, error) {
	row := c.db.QueryRowContext(ctx, `SELECT count() FROM events`)
	var total int64
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// Ping ensures the database is reachable.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.db.PingContext(ctx); err != nil {
		return fmt.Errorf("clickhouse ping: %w", err)
	}
	return nil
}
