package config

import (
	"os"
	"strings"
)

type Config struct {
	Transport             string
	Port                  string
	MCPAuthToken          string
	TelegramBotToken      string
	TelegramDefaultChatID string
	DatabaseURL           string
}

func Load() *Config {
	transport := getEnv("NUDGE_TRANSPORT", "sse")
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--stdio":
			transport = "stdio"
		case "--sse":
			transport = "sse"
		}
	}

	return &Config{
		Transport:             transport,
		Port:                  getEnv("PORT", "8080"),
		MCPAuthToken:          getEnv("MCP_AUTH_TOKEN", ""),
		TelegramBotToken:      getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramDefaultChatID: getEnv("TELEGRAM_DEFAULT_CHAT_ID", ""),
		DatabaseURL:           getEnv("DATABASE_URL", "nudge.db"),
	}
}

func getEnv(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}
