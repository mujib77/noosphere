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

func TestReadAllRestoresJournalEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")

	journal, err := Open(path)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}

	for _, eventID := range []string{"evt_replay_001", "evt_replay_002"} {
		if err := journal.Append(event.Event{ID: eventID}); err != nil {
			t.Fatalf("append event: %v", err)
		}
	}

	if err := journal.Close(); err != nil {
		t.Fatalf("close journal: %v", err)
	}

	events, err := ReadAll(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[1].ID != "evt_replay_002" {
		t.Fatalf("expected second event to be restored, got %q", events[1].ID)
	}
}
