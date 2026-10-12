package event

import "time"

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
