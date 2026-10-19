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
	"strconv"
	"time"

	"noosphere/services/ingest/internal/event"
)

const openMeteoURL = "https://api.open-meteo.com/v1/forecast"

type weatherResponse struct {
	Latitude  float64        `json:"latitude"`
	Longitude float64        `json:"longitude"`
	Current   weatherCurrent `json:"current"`
}

type weatherCurrent struct {
	Time         string  `json:"time"`
	TemperatureC float64 `json:"temperature_2m"`
	HumidityPct  float64 `json:"relative_humidity_2m"`
	PressureHpa  float64 `json:"pressure_msl"`
	WindSpeedKph float64 `json:"wind_speed_10m"`
}

func main() {
	endpoint := flag.String(
		"endpoint",
		"http://localhost:8080/v1/events",
		"Gaia ingest endpoint",
	)

	latitude := flag.Float64(
		"latitude",
		12.9716,
		"weather location latitude",
	)

	longitude := flag.Float64(
		"longitude",
		77.5946,
		"weather location longitude",
	)

	interval := flag.Duration(
		"interval",
		5*time.Minute,
		"how often to check Open-Meteo",
	)

	flag.Parse()

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	lastObservationTime := ""

	poll := func() {
		reading, err := fetchCurrentWeather(client, *latitude, *longitude)
		if err != nil {
			log.Printf("fetch Open-Meteo weather: %v", err)
			return
		}

		if reading.Current.Time == lastObservationTime {
			return
		}

		observedAt, err := parseObservationTime(reading.Current.Time)
		if err != nil {
			log.Printf("parse Open-Meteo observation time: %v", err)
			return
		}

		incoming := event.Event{
			ID: fmt.Sprintf(
				"evt_open_meteo_%s",
				observedAt.Format("20060102T150405Z"),
			),
			Type:       "environment.weather",
			ObservedAt: observedAt,
			ReceivedAt: time.Now().UTC(),
			Source: event.Source{
				ID:    "open-meteo.current-weather",
				Kind:  "public-feed",
				Trust: 0.9,
			},
			Location: event.Location{
				Latitude:   reading.Latitude,
				Longitude:  reading.Longitude,
				AccuracyKm: 11,
			},
			Data: map[string]any{
				"condition":    "open-meteo-current",
				"temperatureC": reading.Current.TemperatureC,
				"humidityPct":  reading.Current.HumidityPct,
				"pressureHpa":  reading.Current.PressureHpa,
				"windSpeedKph": reading.Current.WindSpeedKph,
				"provider":     "Open-Meteo",
			},
			Privacy: event.Privacy{
				Level:                "public",
				ContainsPersonalData: false,
			},
		}

		if err := submitEvent(client, *endpoint, incoming); err != nil {
			log.Printf("submit Open-Meteo event: %v", err)
			return
		}

		lastObservationTime = reading.Current.Time

		log.Printf(
			"submitted Open-Meteo weather %.1f°C, %.0f%% humidity",
			reading.Current.TemperatureC,
			reading.Current.HumidityPct,
		)
	}

	poll()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	log.Println("Open-Meteo weather feed started. Press Ctrl+C to stop.")

	for {
		select {
		case <-ticker.C:
			poll()

		case <-stop:
			log.Println("Open-Meteo weather feed stopped.")
			return
		}
	}
}

func fetchCurrentWeather(
	client *http.Client,
	latitude float64,
	longitude float64,
) (weatherResponse, error) {
	request, err := http.NewRequest(http.MethodGet, openMeteoURL, nil)
	if err != nil {
		return weatherResponse{}, err
	}

	query := request.URL.Query()
	query.Set("latitude", strconv.FormatFloat(latitude, 'f', 4, 64))
	query.Set("longitude", strconv.FormatFloat(longitude, 'f', 4, 64))
	query.Set(
		"current",
		"temperature_2m,relative_humidity_2m,pressure_msl,wind_speed_10m",
	)
	query.Set("timezone", "UTC")
	request.URL.RawQuery = query.Encode()

	response, err := client.Do(request)
	if err != nil {
		return weatherResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return weatherResponse{}, fmt.Errorf(
			"Open-Meteo returned %s",
			response.Status,
		)
	}

	var reading weatherResponse
	if err := json.NewDecoder(response.Body).Decode(&reading); err != nil {
		return weatherResponse{}, err
	}

	return reading, nil
}

func parseObservationTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}

	parsed, err := time.Parse("2006-01-02T15:04", value)
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
