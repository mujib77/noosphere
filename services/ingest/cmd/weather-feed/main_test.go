package main

import (
	"testing"
	"time"
)

func TestParseObservationTime(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Time
	}{
		{
			name:  "Open-Meteo UTC format",
			value: "2026-10-23T08:30",
			want:  time.Date(2026, 10, 23, 8, 30, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339 format",
			value: "2026-10-23T08:30:00Z",
			want:  time.Date(2026, 10, 23, 8, 30, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseObservationTime(test.value)
			if err != nil {
				t.Fatalf("parse observation time: %v", err)
			}

			if !got.Equal(test.want) {
				t.Fatalf("expected %s, got %s", test.want, got)
			}
		})
	}
}

func TestParseObservationTimeRejectsInvalidValue(t *testing.T) {
	_, err := parseObservationTime("not-a-timestamp")
	if err == nil {
		t.Fatal("expected invalid timestamp to return an error")
	}
}
