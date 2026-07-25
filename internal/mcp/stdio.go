package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func (s *Server) RunStdio() error {
	log.SetOutput(os.Stderr)
	log.Println("[Nudge] Starting MCP server in stdio mode")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("[Nudge] Failed to parse request: %v", err)
			continue
		}

		resp := s.processRPCRequest(&req)
		if resp != nil {
			data, err := json.Marshal(resp)
			if err != nil {
				log.Printf("[Nudge] Failed to marshal response: %v", err)
				continue
			}
			fmt.Fprintln(os.Stdout, string(data))
			if f, ok := os.Stdout.(interface{ Sync() error }); ok {
				_ = f.Sync()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdio scanner error: %w", err)
	}
	return nil
}
