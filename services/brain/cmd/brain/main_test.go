package main

import "testing"

func TestApplyUpdatesLatentState(t *testing.T) {
	brain := newPlanetaryBrain()

	brain.apply(incomingEvent{
		Type: "environment.weather",
		ID:   "evt_test_weather_001",
		Source: source{
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
