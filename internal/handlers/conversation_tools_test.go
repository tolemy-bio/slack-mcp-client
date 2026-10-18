package handlers

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"

	"github.com/tolemy-bio/slack-mcp-client/internal/common/logging"
	"github.com/tolemy-bio/slack-mcp-client/internal/config"
	"github.com/tolemy-bio/slack-mcp-client/internal/mcp"
	"github.com/tolemy-bio/slack-mcp-client/internal/orbycontext"
)

// scopeRecordingClient records the conversation scope each tool call carried.
type scopeRecordingClient struct {
	scopes []*orbycontext.Scope
}

func (c *scopeRecordingClient) CallTool(ctx context.Context, _ string, _ map[string]interface{}) (string, error) {
	scope, _ := orbycontext.FromContext(ctx)
	c.scopes = append(c.scopes, scope)
	return "done", nil
}

func testBridge(enabled bool, client mcp.MCPClientInterface) *LLMMCPBridge {
	cfg := &config.Config{}
	cfg.ApplyDefaults()
	cfg.CustomerContext.Enabled = enabled
	tools := map[string]mcp.ToolInfo{
		"orby_create_bug":         {ServerName: "orby", ToolName: "orby_create_bug"},
		"orby_list_all_issues":    {ServerName: "orby", ToolName: "orby_list_all_issues"},
		"orby-langfuse_get_trace": {ServerName: "orby-langfuse", ToolName: "orby-langfuse_get_trace"},
	}
	clients := map[string]mcp.MCPClientInterface{"orby": client, "orby-langfuse": client}
	return NewLLMMCPBridgeWithLogLevel(clients, nil, tools, logging.LevelError, nil, cfg)
}

func scopeCtx(decision orbycontext.Decision) context.Context {
	return orbycontext.WithScope(context.Background(), orbycontext.NewScope(orbycontext.Facts{UserID: "U1"}, decision))
}

func names(tools map[string]mcp.ToolInfo) []string {
	out := make([]string, 0, len(tools))
	for name := range tools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func TestToolsForConversation_DisabledOffersEverything(t *testing.T) {
	b := testBridge(false, &scopeRecordingClient{})
	got, err := b.toolsForConversation(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestToolsForConversation_CustomerSeesOnlyAllowedToolsAcrossServers(t *testing.T) {
	b := testBridge(true, &scopeRecordingClient{})
	ctx := scopeCtx(orbycontext.Decision{
		Mode:         orbycontext.ModeCustomer,
		Customer:     &orbycontext.Customer{Key: "anomer", Name: "Anomer"},
		AllowedTools: []string{"create_bug"},
	})
	got, err := b.toolsForConversation(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"orby_create_bug"}, names(got))
}

func TestToolsForConversation_InternalSeesEverything(t *testing.T) {
	b := testBridge(true, &scopeRecordingClient{})
	got, err := b.toolsForConversation(scopeCtx(orbycontext.Decision{Mode: orbycontext.ModeInternal}))
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestToolsForConversation_EnabledWithoutScopeFails(t *testing.T) {
	b := testBridge(true, &scopeRecordingClient{})
	_, err := b.toolsForConversation(context.Background())
	assert.Error(t, err)
}

func TestGenerateToolPrompt_ListsOnlyGivenTools(t *testing.T) {
	b := testBridge(true, &scopeRecordingClient{})
	got, err := b.toolsForConversation(scopeCtx(orbycontext.Decision{
		Mode: orbycontext.ModeCustomer, Customer: &orbycontext.Customer{Key: "a", Name: "A"},
		AllowedTools: []string{"create_bug"},
	}))
	require.NoError(t, err)
	prompt := b.generateToolPrompt(got)
	assert.Contains(t, prompt, "orby_create_bug")
	assert.NotContains(t, prompt, "orby_list_all_issues")
	assert.NotContains(t, prompt, "orby-langfuse_get_trace")
}

// The non-agent path (ProcessLLMResponse -> executeToolCall) must hand the
// request's context, and so the scope, to the client's CallTool.
func TestProcessLLMResponse_PassesScopeToClient(t *testing.T) {
	client := &scopeRecordingClient{}
	b := testBridge(true, client)
	ctx := scopeCtx(orbycontext.Decision{Mode: orbycontext.ModeInternal})
	response := &llms.ContentChoice{FuncCall: &llms.FunctionCall{Name: "orby_create_bug", Arguments: `{"title":"x"}`}}

	_, err := b.ProcessLLMResponse(ctx, response, "", map[string]interface{}{"channel_id": "C1"})

	require.NoError(t, err)
	require.Len(t, client.scopes, 1)
	assert.NotNil(t, client.scopes[0])
	assert.Equal(t, "U1", client.scopes[0].Facts.UserID)
}
