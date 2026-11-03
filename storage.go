package main

import "sync"

type Store struct {
	mu     sync.RWMutex
	events map[string]Event
}

func NewStore() *Store { return &Store{events: make(map[string]Event)} }

var eventStore = NewStore()

// insert atomically checks and stores an event ID, making concurrent retries safe.
func (s *Store) insert(ev Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.events[ev.EventID]; exists {
		return false
	}
	s.events[ev.EventID] = ev
	return true
}

func (s *Store) stats(campaignID string) CampaignStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := CampaignStats{CampaignID: campaignID}
	contacts := make(map[string]struct{})
	for _, ev := range s.events {
		if ev.CampaignID != campaignID {
			continue
		}
		switch ev.Type {
		case "sent":
			result.Sent++
		case "delivered":
			result.Delivered++
		case "opened":
			result.Opened++
			contacts[ev.ContactID] = struct{}{}
		case "clicked":
			result.Clicked++
		}
	}
	result.UniqueOpens = len(contacts)
	return result
}
