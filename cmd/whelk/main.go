package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/manolis/whelk/internal/config"
	"github.com/manolis/whelk/internal/proxy"
)

func main() {
	// Set up structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Log startup configuration
	logger.Info("starting whelk proxy",
		"bind", fmt.Sprintf("%s:%d", cfg.BindHost, cfg.BindPort),
		"timeout", cfg.Timeout.String(),
	)

	// Create proxy handler
	handler := proxy.New(cfg.Timeout, logger)

	// Create HTTP server
	server := &http.Server{
		Addr:              cfg.Address(),
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Block until signal received
	sig := <-sigChan
	logger.Info("received shutdown signal", "signal", sig)

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		cancel()
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
	cancel()

	logger.Info("server shutdown complete")
}
