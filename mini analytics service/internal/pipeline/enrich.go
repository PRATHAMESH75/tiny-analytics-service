package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"time"

	"tiny-analytics/internal/model"
	"tiny-analytics/internal/util"
)

// Enrich transforms a raw event into the ClickHouse-ready schema.
func Enrich(raw model.RawEvent, ipSalt string) (model.EnrichedEvent, error) {
	eventTime := time.UnixMilli(raw.TS).UTC()
	if raw.TS == 0 {
		eventTime = time.Now().UTC()
	}
	eventDate := time.Date(eventTime.Year(), eventTime.Month(), eventTime.Day(), 0, 0, 0, 0, time.UTC)
	utmSource, utmMedium, utmCampaign := parseUTM(raw.URL)

	deviceType := util.ParseDeviceType(raw.UA)
	browser := util.ParseBrowser(raw.UA)
	os := util.ParseOS(raw.UA)

	payload := raw.Props
	if payload == nil {
		payload = map[string]any{}
	}

	ipHash := hashIP(ipSalt, raw.IP)

	return model.EnrichedEvent{
		EventTime:   eventTime,
		EventDate:   eventDate,
		EventName:   raw.EventName,
		UserID:      raw.UserID,
		SessionID:   raw.SessionID,
		SiteID:      raw.SiteID,
		URL:         raw.URL,
		Referrer:    raw.Referrer,
		UTMSource:   utmSource,
		UTMMedium:   utmMedium,
		UTMCampaign: utmCampaign,
		Country:     "unknown",
		DeviceType:  deviceType,
		Browser:     browser,
		OS:          os,
		IPHash:      ipHash,
		Payload:     payload,
		IngestedAt:  time.Now().UTC(),
	}, nil
}

func parseUTM(rawURL string) (source, medium, campaign string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", ""
	}
	values := u.Query()
	return values.Get("utm_source"), values.Get("utm_medium"), values.Get("utm_campaign")
}

func hashIP(salt, ip string) string {
	hasher := sha256.New()
	hasher.Write([]byte(salt))
	hasher.Write([]byte(ip))
	return hex.EncodeToString(hasher.Sum(nil))
}
