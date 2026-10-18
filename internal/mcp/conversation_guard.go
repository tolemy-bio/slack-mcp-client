package mcp

import (
	"context"
	"fmt"

	customErrors "github.com/tolemy-bio/slack-mcp-client/internal/common/errors"
	"github.com/tolemy-bio/slack-mcp-client/internal/orbycontext"
)

// conversationGuard enforces the Orby server's decision on every tool call of a
// client. It lives in Client.CallTool, the only place in this program that sends
// an MCP tools/call request, so no tool-calling path (agent or bridge) can skip it.
type conversationGuard struct {
	// injectFacts marks the Orby MCP server: its calls carry _orby_context and
	// bare names in allowed_tools refer to its tools.
	injectFacts bool
}

// RequireConversationScope turns the guard on for this client. From then on a
// tool call is refused unless its context carries an orbycontext.Scope that
// allows the tool. With injectFacts, the scope's facts are set as _orby_context,
// overwriting any value the model supplied; without it, any _orby_context the
// model supplied is removed. Call it once at startup, before tool calls start.
func (c *Client) RequireConversationScope(injectFacts bool) {
	c.guard = &conversationGuard{injectFacts: injectFacts}
}

// ServerName returns the configured name of the MCP server this client talks to.
func (c *Client) ServerName() string {
	return c.serverName
}

// guardToolCall applies the guard to one call. toolName is the server's own tool
// name (prefix already stripped). It returns the arguments to send: a copy, so
// the caller's map is never changed.
func guardToolCall(ctx context.Context, guard *conversationGuard, serverName, toolName string,
	args map[string]interface{}) (map[string]interface{}, error) {
	if guard == nil {
		return args, nil
	}
	scope, ok := orbycontext.FromContext(ctx)
	if !ok {
		return nil, customErrors.NewMCPError("conversation_scope_missing",
			fmt.Sprintf("Refusing tool '%s': the call carries no Slack conversation context", toolName))
	}
	if !scope.AllowsTool(serverName, toolName, guard.injectFacts) {
		return nil, customErrors.NewMCPError("tool_not_allowed",
			fmt.Sprintf("Tool '%s' is not available in this conversation", toolName))
	}

	guarded := make(map[string]interface{}, len(args)+1)
	for k, v := range args {
		guarded[k] = v
	}
	if guard.injectFacts {
		guarded[orbycontext.ArgumentName] = scope.Facts.AsArgument()
	} else {
		delete(guarded, orbycontext.ArgumentName)
	}
	return guarded, nil
}
