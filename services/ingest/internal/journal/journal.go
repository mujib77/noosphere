package journal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"noosphere/services/ingest/internal/event"
)

type Journal struct {
	file *os.File
	mu   sync.Mutex
}

func Open(path string) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(
		path,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return nil, err
	}

	return &Journal{
		file: file,
	}, nil
}

func (journal *Journal) Append(incoming event.Event) error {
	payload, err := json.Marshal(incoming)
	if err != nil {
		return err
	}

	payload = append(payload, '\n')

	journal.mu.Lock()
	defer journal.mu.Unlock()

	_, err = journal.file.Write(payload)
	return err
}

func (journal *Journal) Close() error {
	return journal.file.Close()
}
