package main

import "testing"

func TestApplyUpdatesLatentState(t *testing.T) {
	brain := newPlanetaryBrain()

	brain.apply(incomingEvent{
		Type: "environment.weather",
		ID:   "evt_test_weather_001",
		Source: source{
			ID:    "simulator.weather.bengaluru",
			Trust: 0.9,
		},
		Data: map[string]any{
			"temperatureC": 26.0,
			"humidityPct":  74.0,
			"pressureHpa":  1010.0,
			"windSpeedKph": 18.0,
		},
	})

	state := brain.snapshot()

	if state.LastEventID != "evt_test_weather_001" {
		t.Fatalf("expected event ID to be recorded, got %q", state.LastEventID)
	}

	if state.LastEventType != "environment.weather" {
		t.Fatalf(
			"expected weather event type, got %q",
			state.LastEventType,
		)
	}

	if state.LastSourceID != "simulator.weather.bengaluru" {
		t.Fatalf(
			"expected weather source ID, got %q",
			state.LastSourceID,
		)
	}

	if state.UpdatedAt == "" {
		t.Fatal("expected updated timestamp to be recorded")
	}

	if state.Dimensions["thermalEnergy"] <= 0.5 {
		t.Errorf(
			"expected thermal energy to increase, got %f",
			state.Dimensions["thermalEnergy"],
		)
	}

	for name, value := range state.Dimensions {
		if value < 0 || value > 1 {
			t.Errorf(
				"expected %s to stay within 0–1, got %f",
				name,
				value,
			)
		}
	}
}

func TestApplyRecordsStateHistory(t *testing.T) {
	brain := newPlanetaryBrain()

	brain.apply(incomingEvent{
		Type: "environment.weather",

		ID: "evt_test_history_001",
		Source: source{
			Trust: 0.8,
		},
		Data: map[string]any{},
	})

	history := brain.historySnapshot()

	if len(history.Samples) != 1 {
		t.Fatalf("expected one history sample, got %d", len(history.Samples))
	}

	if history.Samples[0].EventID != "evt_test_history_001" {
		t.Fatalf(
			"expected sample event ID to be recorded, got %q",
			history.Samples[0].EventID,
		)
	}
}

func TestApplyRespondsToSolarActivity(t *testing.T) {
	brain := newPlanetaryBrain()

	brain.apply(incomingEvent{
		ID:   "evt_test_solar_001",
		Type: "solar.activity",
		Source: source{
			Trust: 0.95,
		},
		Data: map[string]any{
			"kpIndex":      7.0,
			"solarWindKph": 720.0,
			"xrayFlux":     0.8,
		},
	})

	state := brain.snapshot()

	if state.Dimensions["solarExcitation"] <= 0 {
		t.Errorf(
			"expected solar excitation to increase above zero, got %f",
			state.Dimensions["solarExcitation"],
		)
	}
}

func TestApplyRespondsToLinuxAgentSystemEvent(t *testing.T) {
	brain := newPlanetaryBrain()

	brain.apply(incomingEvent{
		ID:   "evt_test_agent_001",
		Type: "agent.system",
		Source: source{
			ID:    "agent.demo-node",
			Trust: 0.85,
		},
		Data: map[string]any{
			"load1":    3.0,
			"cpuCount": 4.0,
		},
	})

	state := brain.snapshot()

	if state.Dimensions["digitalActivity"] <= 0 {
		t.Errorf(
			"expected digital activity to increase, got %f",
			state.Dimensions["digitalActivity"],
		)
	}

	if state.Dimensions["machineVitality"] >= 0.5 {
		t.Errorf(
			"expected machine vitality to decrease under load, got %f",
			state.Dimensions["machineVitality"],
		)
	}
}

func TestApplyRespondsToSeismicEvent(t *testing.T) {
	brain := newPlanetaryBrain()

	brain.apply(incomingEvent{
		ID:   "evt_test_seismic_001",
		Type: "geophysical.seismic",
		Source: source{
			ID:    "usgs.earthquake-feed",
			Trust: 0.98,
		},
		Data: map[string]any{
			"magnitude": 6.5,
			"depthKm":   12.0,
		},
	})

	state := brain.snapshot()

	if state.Dimensions["seismicTension"] <= 0 {
		t.Errorf(
			"expected seismic tension to increase, got %f",
			state.Dimensions["seismicTension"],
		)
	}
}

func TestDreamSnapshotProjectsCurrentLatentState(t *testing.T) {
	brain := newPlanetaryBrain()

	brain.apply(incomingEvent{
		ID:   "evt_test_dream_solar_001",
		Type: "solar.activity",
		Source: source{
			ID:    "test.noaa",
			Trust: 0.98,
		},
		Data: map[string]any{
			"kpIndex":      7.0,
			"solarWindKph": 650.0,
			"xrayFlux":     0.6,
		},
	})

	current := brain.snapshot()
	dream := brain.dreamSnapshot()

	if dream.HorizonMinutes != dreamHorizonMinutes {
		t.Fatalf(
			"expected %d minute horizon, got %d",
			dreamHorizonMinutes,
			dream.HorizonMinutes,
		)
	}

	if dream.Model != "latent-extrapolator-v1" {
		t.Fatalf("unexpected dream model %q", dream.Model)
	}

	if dream.BasisEventID != current.LastEventID {
		t.Fatalf(
			"expected basis event %q, got %q",
			current.LastEventID,
			dream.BasisEventID,
		)
	}

	if dream.Dimensions["solarExcitation"] >= current.Dimensions["solarExcitation"] {
		t.Fatal("expected dream solar excitation to decay slightly")
	}
}
