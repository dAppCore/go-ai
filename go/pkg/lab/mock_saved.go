// SPDX-Licence-Identifier: EUPL-1.2

package lab

import (
	"sync"
	"time"

	core "dappco.re/go"
)

// MemorySavedQueryStore is the in-memory SavedQueryStore the
// default-mocked LabService uses until pkg/ml's DuckDB-backed
// store lands. Holds entries keyed by id; iteration order is
// insertion order so the sidebar lists feel stable.
//
// Thread-safe via a single RWMutex — saved-query CRUD is a low-
// traffic surface (tens of entries per user) so the simpler lock
// beats per-key sharding.
type MemorySavedQueryStore struct {
	mu      sync.RWMutex
	entries []SavedQuery
	nextID  int
}

// NewMemorySavedQueryStore returns an empty in-memory store.
//
// Usage example:
//
//	store := lab.NewMemorySavedQueryStore()
//	_ = store.SaveQuery(lab.SavedQuery{Tab: "duckdb", Name: "recent runs", Body: "SELECT * FROM lora_runs"})
func NewMemorySavedQueryStore() *MemorySavedQueryStore {
	return &MemorySavedQueryStore{}
}

// SaveQuery inserts q. If q.ID is empty the store generates one;
// if q.ID matches an existing entry the call overwrites in place
// (so the frontend can "save" an edited query without re-creating).
// CreatedAt + LastUsedAt default to now if not set.
func (s *MemorySavedQueryStore) SaveQuery(q SavedQuery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if q.ID == "" {
		s.nextID++
		q.ID = "saved-" + core.Sprintf("%d", s.nextID)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if q.CreatedAt == "" {
		q.CreatedAt = now
	}
	if q.LastUsedAt == "" {
		q.LastUsedAt = now
	}
	for i, existing := range s.entries {
		if existing.ID == q.ID {
			s.entries[i] = q
			return nil
		}
	}
	s.entries = append(s.entries, q)
	return nil
}

// ListSavedQueries returns the entries tagged with the given tab.
// Empty tab returns every entry (useful for an "all queries"
// admin view).
func (s *MemorySavedQueryStore) ListSavedQueries(tab string) ([]SavedQuery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if tab == "" {
		out := make([]SavedQuery, len(s.entries))
		copy(out, s.entries)
		return out, nil
	}
	out := make([]SavedQuery, 0, len(s.entries))
	for _, q := range s.entries {
		if q.Tab == tab {
			out = append(out, q)
		}
	}
	return out, nil
}

// DeleteSavedQuery removes the entry with the given id. Returns
// nil whether or not id was present (idempotent — re-deletion
// after a stale reload shouldn't error).
func (s *MemorySavedQueryStore) DeleteSavedQuery(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, q := range s.entries {
		if q.ID == id {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			return nil
		}
	}
	return nil
}
