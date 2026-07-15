package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/baz/tibia-warden-web/backend/internal/api"
	"github.com/baz/tibia-warden-web/backend/internal/auth"
	"github.com/baz/tibia-warden-web/backend/internal/config"
	"github.com/baz/tibia-warden-web/backend/internal/database"
	"github.com/baz/tibia-warden-web/backend/internal/discord"
	"github.com/baz/tibia-warden-web/backend/internal/store"
	"github.com/baz/tibia-warden-web/backend/internal/ws"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	stores := store.New(pool)

	oauth := auth.NewDiscordProvider(cfg)
	hub := ws.NewHub()
	go hub.Run(ctx)

	bot, err := discord.New(cfg, stores, hub)
	if err != nil {
		slog.Error("failed to initialise discord bot", "error", err)
		os.Exit(1)
	}
	if bot != nil {
		if err := bot.Start(ctx); err != nil {
			slog.Error("failed to start discord bot", "error", err)
			os.Exit(1)
		}
		defer bot.Stop()
		slog.Info("discord bot enabled")
	}

	router := api.NewRouter(cfg, stores, oauth, hub, bot)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("server listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
