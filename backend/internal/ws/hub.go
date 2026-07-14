package ws
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
)

// Event is a message broadcast to clients in a group room.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// broadcast bundles a group-scoped event for the hub loop.
type broadcast struct {
	groupID int64
	data    []byte
}

// Hub tracks connected clients grouped by group ID and fans out events.
type Hub struct {
	mu         sync.RWMutex
	rooms      map[int64]map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcast
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[int64]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcast, 64),
	}
}

// Run processes hub events until the context is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.register:
			h.mu.Lock()
			room := h.rooms[c.groupID]
			if room == nil {
				room = make(map[*Client]struct{})
				h.rooms[c.groupID] = room
			}
			room[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if room, ok := h.rooms[c.groupID]; ok {
				if _, ok := room[c]; ok {
					delete(room, c)
					close(c.send)
					if len(room) == 0 {
						delete(h.rooms, c.groupID)
					}
				}
			}
			h.mu.Unlock()
		case b := <-h.broadcast:
			h.mu.RLock()
			for c := range h.rooms[b.groupID] {
				select {
				case c.send <- b.data:
				default:
					// Drop clients that cannot keep up; they will be cleaned up on write failure.
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast marshals and sends an event to all clients in a group room.
func (h *Hub) Broadcast(groupID int64, eventType string, payload any) {
	data, err := json.Marshal(Event{Type: eventType, Payload: payload})
	if err != nil {
		slog.Error("failed to marshal ws event", "error", err)
		return
	}
	select {
	case h.broadcast <- broadcast{groupID: groupID, data: data}:
	default:
		slog.Warn("ws broadcast buffer full, dropping event", "group", groupID, "type", eventType)
	}
}
