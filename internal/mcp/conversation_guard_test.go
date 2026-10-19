package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tolemy-bio/slack-mcp-client/internal/common/logging"
	"github.com/tolemy-bio/slack-mcp-client/internal/orbycontext"
)

// recordingMCP stands in for the mcp-go client: it records every tools/call
// request that would have been sent to the server. Only CallTool is used.
type recordingMCP struct {
	client.MCPClient
	calls []mcp.CallToolRequest
}

func (r *recordingMCP) CallTool(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	r.calls = append(r.calls, req)
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
}

func newRecordingClient(serverName string) (*Client, *recordingMCP) {
	rec := &recordingMCP{}
	return &Client{
		logger:      logging.New("test", logging.LevelError),
		client:      rec,
		serverName:  serverName,
		initialized: true,
	}, rec
}

func customerScopeCtx(allowed ...string) (context.Context, orbycontext.Facts) {
	facts := orbycontext.Facts{
		ChannelID: "C0ANOMER", ChannelType: "channel", UserID: "U0GUEST",
		UserTeamID: "T0ANOMER", HomeTeamID: "T0TOLEMY", UserIsStranger: true,
		UserName: "Grace Guest", UserAvatarURL: "https://avatars.slack-edge.com/grace_72.png",
	}
	decision := orbycontext.Decision{
		Mode:         orbycontext.ModeCustomer,
		Customer:     &orbycontext.Customer{Key: "anomer", Name: "Anomer"},
		AllowedTools: allowed,
	}
	return orbycontext.WithScope(context.Background(), orbycontext.NewScope(facts, decision)), facts
}

func sentArgs(t *testing.T, rec *recordingMCP) map[string]interface{} {
	t.Helper()
	require.Len(t, rec.calls, 1)
	args, ok := rec.calls[0].Params.Arguments.(map[string]interface{})
	require.True(t, ok)
	return args
}

func TestCallTool_GuardOff_SendsArgsUnchanged(t *testing.T) {
	c, rec := newRecordingClient("orby")
	args := map[string]interface{}{"title": "x", orbycontext.ArgumentName: "model value"}

	_, err := c.CallTool(context.Background(), "orby_create_bug", args)

	require.NoError(t, err)
	sent := sentArgs(t, rec)
	assert.Equal(t, "create_bug", rec.calls[0].Params.Name)
	assert.Equal(t, args, sent, "with customerContext disabled nothing is injected or removed")
}

func TestCallTool_OverwritesModelSuppliedContext(t *testing.T) {
	c, rec := newRecordingClient("orby")
	c.RequireConversationScope(true)
	ctx, facts := customerScopeCtx("create_bug")
	forged := map[string]interface{}{"user_team_id": "T0TOLEMY", "user_is_stranger": false, "user_name": "Somebody Else"}
	args := map[string]interface{}{"title": "x", orbycontext.ArgumentName: forged}

	_, err := c.CallTool(ctx, "orby_create_bug", args)

	require.NoError(t, err)
	sent := sentArgs(t, rec)
	assert.Equal(t, "x", sent["title"])
	assert.Equal(t, facts.AsArgument(), sent[orbycontext.ArgumentName])
	assert.Equal(t, forged, args[orbycontext.ArgumentName], "the caller's map is not mutated")
}

func TestCallTool_RefusesToolNotAllowed(t *testing.T) {
	c, rec := newRecordingClient("orby")
	c.RequireConversationScope(true)
	ctx, _ := customerScopeCtx("create_bug")

	_, err := c.CallTool(ctx, "orby_list_all_issues", map[string]interface{}{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available in this conversation")
	assert.Empty(t, rec.calls, "a refused tool never reaches the server")
}

func TestCallTool_RefusesWithoutScope(t *testing.T) {
	c, rec := newRecordingClient("orby")
	c.RequireConversationScope(true)

	_, err := c.CallTool(context.Background(), "orby_create_bug", map[string]interface{}{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Slack conversation context")
	assert.Empty(t, rec.calls)
}

func TestCallTool_OtherServerGetsNoContext(t *testing.T) {
	c, rec := newRecordingClient("orby-langfuse")
	c.RequireConversationScope(false)
	facts := orbycontext.Facts{UserID: "U0STAFF", UserTeamID: "T0TOLEMY", HomeTeamID: "T0TOLEMY"}
	ctx := orbycontext.WithScope(context.Background(),
		orbycontext.NewScope(facts, orbycontext.Decision{Mode: orbycontext.ModeInternal}))
	args := map[string]interface{}{"q": "x", orbycontext.ArgumentName: "model value"}

	_, err := c.CallTool(ctx, "orby-langfuse_get_trace", args)

	require.NoError(t, err)
	sent := sentArgs(t, rec)
	assert.NotContains(t, sent, orbycontext.ArgumentName)
	assert.Equal(t, "x", sent["q"])
}

func TestCallTool_OtherServerNeedsQualifiedName(t *testing.T) {
	c, rec := newRecordingClient("orby-langfuse")
	c.RequireConversationScope(false)
	ctx, _ := customerScopeCtx("get_trace")

	_, err := c.CallTool(ctx, "orby-langfuse_get_trace", map[string]interface{}{})
	require.Error(t, err, "a bare name only opens Orby's own tool")
	assert.Empty(t, rec.calls)

	ctx, _ = customerScopeCtx("orby-langfuse_get_trace")
	_, err = c.CallTool(ctx, "orby-langfuse_get_trace", map[string]interface{}{})
	require.NoError(t, err)
	assert.Len(t, rec.calls, 1)
}

func TestCallTool_BlockedScopeAllowsNothing(t *testing.T) {
	c, rec := newRecordingClient("orby")
	c.RequireConversationScope(true)
	ctx := orbycontext.WithScope(context.Background(),
		orbycontext.NewScope(orbycontext.Facts{}, orbycontext.Decision{Mode: orbycontext.ModeBlocked}))

	_, err := c.CallTool(ctx, "orby_create_bug", map[string]interface{}{})

	require.Error(t, err)
	assert.Empty(t, rec.calls)
}

// The agent path calls tools through ToolInfo.Call; it must hit the same guard.
func TestToolInfoCall_AgentPathIsGuarded(t *testing.T) {
	c, rec := newRecordingClient("orby")
	c.RequireConversationScope(true)
	ctx, facts := customerScopeCtx("create_bug")
	allowed := &ToolInfo{ServerName: "orby", ToolName: "orby_create_bug", Client: c}
	denied := &ToolInfo{ServerName: "orby", ToolName: "orby_delete_issue", Client: c}

	_, err := allowed.Call(ctx, `{"title":"x","_orby_context":{"user_team_id":"T0TOLEMY"}}`)
	require.NoError(t, err)
	assert.Equal(t, facts.AsArgument(), sentArgs(t, rec)[orbycontext.ArgumentName])

	_, err = denied.Call(ctx, `{}`)
	require.Error(t, err)
	assert.Len(t, rec.calls, 1, "the denied call was not sent")
}
