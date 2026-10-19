package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"noosphere/services/ingest/internal/event"
)

type Journal struct {
	mu   sync.Mutex
	file *os.File
}

func Open(path string) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return &Journal{file: file}, nil
}

func (journal *Journal) Append(planetaryEvent event.Event) error {
	payload, err := json.Marshal(planetaryEvent)
	if err != nil {
		return err
	}

	journal.mu.Lock()
	defer journal.mu.Unlock()

	_, err = journal.file.Write(append(payload, '\n'))
	return err
}

func (journal *Journal) Close() error {
	journal.mu.Lock()
	defer journal.mu.Unlock()

	return journal.file.Close()
}

func ReadAll(path string) ([]event.Event, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []event.Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]event.Event, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		var planetaryEvent event.Event
		if err := json.Unmarshal(scanner.Bytes(), &planetaryEvent); err != nil {
			return nil, err
		}

		events = append(events, planetaryEvent)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return events, nil
}
