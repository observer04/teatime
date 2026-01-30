package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/observer/teatime/internal/api"
	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/config"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/websocket"
)

// Dependencies holds all service dependencies for the server
type Dependencies struct {
	DB          *database.DB
	UserRepo    *database.UserRepository
	ConvRepo    *database.ConversationRepository
	AuthService *auth.Service
	AuthHandler *api.AuthHandler
	UserHandler *api.UserHandler
	ConvHandler *api.ConversationHandler
	WSHandler   *websocket.Handler
	StaticDir   string
	Logger      *slog.Logger
}

// New creates an HTTP server with all routes configured.
func New(cfg *config.Config, deps *Dependencies) *http.Server {
	mux := http.NewServeMux()

	// Register routes
	registerRoutes(mux, cfg, deps)

	// Wrap with middleware
	handler := chainMiddleware(mux,
		requestIDMiddleware,
		corsMiddleware(cfg),
		loggingMiddleware(deps.Logger),
		recoverMiddleware(deps.Logger),
	)

	return &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func registerRoutes(mux *http.ServeMux, cfg *config.Config, deps *Dependencies) {
	// Health check - essential for docker, k8s, load balancers
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Ready check - verifies DB connectivity
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := deps.DB.Health(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready","error":"database unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// =========================================================================
	// Auth routes (public)
	// =========================================================================
	mux.HandleFunc("POST /auth/register", deps.AuthHandler.Register)
	mux.HandleFunc("POST /auth/login", deps.AuthHandler.Login)
	mux.HandleFunc("POST /auth/refresh", deps.AuthHandler.Refresh)
	mux.HandleFunc("POST /auth/logout", deps.AuthHandler.Logout)

	// =========================================================================
	// Protected routes (require auth)
	// =========================================================================
	authMiddleware := auth.Middleware(deps.AuthService)

	// Me endpoint
	mux.Handle("GET /auth/me", authMiddleware(http.HandlerFunc(deps.AuthHandler.Me)))

	// =========================================================================
	// User routes
	// =========================================================================
	mux.HandleFunc("GET /users/search", deps.UserHandler.Search) // public search
	mux.HandleFunc("GET /users/{username}", deps.UserHandler.GetByUsername)
	mux.Handle("GET /users/me", authMiddleware(http.HandlerFunc(deps.UserHandler.GetMe)))
	mux.Handle("PUT /users/me", authMiddleware(http.HandlerFunc(deps.UserHandler.UpdateProfile)))

	// =========================================================================
	// Conversation routes
	// =========================================================================
	mux.Handle("POST /conversations", authMiddleware(http.HandlerFunc(deps.ConvHandler.CreateConversation)))
	mux.Handle("GET /conversations", authMiddleware(http.HandlerFunc(deps.ConvHandler.ListConversations)))
	mux.Handle("GET /conversations/{id}", authMiddleware(http.HandlerFunc(deps.ConvHandler.GetConversation)))
	mux.Handle("POST /conversations/{id}/members", authMiddleware(http.HandlerFunc(deps.ConvHandler.AddMember)))
	mux.Handle("DELETE /conversations/{id}/members/{userId}", authMiddleware(http.HandlerFunc(deps.ConvHandler.RemoveMember)))

	// =========================================================================
	// Message routes
	// =========================================================================
	mux.Handle("GET /conversations/{id}/messages", authMiddleware(http.HandlerFunc(deps.ConvHandler.GetMessages)))
	mux.Handle("POST /conversations/{id}/messages", authMiddleware(http.HandlerFunc(deps.ConvHandler.SendMessage)))

	// =========================================================================
	// Block routes
	// =========================================================================
	mux.Handle("POST /blocks/{username}", authMiddleware(http.HandlerFunc(deps.ConvHandler.BlockUser)))
	mux.Handle("DELETE /blocks/{username}", authMiddleware(http.HandlerFunc(deps.ConvHandler.UnblockUser)))

	// =========================================================================
	// WebSocket route
	// =========================================================================
	mux.Handle("GET /ws", deps.WSHandler)

	// =========================================================================
	// Static files (frontend) - serve at root
	// =========================================================================
	staticFS := http.FileServer(http.Dir(deps.StaticDir))
	mux.Handle("GET /", staticFS)
}
