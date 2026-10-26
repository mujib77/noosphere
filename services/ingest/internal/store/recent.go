package store

import (
	"sync"
	"time"

	"noosphere/services/ingest/internal/event"
)

type SourceStatus struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"`
	Trust          float64   `json:"trust"`
	LastEventID    string    `json:"lastEventId"`
	LastEventType  string    `json:"lastEventType"`
	LastObservedAt time.Time `json:"lastObservedAt"`
}

type RecentEvents struct {
	mu       sync.RWMutex
	capacity int
	events   []event.Event
}

func NewRecentEvents(capacity int) *RecentEvents {
	if capacity < 1 {
		capacity = 1
	}

	return &RecentEvents{
		capacity: capacity,
	}
}

func (store *RecentEvents) Add(incoming event.Event) {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.events = append(store.events, incoming)

	if len(store.events) > store.capacity {
		store.events = append(
			[]event.Event(nil),
			store.events[len(store.events)-store.capacity:]...,
		)
	}
}

func (store *RecentEvents) Latest() (event.Event, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	if len(store.events) == 0 {
		return event.Event{}, false
	}

	return store.events[len(store.events)-1], true
}

func (store *RecentEvents) Since(eventID string) []event.Event {
	store.mu.RLock()
	defer store.mu.RUnlock()

	start := 0

	if eventID != "" {
		for index, stored := range store.events {
			if stored.ID == eventID {
				start = index + 1
				break
			}
		}
	}

	result := make([]event.Event, len(store.events[start:]))
	copy(result, store.events[start:])

	return result
}

func (store *RecentEvents) Sources() []SourceStatus {
	store.mu.RLock()
	defer store.mu.RUnlock()

	sourcesByID := make(map[string]SourceStatus)
	order := make([]string, 0)

	for _, stored := range store.events {
		sourceID := stored.Source.ID
		if sourceID == "" {
			continue
		}

		if _, exists := sourcesByID[sourceID]; !exists {
			order = append(order, sourceID)
		}

		sourcesByID[sourceID] = SourceStatus{
			ID:             sourceID,
			Kind:           stored.Source.Kind,
			Trust:          stored.Source.Trust,
			LastEventID:    stored.ID,
			LastEventType:  stored.Type,
			LastObservedAt: stored.ObservedAt,
		}
	}

	result := make([]SourceStatus, 0, len(order))

	for _, sourceID := range order {
		result = append(result, sourcesByID[sourceID])
	}

	return result
}
