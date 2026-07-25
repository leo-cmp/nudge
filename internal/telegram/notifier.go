package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier struct {
	botToken      string
	defaultChatID string
	httpClient    *http.Client
}

func NewNotifier(botToken, defaultChatID string) *Notifier {
	return &Notifier{
		botToken:      botToken,
		defaultChatID: defaultChatID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type sendMessagePayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

func (n *Notifier) SendNotification(message string) error {
	return n.SendToChat(n.defaultChatID, message)
}

func (n *Notifier) SendToChat(chatID, message string) error {
	if n.botToken == "" {
		return fmt.Errorf("telegram bot token is empty")
	}
	if chatID == "" {
		return fmt.Errorf("telegram chat id is empty")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.botToken)
	formattedText := fmt.Sprintf("🔔 *Nudge Lembrete*\n\n%s", message)

	payload := sendMessagePayload{
		ChatID:    chatID,
		Text:      formattedText,
		ParseMode: "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	resp, err := n.httpClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to send request to telegram api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram api returned status code %d", resp.StatusCode)
	}

	return nil
}
