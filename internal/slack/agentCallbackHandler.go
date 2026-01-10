package slackbot

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"

	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
)

type sendMessageFunc func(message string)

type agentCallbackHandler struct {
	callbacks.SimpleHandler
	sendMessage    sendMessageFunc
	iterationCount int32 // atomic counter for iterations
}

// truncateForLog truncates a string for logging purposes
func truncateForLogCallback(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// HandleChainStart is called when a chain starts
func (handler *agentCallbackHandler) HandleChainStart(_ context.Context, inputs map[string]any) {
	iteration := atomic.AddInt32(&handler.iterationCount, 1)
	log.Printf("[AGENT-CALLBACK] === CHAIN START === iteration=%d", iteration)
	for key, value := range inputs {
		valueStr := fmt.Sprintf("%v", value)
		log.Printf("[AGENT-CALLBACK] Chain input [%s]: %s", key, truncateForLogCallback(valueStr, 300))
	}
}

func (handler *agentCallbackHandler) HandleChainEnd(_ context.Context, outputs map[string]any) {
	// NOTE: We intentionally do NOT send messages from HandleChainEnd.
	// This callback is invoked for EVERY chain iteration (tool calls, reasoning steps, etc.),
	// not just the final response. The final user-facing response is sent from client.go
	// after GenerateAgentCompletion returns the complete llmResponse.
	
	log.Printf("[AGENT-CALLBACK] === CHAIN END === iteration=%d", atomic.LoadInt32(&handler.iterationCount))
	for key, value := range outputs {
		valueStr := fmt.Sprintf("%v", value)
		log.Printf("[AGENT-CALLBACK] Chain output [%s]: %s", key, truncateForLogCallback(valueStr, 300))
	}
}

// HandleChainError is called when a chain encounters an error
func (handler *agentCallbackHandler) HandleChainError(_ context.Context, err error) {
	log.Printf("[AGENT-CALLBACK] === CHAIN ERROR === iteration=%d error=%v", atomic.LoadInt32(&handler.iterationCount), err)
}

// HandleLLMStart is called when an LLM call starts
func (handler *agentCallbackHandler) HandleLLMStart(_ context.Context, prompts []string) {
	log.Printf("[AGENT-CALLBACK] LLM Start - prompts_count=%d", len(prompts))
	for i, prompt := range prompts {
		log.Printf("[AGENT-CALLBACK] LLM Prompt [%d] (len=%d): %s", i, len(prompt), truncateForLogCallback(prompt, 500))
	}
}

// HandleLLMGenerateContentStart is called when content generation starts
func (handler *agentCallbackHandler) HandleLLMGenerateContentStart(_ context.Context, ms []llms.MessageContent) {
	log.Printf("[AGENT-CALLBACK] LLM GenerateContent Start - messages_count=%d", len(ms))
	for i, msg := range ms {
		log.Printf("[AGENT-CALLBACK] Message [%d] role=%s parts=%d", i, msg.Role, len(msg.Parts))
	}
}

// HandleLLMGenerateContentEnd is called when content generation ends
func (handler *agentCallbackHandler) HandleLLMGenerateContentEnd(_ context.Context, res *llms.ContentResponse) {
	if res == nil {
		log.Printf("[AGENT-CALLBACK] LLM GenerateContent End - response is nil")
		return
	}
	log.Printf("[AGENT-CALLBACK] LLM GenerateContent End - choices_count=%d", len(res.Choices))
	for i, choice := range res.Choices {
		log.Printf("[AGENT-CALLBACK] Choice [%d] content (len=%d): %s", i, len(choice.Content), truncateForLogCallback(choice.Content, 300))
		if choice.FuncCall != nil {
			log.Printf("[AGENT-CALLBACK] Choice [%d] has function call: %s", i, choice.FuncCall.Name)
		}
		if len(choice.ToolCalls) > 0 {
			for j, tc := range choice.ToolCalls {
				log.Printf("[AGENT-CALLBACK] Choice [%d] ToolCall [%d]: %s args=%s", i, j, tc.FunctionCall.Name, truncateForLogCallback(tc.FunctionCall.Arguments, 200))
			}
		}
	}
}

// HandleLLMError is called when an LLM call encounters an error
func (handler *agentCallbackHandler) HandleLLMError(_ context.Context, err error) {
	log.Printf("[AGENT-CALLBACK] === LLM ERROR === error=%v", err)
}

// HandleToolStart is called when a tool is invoked
func (handler *agentCallbackHandler) HandleToolStart(_ context.Context, input string) {
	log.Printf("[AGENT-CALLBACK] Tool Start - input (len=%d): %s", len(input), truncateForLogCallback(input, 500))
}

// HandleToolEnd is called when a tool returns
func (handler *agentCallbackHandler) HandleToolEnd(_ context.Context, output string) {
	log.Printf("[AGENT-CALLBACK] Tool End - output (len=%d): %s", len(output), truncateForLogCallback(output, 500))
}

// HandleToolError is called when a tool encounters an error
func (handler *agentCallbackHandler) HandleToolError(_ context.Context, err error) {
	log.Printf("[AGENT-CALLBACK] === TOOL ERROR === error=%v", err)
}

// HandleAgentAction is called when an agent takes an action
func (handler *agentCallbackHandler) HandleAgentAction(_ context.Context, action schema.AgentAction) {
	log.Printf("[AGENT-CALLBACK] Agent Action - tool=%s input=%s log=%s",
		action.Tool,
		truncateForLogCallback(action.ToolInput, 300),
		truncateForLogCallback(action.Log, 300),
	)
}

// HandleAgentFinish is called when an agent finishes
func (handler *agentCallbackHandler) HandleAgentFinish(_ context.Context, finish schema.AgentFinish) {
	log.Printf("[AGENT-CALLBACK] === AGENT FINISH === return_values=%d log=%s",
		len(finish.ReturnValues),
		truncateForLogCallback(finish.Log, 300),
	)
	for key, value := range finish.ReturnValues {
		valueStr := fmt.Sprintf("%v", value)
		log.Printf("[AGENT-CALLBACK] Agent finish return [%s]: %s", key, truncateForLogCallback(valueStr, 300))
	}
}
