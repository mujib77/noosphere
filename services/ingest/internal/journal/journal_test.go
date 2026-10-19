package journal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"noosphere/services/ingest/internal/event"
)

func TestAppendWritesNDJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")

	journal, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()

	incoming := event.Event{
		ID:   "evt_test_001",
		Type: "environment.weather",
	}

	if err := journal.Append(incoming); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var stored event.Event

	if err := json.Unmarshal(
		[]byte(strings.TrimSpace(string(contents))),
		&stored,
	); err != nil {
		t.Fatal(err)
	}

	if stored.ID != incoming.ID {
		t.Fatalf("expected %q, got %q", incoming.ID, stored.ID)
	}
}
