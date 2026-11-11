// Package hub fans live game events out to subscribers (SSE connections, the CLI).
package hub

import (
	"dnd/internal/domain"
	"sync"
)

// Hub is an in-memory pub/sub keyed by game ID. Slow subscribers drop events
// rather than block the game.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan domain.Event]struct{}
}

// New returns an empty hub.
func New() *Hub {
	return &Hub{subs: map[string]map[chan domain.Event]struct{}{}}
}

// Publish sends e to every subscriber of gameID.
func (h *Hub) Publish(gameID string, e domain.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[gameID] {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns a channel of events and a function to unsubscribe.
func (h *Hub) Subscribe(gameID string) (<-chan domain.Event, func()) {
	ch := make(chan domain.Event, 1024)
	h.mu.Lock()
	if h.subs[gameID] == nil {
		h.subs[gameID] = map[chan domain.Event]struct{}{}
	}
	h.subs[gameID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs[gameID], ch)
		close(ch)
	}
}
