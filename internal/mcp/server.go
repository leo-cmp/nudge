package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/leomaciel/nudge/internal/config"
	"github.com/leomaciel/nudge/internal/db"
	"github.com/leomaciel/nudge/internal/telegram"
)

type Server struct {
	cfg         *config.Config
	toolHandler *ToolHandler
	sessions    map[string]chan []byte
	mu          sync.RWMutex
}

func NewHandler(cfg *config.Config, database *db.DB, notifier *telegram.Notifier) http.Handler {
	s := &Server{
		cfg:         cfg,
		toolHandler: NewToolHandler(database, notifier),
		sessions:    make(map[string]chan []byte),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /sse", s.authMiddleware(s.handleSSE))
	mux.HandleFunc("POST /messages", s.authMiddleware(s.handleMessages))
	return mux
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.MCPAuthToken != "" {
			authHeader := r.Header.Get("Authorization")
			expectedHeader := "Bearer " + s.cfg.MCPAuthToken
			if authHeader != expectedHeader {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sessionID := uuid.New().String()
	messageChan := make(chan []byte, 100)

	s.mu.Lock()
	s.sessions[sessionID] = messageChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		close(messageChan)
		log.Printf("[MCP Server] Session closed: %s\n", sessionID)
	}()

	log.Printf("[MCP Server] New SSE session established: %s\n", sessionID)

	// Send endpoint event with session message URI
	endpointURI := fmt.Sprintf("/messages?sessionId=%s", sessionID)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURI)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageChan:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Missing sessionId query parameter", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	msgChan, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
		return
	}

	// Process JSON-RPC request
	resp := s.processRPCRequest(&req)

	if resp != nil {
		respBytes, err := json.Marshal(resp)
		if err == nil {
			select {
			case msgChan <- respBytes:
			default:
				log.Printf("[MCP Server] Warning: buffer full for session %s\n", sessionID)
			}
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) processRPCRequest(req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "nudge-mcp",
					"version": "1.0.0",
				},
			},
		}

	case "notifications/initialized":
		// No response required for notifications
		return nil

	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  s.toolHandler.HandleListTools(),
		}

	case "tools/call":
		result, err := s.toolHandler.HandleCallTool(req.Params)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &RPCError{
					Code:    -32603,
					Message: err.Error(),
				},
			}
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Method '%s' not found", req.Method),
			},
		}
	}
}
