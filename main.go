package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", handlePostEvents)
	mux.HandleFunc("GET /campaigns/{campaignID}/stats", handleGetStats)
	addr := ":8080"
	fmt.Printf("listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// handlePostEvents validates each array element independently, so one bad
// item does not prevent valid events in the same batch from being accepted.
func handlePostEvents(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	var raw []json.RawMessage
	if err := dec.Decode(&raw); err != nil || raw == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must be a JSON array"})
		return
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request must contain exactly one JSON value"})
		return
	}
	response := map[string]any{"received": len(raw), "accepted": 0, "duplicates": 0, "rejected": 0}
	var itemErrors []map[string]any
	for i, item := range raw {
		var ev Event
		if err := json.Unmarshal(item, &ev); err != nil {
			response["rejected"] = response["rejected"].(int) + 1
			itemErrors = append(itemErrors, map[string]any{"index": i, "error": "invalid event JSON"})
			continue
		}
		if err := validateEvent(ev); err != nil {
			response["rejected"] = response["rejected"].(int) + 1
			itemErrors = append(itemErrors, map[string]any{"index": i, "error": err.Error()})
			continue
		}
		if eventStore.insert(ev) {
			response["accepted"] = response["accepted"].(int) + 1
		} else {
			response["duplicates"] = response["duplicates"].(int) + 1
		}
	}
	if len(itemErrors) > 0 {
		response["errors"] = itemErrors
	}
	writeJSON(w, http.StatusOK, response)
}

func validateEvent(ev Event) error {
	if ev.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if ev.CampaignID == "" {
		return fmt.Errorf("campaign_id is required")
	}
	if ev.ContactID == "" {
		return fmt.Errorf("contact_id is required")
	}
	switch ev.Type {
	case "sent", "delivered", "opened", "clicked":
	default:
		return fmt.Errorf("type must be sent, delivered, opened, or clicked")
	}
	if _, err := time.Parse(time.RFC3339, ev.Timestamp); err != nil {
		return fmt.Errorf("timestamp must be a valid RFC3339 timestamp")
	}
	return nil
}

func handleGetStats(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("campaignID")
	if campaignID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "campaign ID is required"})
		return
	}
	writeJSON(w, http.StatusOK, eventStore.stats(campaignID))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
