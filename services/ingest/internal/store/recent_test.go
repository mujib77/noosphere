package store

import (
	"testing"
	"time"

	"noosphere/services/ingest/internal/event"
)

func TestSourcesReturnsLatestStatusForEachSource(t *testing.T) {
	events := NewRecentEvents(10)

	events.Add(event.Event{
		ID:         "evt_noaa_001",
		Type:       "solar.activity",
		ObservedAt: time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC),
		Source: event.Source{
			ID:    "noaa.swpc.planetary-k-index",
			Kind:  "public-feed",
			Trust: 0.98,
		},
	})

	events.Add(event.Event{
		ID:         "evt_usgs_001",
		Type:       "geophysical.seismic",
		ObservedAt: time.Date(2026, 10, 26, 8, 1, 0, 0, time.UTC),
		Source: event.Source{
			ID:    "usgs.earthquake.geojson",
			Kind:  "public-feed",
			Trust: 0.99,
		},
	})

	events.Add(event.Event{
		ID:         "evt_noaa_002",
		Type:       "solar.activity",
		ObservedAt: time.Date(2026, 10, 26, 8, 2, 0, 0, time.UTC),
		Source: event.Source{
			ID:    "noaa.swpc.planetary-k-index",
			Kind:  "public-feed",
			Trust: 0.98,
		},
	})

	sources := events.Sources()

	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}

	if sources[0].ID != "noaa.swpc.planetary-k-index" {
		t.Fatalf("unexpected first source %q", sources[0].ID)
	}

	if sources[0].LastEventID != "evt_noaa_002" {
		t.Fatalf("expected latest NOAA event, got %q", sources[0].LastEventID)
	}

	if sources[1].ID != "usgs.earthquake.geojson" {
		t.Fatalf("unexpected second source %q", sources[1].ID)
	}
}
