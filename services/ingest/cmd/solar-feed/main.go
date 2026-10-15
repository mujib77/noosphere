package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"noosphere/services/ingest/internal/event"
)

const noaaKpURL = "https://services.swpc.noaa.gov/json/planetary_k_index_1m.json"

type kpRecord struct {
	TimeTag string  `json:"time_tag"`
	KPIndex float64 `json:"kp_index"`
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
		"how often to check NOAA SWPC",
	)

	flag.Parse()

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	lastTimeTag := ""

	poll := func() {
		record, err := fetchLatestKp(client)
		if err != nil {
			log.Printf("fetch NOAA Kp index: %v", err)
			return
		}

		if record.TimeTag == lastTimeTag {
			return
		}

		observedAt, err := parseTimeTag(record.TimeTag)
		if err != nil {
			log.Printf("parse NOAA time tag: %v", err)
			return
		}

		incoming := event.Event{
			ID:         fmt.Sprintf("evt_noaa_kp_%s", observedAt.Format("20060102T150405Z")),
			Type:       "solar.activity",
			ObservedAt: observedAt,
			ReceivedAt: time.Now().UTC(),
			Source: event.Source{
				ID:    "noaa.swpc.planetary-k-index",
				Kind:  "public-feed",
				Trust: 0.98,
			},
			Location: event.Location{
				Latitude:   0,
				Longitude:  0,
				AccuracyKm: 40075,
			},
			Data: map[string]any{
				"kpIndex":  record.KPIndex,
				"provider": "NOAA SWPC",
				"product":  "planetary_k_index_1m",
			},
			Privacy: event.Privacy{
				Level:                "public",
				ContainsPersonalData: false,
			},
		}

		if err := submitEvent(client, *endpoint, incoming); err != nil {
			log.Printf("submit NOAA event: %v", err)
			return
		}

		lastTimeTag = record.TimeTag

		log.Printf(
			"submitted NOAA planetary Kp %.2f from %s",
			record.KPIndex,
			record.TimeTag,
		)
	}

	poll()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	log.Println("NOAA SWPC solar feed started. Press Ctrl+C to stop.")

	for {
		select {
		case <-ticker.C:
			poll()

		case <-stop:
			log.Println("NOAA SWPC solar feed stopped.")
			return
		}
	}
}

func fetchLatestKp(client *http.Client) (kpRecord, error) {
	response, err := client.Get(noaaKpURL)
	if err != nil {
		return kpRecord{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return kpRecord{}, fmt.Errorf(
			"NOAA returned %s",
			response.Status,
		)
	}

	var records []kpRecord

	if err := json.NewDecoder(response.Body).Decode(&records); err != nil {
		return kpRecord{}, err
	}

	if len(records) == 0 {
		return kpRecord{}, errors.New("NOAA returned no Kp records")
	}

	return records[len(records)-1], nil
}

func parseTimeTag(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}

	parsed, err := time.Parse(
		"2006-01-02T15:04:05",
		value,
	)

	if err != nil {
		return time.Time{}, err
	}

	return parsed.UTC(), nil
}

func submitEvent(
	client *http.Client,
	endpoint string,
	incoming event.Event,
) error {
	payload, err := json.Marshal(incoming)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	request, err := http.NewRequest(
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"ingest service returned %s",
			response.Status,
		)
	}

	return nil
}
