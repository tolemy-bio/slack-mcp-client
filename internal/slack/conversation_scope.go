package slackbot

import (
	"context"
	"fmt"

	"github.com/tolemy-bio/slack-mcp-client/internal/config"
	"github.com/tolemy-bio/slack-mcp-client/internal/mcp"
	"github.com/tolemy-bio/slack-mcp-client/internal/orbycontext"
)

const (
	// orbyUnavailableMessage is posted when the Orby server cannot say what this
	// conversation may do: Orby then does not answer at all (fail closed).
	orbyUnavailableMessage = "Sorry, Orby is unavailable right now. Please try again in a few minutes."
	// blockedFallbackMessage is posted when the server blocks a conversation without a message.
	blockedFallbackMessage = "Sorry, I can't help in this conversation."
)

// guardMCPClients turns on the per-conversation guard in every MCP client when
// customerContext is enabled. Only the Orby server's client attaches the facts.
func guardMCPClients(cfg *config.Config, mcpClients map[string]*mcp.Client, discoveredTools map[string]mcp.ToolInfo) {
	if !cfg.CustomerContext.Enabled {
		return
	}
	for name, client := range mcpClients {
		client.RequireConversationScope(cfg.CustomerContext.IsContextServer(name))
	}
	// Tools hold their own client reference; guard those too, in case one is
	// not in the map (each client is configured from its own server name).
	for _, tool := range discoveredTools {
		if client, ok := tool.Client.(*mcp.Client); ok && client != nil {
			client.RequireConversationScope(cfg.CustomerContext.IsContextServer(client.ServerName()))
		}
	}
}

// resolveConversationScope asks the Orby server what this conversation may do
// and returns a context carrying the answer. ok is false when Orby must not
// answer; the user has then already been told why. With customerContext
// disabled it returns ctx unchanged and a nil scope (today's behaviour).
func (c *Client) resolveConversationScope(ctx context.Context, channelID, channelType, threadTS, userID string) (context.Context, *orbycontext.Scope, bool) {
	if !c.cfg.CustomerContext.Enabled {
		return ctx, nil, true
	}
	if c.contextResolver == nil {
		c.logger.ErrorKV("customerContext is enabled but no context resolver is configured", "channel", channelID)
		c.userFrontend.SendMessage(channelID, threadTS, orbyUnavailableMessage)
		return ctx, nil, false
	}

	facts := c.identityFacts(channelID, channelType, userID)
	decision, err := c.contextResolver.Resolve(ctx, facts)
	if err != nil {
		c.logger.ErrorKV("Orby context endpoint failed; not answering", "channel", channelID, "user", userID, "error", err)
		c.userFrontend.SendMessage(channelID, threadTS, orbyUnavailableMessage)
		return ctx, nil, false
	}
	c.logger.InfoKV("Conversation scope resolved", "channel", channelID, "user", userID,
		"mode", string(decision.Mode), "user_team_id", facts.UserTeamID, "allowed_tools", len(decision.AllowedTools))

	if decision.Mode == orbycontext.ModeBlocked {
		message := decision.MessageText()
		if message == "" {
			message = blockedFallbackMessage
		}
		c.userFrontend.SendMessage(channelID, threadTS, message)
		return ctx, nil, false
	}
	scope := orbycontext.NewScope(facts, *decision)
	return orbycontext.WithScope(ctx, scope), scope, true
}

// identityFacts builds the facts for one message from Slack. A failed users.info
// lookup leaves the user's team empty, so the server does not treat them as staff.
func (c *Client) identityFacts(channelID, channelType, userID string) orbycontext.Facts {
	user, err := c.userFrontend.GetUserFacts(userID)
	if err != nil {
		c.logger.WarnKV("users.info failed; sending facts without a team (not staff)", "user", userID, "error", err)
	}
	return orbycontext.BuildFacts(channelID, channelType, userID, c.userFrontend.GetHomeTeamID(), user, err)
}

// customerPromptLine tells the model whose team it is helping. It is UX only:
// the boundary is enforced by the tool guard and by the Orby server.
func customerPromptLine(scope *orbycontext.Scope) string {
	if scope == nil {
		return ""
	}
	name := scope.CustomerName()
	if name == "" {
		return ""
	}
	return fmt.Sprintf("\n\nCUSTOMER CONTEXT:\n- You are helping %s's team. You can only see and work on %s's issues; do not discuss other organisations or Tolemy internals.", name, name)
}
