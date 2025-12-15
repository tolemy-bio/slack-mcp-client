package slackbot

import (
	"context"
	"strings"

	"github.com/tmc/langchaingo/callbacks"
)

type sendMessageFunc func(message string)

type agentCallbackHandler struct {
	callbacks.SimpleHandler
	sendMessage sendMessageFunc
}

func (handler *agentCallbackHandler) HandleChainEnd(_ context.Context, outputs map[string]any) {
	if text, ok := outputs["text"]; ok {
		if textStr, ok := text.(string); ok {
			// Filter out raw agent reasoning - only send user-facing messages
			// Agent reasoning contains "Thought:", "Action:", "Action Input:" etc.
			if isAgentReasoning(textStr) {
				return // Don't send agent internal reasoning to Slack
			}
			handler.sendMessage(textStr)
		}
	}
}

// isAgentReasoning checks if the text is agent internal reasoning that shouldn't be shown to users
func isAgentReasoning(text string) bool {
	// Check for common agent reasoning patterns
	reasoningPatterns := []string{
		"Thought:",
		"Action:",
		"Action Input:",
		"Justification:",
		"Observation:",
		"Do I need to use a tool?",
	}

	for _, pattern := range reasoningPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}
