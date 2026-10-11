package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"time"
)

type Event struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	ObservedAt time.Time      `json:"observedAt"`
	ReceivedAt time.Time      `json:"receivedAt"`
	Source     Source         `json:"source"`
	Location   Location       `json:"location"`
	Data       map[string]any `json:"data"`
	Privacy    Privacy        `json:"privacy"`
}

type Source struct {
	ID    string  `json:"id"`
	Kind  string  `json:"kind"`
	Trust float64 `json:"trust"`
}

type Location struct {
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	AccuracyKm float64 `json:"accuracyKm"`
}

type Privacy struct {
	Level                string `json:"level"`
	ContainsPersonalData bool   `json:"containsPersonalData"`
}

func main() {
	encoder := json.NewEncoder(os.Stdout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	log.Println("Gaia weather simulator started. Press Ctrl+C to stop.")

	for {
		select {
		case now := <-ticker.C:
			event := weatherEvent(now)

			if err := encoder.Encode(event); err != nil {
				log.Printf("encode event: %v", err)
			}

		case <-stop:
			log.Println("Gaia weather simulator stopped.")
			return
		}
	}
}

func weatherEvent(now time.Time) Event {
	wave := math.Sin(float64(now.Unix()) / 60)

	return Event{
		ID:         fmt.Sprintf("evt_weather_bengaluru_%d", now.UnixNano()),
		Type:       "environment.weather",
		ObservedAt: now.UTC(),
		ReceivedAt: time.Now().UTC(),
		Source: Source{
			ID:    "simulator.weather.bengaluru",
			Kind:  "simulator",
			Trust: 0.9,
		},
		Location: Location{
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
		Privacy: Privacy{
			Level:                "public",
			ContainsPersonalData: false,
		},
	}
}

func round(value float64) float64 {
	return math.Round(value*10) / 10
}
