package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leomaciel/nudge/internal/config"
	"github.com/leomaciel/nudge/internal/db"
	"github.com/leomaciel/nudge/internal/mcp"
	"github.com/leomaciel/nudge/internal/scheduler"
	"github.com/leomaciel/nudge/internal/telegram"
)

func main() {
	log.Println("[Nudge] Initializing application...")

	cfg := config.Load()

	database, err := db.Init(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[Fatal] Database initialization failed: %v", err)
	}
	defer database.Close()
	log.Printf("[Nudge] SQLite database connected (%s)", cfg.DatabaseURL)

	notifier := telegram.NewNotifier(cfg.TelegramBotToken, cfg.TelegramDefaultChatID)

	sched := scheduler.NewScheduler(database, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)

	if cfg.Transport == "stdio" {
		mcpServer := mcp.NewServer(cfg, database, notifier)
		if err := mcpServer.RunStdio(); err != nil {
			log.Fatalf("[Fatal] MCP stdio stopped: %v", err)
		}
		return
	}

	mcpHandler := mcp.NewHandler(cfg, database, notifier)

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mcpHandler,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[Nudge] MCP Server listening on port %s\n", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Fatal] MCP Server stopped with error: %v", err)
		}
	}()

	log.Printf("[Nudge] Application running. Press Ctrl+C to terminate.\n")

	<-stop
	log.Println("[Nudge] Shutting down gracefully...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Nudge] HTTP server forced to shutdown: %v", err)
	}

	log.Println("[Nudge] Shutdown complete.")
}
