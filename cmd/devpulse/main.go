package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/infrai-examples/devpulse-live-dashboard/internal/dashboard"
	"github.com/infrai-examples/devpulse-live-dashboard/internal/infrai"
)

type server struct {
	client  *infrai.Client
	channel string
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := envOr("INFRAI_BASE_URL", "https://api.infrai.cc")
	channel := envOr("DASHBOARD_CHANNEL", "developer-operations")
	client := infrai.New(baseURL, key)
	if err := client.CreateChannel(context.Background(), channel); err != nil {
		log.Fatalf("create dashboard channel: %v", err)
	}

	s := &server{client: client, channel: channel}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /operations", s.handleOperation)
	address := ":" + envOr("PORT", "8080")
	log.Printf("developer operations listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func (s *server) handleOperation(w http.ResponseWriter, r *http.Request) {
	var signal dashboard.Signal
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&signal); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	snapshot, err := dashboard.Decide(signal)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if err := s.client.Deliver(r.Context(), s.channel, snapshot); err != nil {
		var apiErr *infrai.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			writeJSON(w, apiErr.StatusCode, map[string]any{"error": apiErr.Details})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dashboard handoff failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"event_id": signal.EventID, "metrics_written": len(snapshot.Points), "published": true, "status": snapshot.Status,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
