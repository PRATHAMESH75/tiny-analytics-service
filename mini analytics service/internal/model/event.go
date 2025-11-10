package model

import "time"

// Event represents the payload accepted by the public ingest API.
type Event struct {
	SiteID    string         `json:"site_id" binding:"required"`
	EventName string         `json:"event_name" binding:"required"`
	URL       string         `json:"url"`
	Referrer  string         `json:"referrer"`
	UserID    string         `json:"user_id"`
	SessionID string         `json:"session_id"`
	TS        int64          `json:"ts"` // milliseconds epoch
	Props     map[string]any `json:"props"`
	IP        string         `json:"-"`
	UA        string         `json:"-"`
}

// RawEvent is the message stored in the raw Kafka topic.
type RawEvent struct {
	Event
	IP string `json:"ip"`
	UA string `json:"ua"`
}

// EnrichedEvent is the denormalized document ready for ClickHouse ingestion.
type EnrichedEvent struct {
	EventTime   time.Time      `json:"event_time"`
	EventDate   time.Time      `json:"event_date"`
	EventName   string         `json:"event_name"`
	UserID      string         `json:"user_id"`
	SessionID   string         `json:"session_id"`
	SiteID      string         `json:"site_id"`
	URL         string         `json:"url"`
	Referrer    string         `json:"referrer"`
	UTMSource   string         `json:"utm_source"`
	UTMMedium   string         `json:"utm_medium"`
	UTMCampaign string         `json:"utm_campaign"`
	Country     string         `json:"country"`
	DeviceType  string         `json:"device_type"`
	Browser     string         `json:"browser"`
	OS          string         `json:"os"`
	IPHash      string         `json:"ip_hash"`
	Payload     map[string]any `json:"payload"`
	IngestedAt  time.Time      `json:"_ingested_at"`
}

// NewRawEvent builds a RawEvent from a validated Event and server metadata.
func NewRawEvent(evt Event, ip, ua string) RawEvent {
	evt.IP = ip
	evt.UA = ua
	return RawEvent{
		Event: evt,
		IP:    ip,
		UA:    ua,
	}
}
