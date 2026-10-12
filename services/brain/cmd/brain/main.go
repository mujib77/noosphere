package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

const (
	ingestURL    = "http://localhost:8080/v1/events/latest"
	brainPort    = ":8090"
	pollInterval = time.Second
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

type planetaryBrain struct {
	mu    sync.RWMutex
	state brainState
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
		brain.ingestLatest(client)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (brain *planetaryBrain) ingestLatest(client *http.Client) {
	incoming, err := fetchLatestEvent(client)
	if err != nil {
		return
	}

	brain.mu.RLock()
	alreadyProcessed := incoming.ID == brain.state.LastEventID
	brain.mu.RUnlock()

	if !alreadyProcessed {
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

func fetchLatestEvent(client *http.Client) (incomingEvent, error) {
	response, err := client.Get(ingestURL)
	if err != nil {
		return incomingEvent{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return incomingEvent{}, errors.New("latest event is unavailable")
	}

	var incoming incomingEvent

	if err := json.NewDecoder(response.Body).Decode(&incoming); err != nil {
		return incomingEvent{}, err
	}

	return incoming, nil
}

func stateHandler(brain *planetaryBrain) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, brain.snapshot())
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
