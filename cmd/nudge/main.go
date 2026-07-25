package main

import (
	"context"
	"log"
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

	// Initialize Database
	database, err := db.Init(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[Fatal] Database initialization failed: %v", err)
	}
	defer database.Close()
	log.Println("[Nudge] SQLite database connected and migrated successfully")

	// Initialize Telegram Notifier
	notifier := telegram.NewNotifier(cfg.TelegramBotToken, cfg.TelegramDefaultChatID)

	// Initialize and start Scheduler in background
	sched := scheduler.NewScheduler(database, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)

	// Initialize MCP Server
	mcpServer := mcp.NewServer(cfg, database, notifier)

	// Graceful shutdown listener
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := mcpServer.ListenAndServe(); err != nil {
			log.Fatalf("[Fatal] MCP Server stopped with error: %v", err)
		}
	}()

	log.Printf("[Nudge] Application running. Press Ctrl+C to terminate.\n")

	<-stop
	log.Println("[Nudge] Shutting down gracefully...")

	cancel()
	time.Sleep(500 * time.Millisecond)
	log.Println("[Nudge] Shutdown complete.")
}
