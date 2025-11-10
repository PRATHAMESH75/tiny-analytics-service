package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tiny-analytics/internal/model"
	"tiny-analytics/internal/pipeline"
)

func TestEnrichAddsMetadata(t *testing.T) {
	raw := model.RawEvent{
		Event: model.Event{
			SiteID:    "site-1",
			EventName: "pageview",
			URL:       "https://example.com/?utm_source=ads&utm_medium=cpc&utm_campaign=launch",
			Referrer:  "https://google.com",
			UserID:    "user-1",
			SessionID: "session-1",
			TS:        time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC).UnixMilli(),
			Props:     map[string]any{"foo": "bar"},
		},
		IP: "1.2.3.4",
		UA: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/90.0",
	}

	enriched, err := pipeline.Enrich(raw, "salt")
	require.NoError(t, err)
	require.Equal(t, "ads", enriched.UTMSource)
	require.Equal(t, "cpc", enriched.UTMMedium)
	require.Equal(t, "launch", enriched.UTMCampaign)
	require.Equal(t, "chrome", enriched.Browser)
	require.Equal(t, "desktop", enriched.DeviceType)
	require.Equal(t, "windows", enriched.OS)
	require.Len(t, enriched.IPHash, 64)
	require.Equal(t, raw.Referrer, enriched.Referrer)
	require.Equal(t, raw.SiteID, enriched.SiteID)
}
