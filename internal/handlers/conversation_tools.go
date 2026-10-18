package handlers

import (
	"context"
	"strings"

	customErrors "github.com/tolemy-bio/slack-mcp-client/internal/common/errors"
	"github.com/tolemy-bio/slack-mcp-client/internal/mcp"
	"github.com/tolemy-bio/slack-mcp-client/internal/orbycontext"
)

// toolsForConversation returns the tools the model may see in this conversation.
// With customerContext disabled that is every discovered tool. With it enabled,
// ctx must carry the conversation scope, and only the tools it allows are kept,
// across all MCP servers. This only shapes what the model is offered; the
// enforcement is in mcp.Client.CallTool, which refuses any other tool anyway.
func (b *LLMMCPBridge) toolsForConversation(ctx context.Context) (map[string]mcp.ToolInfo, error) {
	if !b.cfg.CustomerContext.Enabled {
		return b.availableTools, nil
	}
	scope, ok := orbycontext.FromContext(ctx)
	if !ok {
		return nil, customErrors.NewMCPError("conversation_scope_missing",
			"No Slack conversation context: refusing to offer tools to the model")
	}
	allowed := make(map[string]mcp.ToolInfo, len(b.availableTools))
	for name, tool := range b.availableTools {
		serverToolName := strings.TrimPrefix(name, tool.ServerName+"_")
		if scope.AllowsTool(tool.ServerName, serverToolName, b.cfg.CustomerContext.IsContextServer(tool.ServerName)) {
			allowed[name] = tool
		}
	}
	return allowed, nil
}
