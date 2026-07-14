package ws

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

// Client represents a single WebSocket connection subscribed to a group room.
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	groupID int64
	userID  int64
	send    chan []byte
}

// Serve registers a new client for the group and pumps messages until the
// connection closes. It blocks until the connection is done.
func (h *Hub) Serve(ctx context.Context, conn *websocket.Conn, groupID, userID int64) {
	c := &Client{
		hub:     h,
		conn:    conn,
		groupID: groupID,
		userID:  userID,
		send:    make(chan []byte, 32),
	}
	h.register <- c

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.readLoop(ctx, cancel)
	c.writeLoop(ctx)

	h.unregister <- c
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

// readLoop drains inbound messages (used only for connection liveness) and
// cancels the context when the peer disconnects.
func (c *Client) readLoop(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	for {
		if _, _, err := c.conn.Read(ctx); err != nil {
			return
		}
	}
}

// writeLoop delivers queued events and periodic pings to the client.
func (c *Client) writeLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.conn.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
