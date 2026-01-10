package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tolemy-bio/slack-mcp-client/internal/monitoring"
)

type ToolInfo struct {
	ServerName       string
	ToolName         string
	ToolDescription  string
	InputSchema      map[string]interface{}
	InputSchemaBytes []byte
	Client           MCPClientInterface
}

func (t *ToolInfo) Name() string {
	return t.ToolName
}

func (t *ToolInfo) Description() string {
	if t.InputSchemaBytes == nil {
		t.InputSchemaBytes, _ = json.Marshal(t.InputSchema)
	}
	return t.ToolDescription + "\n The input schema is: " + string(t.InputSchemaBytes)
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (t *ToolInfo) Call(ctx context.Context, input string) (string, error) {
	startTime := time.Now()
	
	log.Printf("[TOOL-CALL] === TOOL INVOCATION START === tool=%s server=%s", t.ToolName, t.ServerName)
	log.Printf("[TOOL-CALL] Input (len=%d): %s", len(input), truncateForLog(input, 500))
	
	// Check context before starting
	if ctx.Err() != nil {
		log.Printf("[TOOL-CALL] ERROR: Context already cancelled before tool call: %v", ctx.Err())
		return "", fmt.Errorf("context cancelled before tool call: %w", ctx.Err())
	}
	
	var args map[string]interface{}
	err := json.Unmarshal([]byte(input), &args)
	if err != nil {
		log.Printf("[TOOL-CALL] ERROR: Failed to unmarshal input JSON: %v", err)
		log.Printf("[TOOL-CALL] Raw input that failed to parse: %s", input)
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}
	
	log.Printf("[TOOL-CALL] Parsed args: %+v", args)

	isError := "false"
	defer func() {
		duration := time.Since(startTime)
		monitoring.ToolInvocations.With(prometheus.Labels{
			monitoring.MetricLabelTool:   t.ToolName,
			monitoring.MetricLabelServer: t.ServerName,
			monitoring.MetricLabelError:  isError,
		}).Inc()
		log.Printf("[TOOL-CALL] === TOOL INVOCATION END === tool=%s duration=%v error=%s", t.ToolName, duration, isError)
	}()

	// Check if client is nil
	if t.Client == nil {
		log.Printf("[TOOL-CALL] ERROR: MCP client is nil for tool %s", t.ToolName)
		isError = "true"
		return "", fmt.Errorf("MCP client is nil for tool %s", t.ToolName)
	}

	log.Printf("[TOOL-CALL] Calling MCP client for tool %s...", t.ToolName)
	res, err := t.Client.CallTool(ctx, t.Name(), args)
	if err != nil {
		isError = "true"
		log.Printf("[TOOL-CALL] ERROR: Tool call failed: %v", err)
		
		// Check if it's a context error
		if ctx.Err() != nil {
			log.Printf("[TOOL-CALL] Context error during tool call: %v", ctx.Err())
		}
		
		return "", fmt.Errorf("while calling tool %s: %w", t.Name(), err)
	}

	log.Printf("[TOOL-CALL] SUCCESS: Tool %s returned result (len=%d): %s", t.ToolName, len(res), truncateForLog(res, 500))
	return res, nil
}
