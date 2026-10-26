package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"noosphere/services/ingest/internal/event"
	"noosphere/services/ingest/internal/journal"
	"noosphere/services/ingest/internal/store"
)

type server struct {
	events  *store.RecentEvents
	journal *journal.Journal
}

func main() {
	journalPath := "data/events.ndjson"

	savedEvents, err := journal.ReadAll(journalPath)
	if err != nil {
		log.Fatal(err)
	}

	eventJournal, err := journal.Open(journalPath)
	if err != nil {
		log.Fatal(err)
	}
	defer eventJournal.Close()

	app := server{
		events:  store.NewRecentEvents(256),
		journal: eventJournal,
	}

	for _, savedEvent := range savedEvents {
		app.events.Add(savedEvent)
	}

	log.Printf("restored %d persisted planetary events", len(savedEvents))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", app.health)
	mux.HandleFunc("POST /v1/events", app.ingestEvent)
	mux.HandleFunc("GET /v1/events", app.recentEvents)
	mux.HandleFunc("GET /v1/events/latest", app.latestEvent)
	mux.HandleFunc("GET /v1/sources", app.sourceCensus)

	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("Gaia ingest service listening on http://localhost:8080")

	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func (app server) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (app server) ingestEvent(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	var incoming event.Event
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&incoming); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{
			"error": "request body must be a valid Gaia event",
		})
		return
	}

	if incoming.ID == "" || incoming.Type == "" || incoming.ObservedAt.IsZero() ||
		incoming.ReceivedAt.IsZero() || incoming.Source.ID == "" || incoming.Source.Kind == "" {
		writeJSON(writer, http.StatusBadRequest, map[string]string{
			"error": "event is missing required fields",
		})
		return
	}

	if err := app.journal.Append(incoming); err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{
			"error": "could not persist event",
		})
		return
	}

	app.events.Add(incoming)

	writeJSON(writer, http.StatusAccepted, map[string]string{
		"status": "accepted",
		"id":     incoming.ID,
	})
}

func (app server) latestEvent(writer http.ResponseWriter, _ *http.Request) {
	latest, exists := app.events.Latest()

	if !exists {
		writeJSON(writer, http.StatusNotFound, map[string]string{
			"error": "no events have been ingested",
		})
		return
	}

	writeJSON(writer, http.StatusOK, latest)
}

func (app server) recentEvents(writer http.ResponseWriter, request *http.Request) {
	afterEventID := request.URL.Query().Get("after")

	writeJSON(
		writer,
		http.StatusOK,
		app.events.Since(afterEventID),
	)
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func (app server) sourceCensus(
	writer http.ResponseWriter,
	_ *http.Request,
) {
	sources := app.events.Sources()

	writeJSON(writer, http.StatusOK, map[string]any{
		"count":   len(sources),
		"sources": sources,
	})
}
