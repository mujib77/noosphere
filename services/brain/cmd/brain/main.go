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
	Type   string         `json:"type"`
	Source source         `json:"source"`
	Data   map[string]any `json:"data"`
}

type source struct {
	ID    string  `json:"id"`
	Trust float64 `json:"trust"`
}

type brainState struct {
	UpdatedAt     string             `json:"updatedAt"`
	LastEventID   string             `json:"lastEventId"`
	LastEventType string             `json:"lastEventType"`
	LastSourceID  string             `json:"lastSourceId"`
	Dimensions    map[string]float64 `json:"dimensions"`
}

type stateSample struct {
	ObservedAt      string  `json:"observedAt"`
	EventID         string  `json:"eventId"`
	GlobalArousal   float64 `json:"globalArousal"`
	Coherence       float64 `json:"coherence"`
	Anomaly         float64 `json:"anomaly"`
	SolarExcitation float64 `json:"solarExcitation"`
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
				"solarExcitation":     0.0,
				"machineVitality":     0.5,
				"digitalActivity":     0.0,
				"seismicTension":      0.0,
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
	targets := targetsForEvent(incoming)

	if len(targets) == 0 {
		log.Printf("ignored unsupported event type %q", incoming.Type)
		return
	}

	brain.mu.Lock()
	defer brain.mu.Unlock()

	dimensions := brain.state.Dimensions

	for name, target := range targets {
		dimensions[name] = round(blend(dimensions[name], target, 0.35))
	}

	arousalTarget := clamp(
		(dimensions["thermalEnergy"] +
			dimensions["windMotion"] +
			dimensions["solarExcitation"] +
			dimensions["digitalActivity"] +
			dimensions["seismicTension"] +
			dimensions["anomaly"]) / 6,
	)

	dimensions["globalArousal"] = round(
		blend(dimensions["globalArousal"], arousalTarget, 0.35),
	)

	coherenceTarget := clamp(
		(dimensions["sourceTrust"] + (1 - dimensions["anomaly"])) / 2,
	)

	dimensions["coherence"] = round(
		blend(dimensions["coherence"], coherenceTarget, 0.35),
	)

	brain.state.LastEventID = incoming.ID
	brain.state.LastEventType = incoming.Type
	brain.state.LastSourceID = incoming.Source.ID
	brain.state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	brain.history = append(brain.history, stateSample{
		ObservedAt:      brain.state.UpdatedAt,
		EventID:         incoming.ID,
		GlobalArousal:   brain.state.Dimensions["globalArousal"],
		Coherence:       brain.state.Dimensions["coherence"],
		Anomaly:         brain.state.Dimensions["anomaly"],
		SolarExcitation: brain.state.Dimensions["solarExcitation"],
	})

	if len(brain.history) > historyLimit {
		brain.history = append(
			[]stateSample(nil),
			brain.history[len(brain.history)-historyLimit:]...,
		)
	}

	log.Printf("brain updated from %s event %s", incoming.Type, incoming.ID)
}

func targetsForEvent(incoming incomingEvent) map[string]float64 {
	switch incoming.Type {
	case "environment.weather":
		return weatherTargets(incoming)

	case "solar.activity":
		return solarTargets(incoming)

	case "agent.system":
		return agentSystemTargets(incoming)

	case "geophysical.seismic":
		return seismicTargets(incoming)

	default:
		return nil
	}
}

func weatherTargets(incoming incomingEvent) map[string]float64 {
	temperature := number(incoming.Data["temperatureC"], 20)
	humidity := number(incoming.Data["humidityPct"], 50)
	pressure := number(incoming.Data["pressureHpa"], 1013)
	windSpeed := number(incoming.Data["windSpeedKph"], 10)

	anomaly := clamp(
		(abs(temperature-20)/30 +
			abs(humidity-55)/100 +
			abs(pressure-1013)/60 +
			windSpeed/100) / 4,
	)

	return map[string]float64{
		"thermalEnergy":       clamp((temperature + 20) / 60),
		"atmosphericMoisture": clamp(humidity / 100),
		"pressureField":       clamp((pressure - 960) / 100),
		"windMotion":          clamp(windSpeed / 60),
		"anomaly":             anomaly,
		"sourceTrust":         clamp(incoming.Source.Trust),
	}
}

func solarTargets(incoming incomingEvent) map[string]float64 {
	kpIndex := number(incoming.Data["kpIndex"], 0)
	solarWind := number(incoming.Data["solarWindKph"], 400)
	xrayFlux := number(incoming.Data["xrayFlux"], 0)

	solarExcitation := clamp(
		(kpIndex/9)*0.6 +
			((solarWind-250)/650)*0.25 +
			xrayFlux*0.15,
	)

	return map[string]float64{
		"solarExcitation": solarExcitation,
		"anomaly":         clamp(solarExcitation * 0.8),
		"sourceTrust":     clamp(incoming.Source.Trust),
	}
}

func agentSystemTargets(incoming incomingEvent) map[string]float64 {
	load1 := number(incoming.Data["load1"], 0)
	cpuCount := number(incoming.Data["cpuCount"], 1)

	if cpuCount < 1 {
		cpuCount = 1
	}

	loadRatio := clamp(load1 / cpuCount)

	return map[string]float64{
		"machineVitality": 1 - loadRatio,
		"digitalActivity": loadRatio,
		"sourceTrust":     clamp(incoming.Source.Trust),
	}
}

func seismicTargets(incoming incomingEvent) map[string]float64 {
	magnitude := number(incoming.Data["magnitude"], 0)
	depthKm := number(incoming.Data["depthKm"], 700)

	depthInfluence := 1 - clamp(depthKm/700)

	seismicTension := clamp(
		(magnitude/8)*0.8 +
			depthInfluence*0.2,
	)

	return map[string]float64{
		"seismicTension": seismicTension,
		"anomaly":        clamp(seismicTension * 0.9),
		"sourceTrust":    clamp(incoming.Source.Trust),
	}
}

func (brain *planetaryBrain) snapshot() brainState {
	brain.mu.RLock()
	defer brain.mu.RUnlock()

	dimensions := make(map[string]float64, len(brain.state.Dimensions))

	for name, value := range brain.state.Dimensions {
		dimensions[name] = value
	}

	return brainState{
		UpdatedAt:     brain.state.UpdatedAt,
		LastEventID:   brain.state.LastEventID,
		Dimensions:    dimensions,
		LastEventType: brain.state.LastEventType,
		LastSourceID:  brain.state.LastSourceID,
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
