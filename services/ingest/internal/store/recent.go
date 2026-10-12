package store

import (
	"sync"

	"noosphere/services/ingest/internal/event"
)

type RecentEvents struct {
	mu     sync.RWMutex
	latest event.Event
	hasOne bool
}

func (store *RecentEvents) Add(incoming event.Event) {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.latest = incoming
	store.hasOne = true
}

func (store *RecentEvents) Latest() (event.Event, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	return store.latest, store.hasOne
}
