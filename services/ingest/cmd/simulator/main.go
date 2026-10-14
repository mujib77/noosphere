package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"time"

	"noosphere/services/ingest/internal/event"
)

func main() {
	mode := flag.String(
		"mode",
		"weather",
		"simulator mode: weather or solar",
	)

	endpoint := flag.String(
		"endpoint",
		"",
		"optional Gaia ingest endpoint, for example http://localhost:8080/v1/events",
	)

	flag.Parse()

	if *mode != "weather" && *mode != "solar" {
		log.Fatal("mode must be either weather or solar")
	}

	encoder := json.NewEncoder(os.Stdout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	log.Printf(
		"Gaia %s simulator started. Press Ctrl+C to stop.",
		*mode,
	)

	for {
		select {
		case now := <-ticker.C:
			var outgoing event.Event

			switch *mode {
			case "weather":
				outgoing = weatherEvent(now)

			case "solar":
				outgoing = solarEvent(now)
			}

			if err := encoder.Encode(outgoing); err != nil {
				log.Printf("encode event: %v", err)
			}

			if *endpoint != "" {
				if err := submitEvent(client, *endpoint, outgoing); err != nil {
					log.Printf("submit event: %v", err)
					continue
				}

				log.Printf(
					"submitted %s event %s",
					outgoing.Type,
					outgoing.ID,
				)
			}

		case <-stop:
			log.Printf("Gaia %s simulator stopped.", *mode)
			return
		}
	}
}

func submitEvent(client *http.Client, endpoint string, weather event.Event) error {
	payload, err := json.Marshal(weather)
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

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("ingest service returned %s", response.Status)
	}

	return nil
}

func weatherEvent(now time.Time) event.Event {
	wave := math.Sin(float64(now.Unix()) / 60)

	return event.Event{
		ID:         fmt.Sprintf("evt_weather_bengaluru_%d", now.UnixNano()),
		Type:       "environment.weather",
		ObservedAt: now.UTC(),
		ReceivedAt: time.Now().UTC(),
		Source: event.Source{
			ID:    "simulator.weather.bengaluru",
			Kind:  "simulator",
			Trust: 0.9,
		},
		Location: event.Location{
			Latitude:   12.9716,
			Longitude:  77.5946,
			AccuracyKm: 5,
		},
		Data: map[string]any{
			"condition":    "partly-cloudy",
			"temperatureC": round(24.8 + wave*1.5),
			"humidityPct":  round(76 - wave*8),
			"pressureHpa":  round(1012.4 + wave*1.8),
			"windSpeedKph": round(11.6 + wave*4),
		},
		Privacy: event.Privacy{
			Level:                "public",
			ContainsPersonalData: false,
		},
	}
}

func solarEvent(now time.Time) event.Event {
	slowWave := math.Sin(float64(now.Unix()) / 18)
	fastWave := math.Sin(float64(now.Unix()) / 6)

	kpIndex := round(4.5 + slowWave*2.8 + fastWave*0.7)
	solarWind := round(490 + slowWave*150 + fastWave*45)
	xrayFlux := round(0.45 + slowWave*0.2 + fastWave*0.08)

	return event.Event{
		ID:         fmt.Sprintf("evt_solar_global_%d", now.UnixNano()),
		Type:       "solar.activity",
		ObservedAt: now.UTC(),
		ReceivedAt: time.Now().UTC(),
		Source: event.Source{
			ID:    "simulator.solar.global",
			Kind:  "simulator",
			Trust: 0.85,
		},
		Location: event.Location{
			Latitude:   0,
			Longitude:  0,
			AccuracyKm: 40075,
		},
		Data: map[string]any{
			"kpIndex":      kpIndex,
			"solarWindKph": solarWind,
			"xrayFlux":     xrayFlux,
		},
		Privacy: event.Privacy{
			Level:                "public",
			ContainsPersonalData: false,
		},
	}
}

func round(value float64) float64 {
	return math.Round(value*10) / 10
}
