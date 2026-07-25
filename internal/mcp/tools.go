package mcp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/leomaciel/nudge/internal/db"
	"github.com/leomaciel/nudge/internal/telegram"
)

type ToolHandler struct {
	database *db.DB
	notifier *telegram.Notifier
}

func NewToolHandler(database *db.DB, notifier *telegram.Notifier) *ToolHandler {
	return &ToolHandler{
		database: database,
		notifier: notifier,
	}
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallToolResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

func (h *ToolHandler) HandleListTools() interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "create_reminder",
				"description": "Cria um novo lembrete (instantâneo, agendado ou recorrente) para notificação via Telegram. Se a data/hora não for especificada para lembretes agendados, pergunte ao usuário antes de chamar.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "O texto do lembrete.",
						},
						"type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"instant", "scheduled", "recurring"},
							"description": "Tipo de lembrete: 'instant' (envia na hora), 'scheduled' (data/hora fixa), 'recurring' (repetitivo via cron).",
						},
						"scheduled_at": map[string]interface{}{
							"type":        "string",
							"description": "Data e hora no formato ISO-8601 (ex: 2026-07-26T09:00:00Z). Obrigatório para type='scheduled'.",
						},
						"cron_pattern": map[string]interface{}{
							"type":        "string",
							"description": "Padrão de recorrência (ex: '@daily', '0 9 * * 1'). Obrigatório para type='recurring'.",
						},
					},
					"required": []string{"message", "type"},
				},
			},
			{
				"name":        "list_reminders",
				"description": "Lista os lembretes cadastrados no sistema (pendentes, enviados ou cancelados).",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"status": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"pending", "sent", "cancelled", "all"},
							"description": "Filtrar por status. Padrão é 'pending'.",
						},
					},
				},
			},
			{
				"name":        "cancel_reminder",
				"description": "Cancela um lembrete pendente através do seu ID.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID do lembrete a ser cancelado.",
						},
					},
					"required": []string{"id"},
				},
			},
		},
	}
}

func (h *ToolHandler) HandleCallTool(params json.RawMessage) (*CallToolResult, error) {
	var callParams CallToolParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, fmt.Errorf("invalid tool call parameters: %w", err)
	}

	switch callParams.Name {
	case "create_reminder":
		return h.executeCreateReminder(callParams.Arguments)
	case "list_reminders":
		return h.executeListReminders(callParams.Arguments)
	case "cancel_reminder":
		return h.executeCancelReminder(callParams.Arguments)
	default:
		return &CallToolResult{
			IsError: true,
			Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Ferramenta desconhecida: %s", callParams.Name)}},
		}, nil
	}
}

func (h *ToolHandler) executeCreateReminder(args map[string]interface{}) (*CallToolResult, error) {
	msgRaw, _ := args["message"].(string)
	typeRaw, _ := args["type"].(string)
	scheduledAtRaw, _ := args["scheduled_at"].(string)
	cronPatternRaw, _ := args["cron_pattern"].(string)

	if msgRaw == "" {
		return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: "O campo 'message' é obrigatório."}}}, nil
	}

	remType := db.ReminderType(typeRaw)
	reminder := &db.Reminder{
		Message: msgRaw,
		Type:    remType,
	}

	switch remType {
	case db.TypeInstant:
		// Instant reminder will be sent right away by scheduler or directly
		now := time.Now().UTC()
		reminder.ScheduledAt = &now
		reminder.Status = db.StatusPending

	case db.TypeScheduled:
		if scheduledAtRaw == "" {
			return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: "Para lembretes agendados ('scheduled'), informe o parâmetro 'scheduled_at' em ISO-8601."}}}, nil
		}
		t, err := time.Parse(time.RFC3339, scheduledAtRaw)
		if err != nil {
			return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Data/hora inválida '%s'. Use o formato ISO-8601 (ex: 2026-07-26T09:00:00Z).", scheduledAtRaw)}}}, nil
		}
		reminder.ScheduledAt = &t
		reminder.Status = db.StatusPending

	case db.TypeRecurring:
		if cronPatternRaw == "" {
			return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: "Para lembretes recorrentes ('recurring'), informe o parâmetro 'cron_pattern'."}}}, nil
		}
		reminder.CronPattern = &cronPatternRaw
		now := time.Now().UTC()
		reminder.ScheduledAt = &now
		reminder.Status = db.StatusPending

	default:
		return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Tipo inválido '%s'. Use 'instant', 'scheduled' ou 'recurring'.", typeRaw)}}}, nil
	}

	if err := h.database.CreateReminder(reminder); err != nil {
		return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Erro ao salvar lembrete: %v", err)}}}, nil
	}

	// If instant, we can also trigger Telegram immediately for zero latency
	if remType == db.TypeInstant {
		_ = h.notifier.SendNotification(reminder.Message)
		_ = h.database.MarkAsSent(reminder.ID)
	}

	respMsg := fmt.Sprintf("✅ Lembrete criado com sucesso!\nID: %s\nMensagem: %s\nTipo: %s", reminder.ID, reminder.Message, reminder.Type)
	if reminder.ScheduledAt != nil {
		respMsg += fmt.Sprintf("\nData/Hora: %s", reminder.ScheduledAt.Format(time.RFC3339))
	}
	if reminder.CronPattern != nil {
		respMsg += fmt.Sprintf("\nRecorrência: %s", *reminder.CronPattern)
	}

	return &CallToolResult{
		Content: []TextContent{{Type: "text", Text: respMsg}},
	}, nil
}

func (h *ToolHandler) executeListReminders(args map[string]interface{}) (*CallToolResult, error) {
	statusFilter, _ := args["status"].(string)
	if statusFilter == "" {
		statusFilter = "pending"
	}

	reminders, err := h.database.ListReminders(statusFilter)
	if err != nil {
		return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Erro ao buscar lembretes: %v", err)}}}, nil
	}

	if len(reminders) == 0 {
		return &CallToolResult{Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Nenhum lembrete encontrado (status: %s).", statusFilter)}}}, nil
	}

	data, err := json.MarshalIndent(reminders, "", "  ")
	if err != nil {
		return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Erro ao formatar resposta: %v", err)}}}, nil
	}

	return &CallToolResult{
		Content: []TextContent{{Type: "text", Text: string(data)}},
	}, nil
}

func (h *ToolHandler) executeCancelReminder(args map[string]interface{}) (*CallToolResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: "O parâmetro 'id' é obrigatório."}}}, nil
	}

	if err := h.database.CancelReminder(id); err != nil {
		return &CallToolResult{IsError: true, Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Erro ao cancelar lembrete: %v", err)}}}, nil
	}

	return &CallToolResult{
		Content: []TextContent{{Type: "text", Text: fmt.Sprintf("✅ Lembrete '%s' cancelado com sucesso.", id)}},
	}, nil
}
