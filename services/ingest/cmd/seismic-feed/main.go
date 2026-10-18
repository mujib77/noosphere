package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"noosphere/services/ingest/internal/event"
)

const usgsFeedURL = "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/all_hour.geojson"

type geoJSONFeed struct {
	Features []earthquakeFeature `json:"features"`
}

type earthquakeFeature struct {
	ID         string               `json:"id"`
	Properties earthquakeProperties `json:"properties"`
	Geometry   earthquakeGeometry   `json:"geometry"`
}

type earthquakeProperties struct {
	Magnitude    *float64 `json:"mag"`
	Place        string   `json:"place"`
	Time         int64    `json:"time"`
	Significance int      `json:"sig"`
	Tsunami      int      `json:"tsunami"`
}

type earthquakeGeometry struct {
	Coordinates []float64 `json:"coordinates"`
}

func main() {
	endpoint := flag.String(
		"endpoint",
		"http://localhost:8080/v1/events",
		"Gaia ingest endpoint",
	)

	interval := flag.Duration(
		"interval",
		time.Minute,
		"how often to check the USGS feed",
	)

	minimumMagnitude := flag.Float64(
		"minimum-magnitude",
		2.5,
		"minimum magnitude Gaia should ingest",
	)

	flag.Parse()

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	published := make(map[string]bool)

	poll := func() {
		features, err := fetchEarthquakes(client)
		if err != nil {
			log.Printf("fetch USGS earthquakes: %v", err)
			return
		}

		submitted := 0

		for _, feature := range features {
			if published[feature.ID] || feature.Properties.Magnitude == nil {
				continue
			}

			if *feature.Properties.Magnitude < *minimumMagnitude {
				continue
			}

			incoming, ok := seismicEvent(feature)
			if !ok {
				continue
			}

			if err := submitEvent(client, *endpoint, incoming); err != nil {
				log.Printf("submit seismic event: %v", err)
				continue
			}

			published[feature.ID] = true
			submitted++

			log.Printf(
				"submitted USGS earthquake M%.1f near %s",
				*feature.Properties.Magnitude,
				feature.Properties.Place,
			)
		}

		if submitted == 0 {
			log.Println("USGS feed checked: no new matching earthquakes")
		}
	}

	poll()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	log.Println("USGS seismic feed started. Press Ctrl+C to stop.")

	for {
		select {
		case <-ticker.C:
			poll()

		case <-stop:
			log.Println("USGS seismic feed stopped.")
			return
		}
	}
}

func fetchEarthquakes(client *http.Client) ([]earthquakeFeature, error) {
	response, err := client.Get(usgsFeedURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("USGS returned %s", response.Status)
	}

	var feed geoJSONFeed

	if err := json.NewDecoder(response.Body).Decode(&feed); err != nil {
		return nil, err
	}

	return feed.Features, nil
}

func seismicEvent(feature earthquakeFeature) (event.Event, bool) {
	if feature.ID == "" ||
		feature.Properties.Magnitude == nil ||
		len(feature.Geometry.Coordinates) < 3 {
		return event.Event{}, false
	}

	observedAt := time.UnixMilli(feature.Properties.Time).UTC()
	longitude := feature.Geometry.Coordinates[0]
	latitude := feature.Geometry.Coordinates[1]
	depthKm := feature.Geometry.Coordinates[2]

	return event.Event{
		ID:         "evt_usgs_" + feature.ID,
		Type:       "geophysical.seismic",
		ObservedAt: observedAt,
		ReceivedAt: time.Now().UTC(),
		Source: event.Source{
			ID:    "usgs.earthquake.geojson",
			Kind:  "public-feed",
			Trust: 0.99,
		},
		Location: event.Location{
			Latitude:   latitude,
			Longitude:  longitude,
			AccuracyKm: 10,
		},
		Data: map[string]any{
			"magnitude":    *feature.Properties.Magnitude,
			"depthKm":      depthKm,
			"place":        feature.Properties.Place,
			"significance": feature.Properties.Significance,
			"tsunami":      feature.Properties.Tsunami == 1,
		},
		Privacy: event.Privacy{
			Level:                "public",
			ContainsPersonalData: false,
		},
	}, true
}

func submitEvent(
	client *http.Client,
	endpoint string,
	incoming event.Event,
) error {
	payload, err := json.Marshal(incoming)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("ingest service returned %s", response.Status)
	}

	return nil
}
