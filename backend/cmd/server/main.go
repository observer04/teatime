package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/observer/teatime/internal/api"
	"github.com/observer/teatime/internal/auth"
	"github.com/observer/teatime/internal/config"
	"github.com/observer/teatime/internal/database"
	"github.com/observer/teatime/internal/pubsub"
	"github.com/observer/teatime/internal/server"
	"github.com/observer/teatime/internal/storage"
	"github.com/observer/teatime/internal/webrtc"
	"github.com/observer/teatime/internal/websocket"
)

func main() {
	// Structured logging from the start
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Create context for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to database
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("connected to database")

	if err := database.EnsureSchema(ctx, db, "migrations"); err != nil {
		slog.Error("failed to ensure database schema", "error", err)
		os.Exit(1)
	}

	// Initialize repositories
	userRepo := database.NewUserRepository(db)
	convRepo := database.NewConversationRepository(db)
	callRepo := database.NewCallRepository(db)
	attachmentRepo := database.NewAttachmentRepository(db.Pool)

	// Initialize token service (use a default key for dev if not set)
	jwtKey := cfg.JWTSigningKey
	if jwtKey == "" {
		if cfg.IsDevelopment() {
			jwtKey = "dev-signing-key-do-not-use-in-production!!" // 44 chars
			slog.Warn("using default JWT signing key - DO NOT USE IN PRODUCTION")
		} else {
			slog.Error("JWT_SIGNING_KEY is required in production")
			os.Exit(1)
		}
	}

	tokenService, err := auth.NewTokenService(jwtKey)
	if err != nil {
		slog.Error("failed to create token service", "error", err)
		os.Exit(1)
	}

	// Initialize auth service
	authService := auth.NewService(userRepo, tokenService)

	// Initialize R2 storage (optional - skip if not configured)
	var r2Storage *storage.R2Storage
	var uploadHandler *api.UploadHandler
	if cfg.R2AccountID != "" && cfg.R2AccessKeyID != "" && cfg.R2SecretAccessKey != "" && cfg.R2Bucket != "" {
		r2Storage, err = storage.NewR2Storage(cfg.R2AccountID, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Bucket)
		if err != nil {
			slog.Error("failed to initialize R2 storage", "error", err)
			os.Exit(1)
		}
		uploadHandler = api.NewUploadHandler(attachmentRepo, convRepo, r2Storage, cfg.MaxUploadBytes, cfg.R2Bucket)
		slog.Info("R2 storage initialized", "bucket", cfg.R2Bucket)
	} else {
		slog.Warn("R2 storage not configured - file uploads disabled")
	}

	// Initialize PubSub (in-memory for single instance, swap for Redis in production)
	ps := pubsub.NewMemoryPubSub()
	defer ps.Close()

	// Initialize broadcaster for API handlers to send WebSocket events
	broadcaster := websocket.NewPubSubBroadcaster(ps)

	// Initialize handlers
	authHandler := api.NewAuthHandler(authService, logger)
	userHandler := api.NewUserHandler(userRepo, logger)
	convHandler := api.NewConversationHandler(convRepo, userRepo, broadcaster, logger)
	apiCallHandler := api.NewCallHandler(callRepo, convRepo, logger)

	// Initialize WebRTC manager
	webrtcConfig := &webrtc.Config{
		STUNURLs:     cfg.ICESTUNURLs,
		TURNURLs:     cfg.ICETURNURLs,
		TURNUsername: cfg.TURNUsername,
		TURNPassword: cfg.TURNPassword,
	}
	webrtcManager := webrtc.NewManager(webrtcConfig, ps, logger)
	callHandler := webrtc.NewCallHandler(webrtcManager, convRepo, callRepo, ps, logger)

	// Initialize SFU for group calls
	sfuConfig := &webrtc.SFUConfig{
		ICEServers: webrtcConfig.GetPionICEServers(),
	}
	sfu := webrtc.NewSFU(sfuConfig, ps, logger)
	sfuHandler := webrtc.NewSFUHandler(sfu, webrtcManager, convRepo, callRepo, ps, logger)

	// Initialize WebSocket hub and handler
	wsHub := websocket.NewHub(authService, convRepo, userRepo, attachmentRepo, ps, logger)
	wsHub.SetCallHandler(callHandler)
	wsHub.SetSFUHandler(sfuHandler)
	go wsHub.Run(context.Background())
	wsHandler := websocket.NewHandler(wsHub, logger)

	// Determine static files directory (relative to working dir in dev, configurable in prod)
	staticDir := "../frontend"
	if cfg.StaticDir != "" {
		staticDir = cfg.StaticDir
	}

	// Create and start server
	deps := &server.Dependencies{
		DB:             db,
		UserRepo:       userRepo,
		ConvRepo:       convRepo,
		CallRepo:       callRepo,
		AttachmentRepo: attachmentRepo,
		R2Storage:      r2Storage,
		AuthService:    authService,
		AuthHandler:    authHandler,
		UserHandler:    userHandler,
		ConvHandler:    convHandler,
		CallHandler:    apiCallHandler,
		UploadHandler:  uploadHandler,
		WSHandler:      wsHandler,
		StaticDir:      staticDir,
		Logger:         logger,
	}

	srv := server.New(cfg, deps)

	// Graceful shutdown setup
	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "addr", cfg.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt
	<-shutdownCtx.Done()
	slog.Info("shutting down gracefully...")

	// Give active connections 10 seconds to finish
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer timeoutCancel()

	if err := srv.Shutdown(timeoutCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}

	slog.Info("server stopped")
}
