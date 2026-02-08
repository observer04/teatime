package server

import (
	"log/slog"
	"net/http"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/observer/teatime/docs" // Import generated docs

	"github.com/observer/teatime/internal/api"
	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/config"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/middleware"
	"github.com/observer/teatime/internal/storage"
	"github.com/observer/teatime/internal/websocket"
)

// Dependencies holds all service dependencies for the server
type Dependencies struct {
	DB             *database.DB
	UserRepo       *database.UserRepository
	ConvRepo       *database.ConversationRepository
	CallRepo       *database.CallRepository
	AttachmentRepo *database.AttachmentRepository
	R2Storage      *storage.R2Storage
	AuthService    *auth.Service
	AuthHandler    *api.AuthHandler
	UserHandler    *api.UserHandler
	ConvHandler    *api.ConversationHandler
	CallHandler    *api.CallHandler
	UploadHandler  *api.UploadHandler
	OAuthHandler   *api.OAuthHandlers
	WSHandler      *websocket.Handler
	StaticDir      string
	Logger         *slog.Logger
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
	// Swagger UI - API documentation
	mux.HandleFunc("GET /swagger/", httpSwagger.WrapHandler)

	// Health check - essential for docker, k8s, load balancers
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Ready check - verifies DB connectivity
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := deps.DB.Health(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not ready","error":"database unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	// =========================================================================
	// Auth routes (public) - with rate limiting for brute force protection
	// =========================================================================
	rateLimiter := middleware.NewRateLimiter(60) // 60 requests/min per user
	mux.Handle("POST /auth/register", rateLimiter.Middleware(http.HandlerFunc(deps.AuthHandler.Register)))
	mux.Handle("POST /auth/login", rateLimiter.Middleware(http.HandlerFunc(deps.AuthHandler.Login)))
	mux.HandleFunc("POST /auth/refresh", deps.AuthHandler.Refresh)
	mux.HandleFunc("POST /auth/logout", deps.AuthHandler.Logout)

	// =========================================================================
	// Protected routes (require auth)
	// =========================================================================
	authMiddleware := auth.Middleware(deps.AuthService)

	// =========================================================================
	// OAuth routes (Google Sign-In)
	// =========================================================================
	if deps.OAuthHandler != nil {
		mux.HandleFunc("GET /auth/google", deps.OAuthHandler.HandleGoogleAuth)
		mux.HandleFunc("GET /auth/google/callback", deps.OAuthHandler.HandleGoogleCallback)
		mux.Handle("POST /auth/set-username", authMiddleware(http.HandlerFunc(deps.OAuthHandler.HandleSetUsername)))
	}

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
	mux.Handle("PATCH /conversations/{id}", authMiddleware(http.HandlerFunc(deps.ConvHandler.UpdateConversation)))
	mux.Handle("POST /conversations/{id}/members", authMiddleware(http.HandlerFunc(deps.ConvHandler.AddMember)))
	mux.Handle("DELETE /conversations/{id}/members/{userId}", authMiddleware(http.HandlerFunc(deps.ConvHandler.RemoveMember)))
	mux.Handle("POST /conversations/{id}/archive", authMiddleware(http.HandlerFunc(deps.ConvHandler.ArchiveConversation)))
	mux.Handle("POST /conversations/{id}/unarchive", authMiddleware(http.HandlerFunc(deps.ConvHandler.UnarchiveConversation)))
	mux.Handle("POST /conversations/{id}/read", authMiddleware(http.HandlerFunc(deps.ConvHandler.MarkConversationRead)))
	mux.Handle("POST /conversations/mark-all-read", authMiddleware(http.HandlerFunc(deps.ConvHandler.MarkAllConversationsRead)))

	// =========================================================================
	// Message routes
	// =========================================================================
	mux.Handle("GET /conversations/{id}/messages", authMiddleware(http.HandlerFunc(deps.ConvHandler.GetMessages)))
	mux.Handle("POST /conversations/{id}/messages", authMiddleware(http.HandlerFunc(deps.ConvHandler.SendMessage)))
	mux.Handle("GET /conversations/{id}/messages/search", authMiddleware(http.HandlerFunc(deps.ConvHandler.SearchMessages)))

	// =========================================================================
	// Starred messages routes
	// =========================================================================
	mux.Handle("GET /messages/starred", authMiddleware(http.HandlerFunc(deps.ConvHandler.GetStarredMessages)))
	mux.Handle("GET /messages/search", authMiddleware(http.HandlerFunc(deps.ConvHandler.SearchAllMessages)))
	mux.Handle("POST /messages/{id}/star", authMiddleware(http.HandlerFunc(deps.ConvHandler.StarMessage)))
	mux.Handle("DELETE /messages/{id}/star", authMiddleware(http.HandlerFunc(deps.ConvHandler.UnstarMessage)))
	mux.Handle("DELETE /messages/{id}", authMiddleware(http.HandlerFunc(deps.ConvHandler.DeleteMessage)))

	// =========================================================================
	// Block routes
	// =========================================================================
	mux.Handle("POST /blocks/{username}", authMiddleware(http.HandlerFunc(deps.ConvHandler.BlockUser)))
	mux.Handle("DELETE /blocks/{username}", authMiddleware(http.HandlerFunc(deps.ConvHandler.UnblockUser)))

	// =========================================================================
	// Call routes (call history)
	// =========================================================================
	if deps.CallHandler != nil {
		mux.Handle("GET /calls", authMiddleware(http.HandlerFunc(deps.CallHandler.GetCallHistory)))
		mux.Handle("GET /calls/missed/count", authMiddleware(http.HandlerFunc(deps.CallHandler.GetMissedCallCount)))
		mux.Handle("GET /calls/{id}", authMiddleware(http.HandlerFunc(deps.CallHandler.GetCall)))
		mux.Handle("POST /calls", authMiddleware(http.HandlerFunc(deps.CallHandler.CreateCall)))
		mux.Handle("PATCH /calls/{id}", authMiddleware(http.HandlerFunc(deps.CallHandler.UpdateCall)))
	}

	// =========================================================================
	// Upload routes (file attachments)
	// =========================================================================
	mux.Handle("POST /uploads/init", authMiddleware(http.HandlerFunc(deps.UploadHandler.InitUpload)))
	mux.Handle("POST /uploads/complete", authMiddleware(http.HandlerFunc(deps.UploadHandler.CompleteUpload)))
	mux.Handle("GET /attachments/{id}/url", authMiddleware(http.HandlerFunc(deps.UploadHandler.GetAttachmentURL)))

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
