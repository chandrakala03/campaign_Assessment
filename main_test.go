package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestPostEventsAndStats(t *testing.T) {
	eventStore = NewStore()
	body := []byte(`[{"event_id":"e1","campaign_id":"c1","contact_id":"u1","type":"sent","timestamp":"2026-08-10T09:00:00Z"},{"event_id":"e2","campaign_id":"c1","contact_id":"u1","type":"opened","timestamp":"2026-08-10T09:01:00Z"},{"event_id":"e2","campaign_id":"c1","contact_id":"u1","type":"opened","timestamp":"2026-08-10T09:01:00Z"},{"event_id":"bad","campaign_id":"c1","contact_id":"u2","type":"unknown","timestamp":"2026-08-10T09:02:00Z"}]`)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handlePostEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["accepted"] != float64(2) || got["duplicates"] != float64(1) || got["rejected"] != float64(1) {
		t.Fatalf("unexpected response: %v", got)
	}
	stats := eventStore.stats("c1")
	if stats.Sent != 1 || stats.Opened != 1 || stats.UniqueOpens != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestConcurrentDuplicateInsert(t *testing.T) {
	s := NewStore()
	ev := Event{EventID: "same", CampaignID: "c", ContactID: "u", Type: "sent"}
	const n = 30
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() { results <- s.insert(ev) }()
	}
	accepted := 0
	for i := 0; i < n; i++ {
		if <-results {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted %d copies; want exactly 1", accepted)
	}
}

func TestSeedBatchSurvivesMalformedEvents(t *testing.T) {
	eventStore = NewStore()
	seed, err := os.ReadFile("seed/events.json")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(seed))
	rec := httptest.NewRecorder()
	handlePostEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed POST status %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	received := int(got["received"].(float64))
	accepted := int(got["accepted"].(float64))
	duplicates := int(got["duplicates"].(float64))
	rejected := int(got["rejected"].(float64))
	if received != accepted+duplicates+rejected {
		t.Fatalf("outcomes do not sum to received: %v", got)
	}
	if accepted == 0 || rejected == 0 {
		t.Fatalf("expected valid and invalid seed records, got: %v", got)
	}
}
