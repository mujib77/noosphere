package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"
)

const (
	ingestURL    = "http://localhost:8080/v1/events"
	brainPort    = ":8090"
	pollInterval = time.Second
	historyLimit = 120
)

type incomingEvent struct {
	ID     string         `json:"id"`
	Source source         `json:"source"`
	Data   map[string]any `json:"data"`
}

type source struct {
	Trust float64 `json:"trust"`
}

type brainState struct {
	UpdatedAt   string             `json:"updatedAt"`
	LastEventID string             `json:"lastEventId"`
	Dimensions  map[string]float64 `json:"dimensions"`
}

type stateSample struct {
	ObservedAt    string  `json:"observedAt"`
	EventID       string  `json:"eventId"`
	GlobalArousal float64 `json:"globalArousal"`
	Coherence     float64 `json:"coherence"`
	Anomaly       float64 `json:"anomaly"`
}

type stateHistory struct {
	Samples []stateSample `json:"samples"`
}

type planetaryBrain struct {
	mu      sync.RWMutex
	state   brainState
	history []stateSample
}

func newPlanetaryBrain() *planetaryBrain {
	return &planetaryBrain{
		state: brainState{
			Dimensions: map[string]float64{
				"thermalEnergy":       0.5,
				"atmosphericMoisture": 0.5,
				"pressureField":       0.5,
				"windMotion":          0.5,
				"globalArousal":       0.5,
				"coherence":           0.5,
				"anomaly":             0.0,
				"sourceTrust":         0.0,
			},
		},
	}
}

func (brain *planetaryBrain) run(ctx context.Context) {
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		brain.ingestPending(client)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (brain *planetaryBrain) ingestPending(client *http.Client) {
	brain.mu.RLock()
	afterEventID := brain.state.LastEventID
	brain.mu.RUnlock()

	events, err := fetchEvents(client, afterEventID)
	if err != nil {
		return
	}

	for _, incoming := range events {
		brain.apply(incoming)
	}
}

func (brain *planetaryBrain) apply(incoming incomingEvent) {
	temperature := number(incoming.Data["temperatureC"], 20)
	humidity := number(incoming.Data["humidityPct"], 50)
	pressure := number(incoming.Data["pressureHpa"], 1013)
	windSpeed := number(incoming.Data["windSpeedKph"], 10)

	targets := map[string]float64{
		"thermalEnergy":       clamp((temperature + 20) / 60),
		"atmosphericMoisture": clamp(humidity / 100),
		"pressureField":       clamp((pressure - 960) / 100),
		"windMotion":          clamp(windSpeed / 60),
		"sourceTrust":         clamp(incoming.Source.Trust),
	}

	targets["anomaly"] = clamp(
		(abs(temperature-20)/30 +
			abs(humidity-55)/100 +
			abs(pressure-1013)/60 +
			windSpeed/100) / 4,
	)

	targets["globalArousal"] = clamp(
		(targets["thermalEnergy"] +
			targets["windMotion"] +
			targets["anomaly"]) / 3,
	)

	targets["coherence"] = clamp(
		(targets["sourceTrust"] + (1 - targets["anomaly"])) / 2,
	)

	brain.mu.Lock()
	defer brain.mu.Unlock()

	for name, target := range targets {
		brain.state.Dimensions[name] = round(
			blend(brain.state.Dimensions[name], target, 0.35),
		)
	}

	brain.state.LastEventID = incoming.ID
	brain.state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	brain.history = append(brain.history, stateSample{
		ObservedAt:    brain.state.UpdatedAt,
		EventID:       incoming.ID,
		GlobalArousal: brain.state.Dimensions["globalArousal"],
		Coherence:     brain.state.Dimensions["coherence"],
		Anomaly:       brain.state.Dimensions["anomaly"],
	})

	if len(brain.history) > historyLimit {
		brain.history = append(
			[]stateSample(nil),
			brain.history[len(brain.history)-historyLimit:]...,
		)
	}

	log.Printf("brain updated from event %s", incoming.ID)
}

func (brain *planetaryBrain) snapshot() brainState {
	brain.mu.RLock()
	defer brain.mu.RUnlock()

	dimensions := make(map[string]float64, len(brain.state.Dimensions))

	for name, value := range brain.state.Dimensions {
		dimensions[name] = value
	}

	return brainState{
		UpdatedAt:   brain.state.UpdatedAt,
		LastEventID: brain.state.LastEventID,
		Dimensions:  dimensions,
	}
}

func (brain *planetaryBrain) historySnapshot() stateHistory {
	brain.mu.RLock()
	defer brain.mu.RUnlock()

	samples := make([]stateSample, len(brain.history))
	copy(samples, brain.history)

	return stateHistory{
		Samples: samples,
	}
}

func fetchEvents(
	client *http.Client,
	afterEventID string,
) ([]incomingEvent, error) {
	endpoint := ingestURL

	if afterEventID != "" {
		endpoint += "?after=" + url.QueryEscape(afterEventID)
	}

	response, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, errors.New("recent events are unavailable")
	}

	var events []incomingEvent

	if err := json.NewDecoder(response.Body).Decode(&events); err != nil {
		return nil, err
	}

	return events, nil
}

func stateHandler(brain *planetaryBrain) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, brain.snapshot())
	}
}

func historyHandler(brain *planetaryBrain) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, brain.historySnapshot())
	}
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json")

	writer.WriteHeader(status)

	_ = json.NewEncoder(writer).Encode(payload)
}

func blend(current, target, weight float64) float64 {
	return current + (target-current)*weight
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

func number(value any, fallback float64) float64 {
	number, ok := value.(float64)
	if !ok {
		return fallback
	}

	return number
}

func round(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func abs(value float64) float64 {
	return math.Abs(value)
}

func main() {
	brain := newPlanetaryBrain()
	appContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	go brain.run(appContext)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/state", stateHandler(brain))
	mux.HandleFunc("GET /v1/state/history", historyHandler(brain))

	server := &http.Server{
		Addr:              brainPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		cancel()

		shutdownContext, shutdownCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer shutdownCancel()

		_ = server.Shutdown(shutdownContext)
	}()

	log.Printf("Gaia brain listening on http://localhost%s/v1/state", brainPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
