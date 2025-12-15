package slackbot

import (
	"context"

	"github.com/tmc/langchaingo/callbacks"
)

type sendMessageFunc func(message string)

type agentCallbackHandler struct {
	callbacks.SimpleHandler
	sendMessage sendMessageFunc
}

func (handler *agentCallbackHandler) HandleChainEnd(_ context.Context, outputs map[string]any) {
	// NOTE: We intentionally do NOT send messages from HandleChainEnd.
	// This callback is invoked for EVERY chain iteration (tool calls, reasoning steps, etc.),
	// not just the final response. The final user-facing response is sent from client.go
	// after GenerateAgentCompletion returns the complete llmResponse.
	//
	// Previously, sending from here caused:
	// 1. Agent reasoning (Thought:, Action:, etc.) being sent to Slack
	// 2. Duplicate final messages (once from here, once from client.go)
	_ = outputs // Intentionally unused - we don't send intermediate outputs
}
