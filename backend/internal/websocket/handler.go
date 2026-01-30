package websocket

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins in development (tighten in production)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Handler handles WebSocket upgrade requests
type Handler struct {
	hub    *Hub
	logger *slog.Logger
}

// NewHandler creates a WebSocket handler
func NewHandler(hub *Hub, logger *slog.Logger) *Handler {
	return &Handler{
		hub:    hub,
		logger: logger,
	}
}

// ServeHTTP upgrades HTTP to WebSocket and handles the connection
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := NewClient(h.hub, conn, h.logger)
	h.hub.Register(client)

	// Use a dedicated context for the WebSocket connection lifecycle
	// The request context gets cancelled when ServeHTTP returns after upgrade
	ctx, cancel := context.WithCancel(context.Background())
	client.SetCancelFunc(cancel)

	// Start client goroutines
	go client.WritePump(ctx)
	client.ReadPump(ctx) // Block here until client disconnects
}
