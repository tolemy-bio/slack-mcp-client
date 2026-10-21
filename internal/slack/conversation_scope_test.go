package slackbot

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tolemy-bio/slack-mcp-client/internal/common/logging"
	"github.com/tolemy-bio/slack-mcp-client/internal/config"
	"github.com/tolemy-bio/slack-mcp-client/internal/orbycontext"
)

// fakeFrontend implements UserFrontend for the parts the scope step uses.
type fakeFrontend struct {
	UserFrontend
	users    map[string]*orbycontext.UserFacts
	usersErr error
	sent     []string
}

func (f *fakeFrontend) GetUserFacts(userID string) (*orbycontext.UserFacts, error) {
	if f.usersErr != nil {
		return nil, f.usersErr
	}
	return f.users[userID], nil
}

func (f *fakeFrontend) GetHomeTeamID() string { return "T0TOLEMY" }

func (f *fakeFrontend) SendMessage(_, _, text string) { f.sent = append(f.sent, text) }

type fakeResolver struct {
	decision *orbycontext.Decision
	err      error
	calls    []orbycontext.Facts
}

func (r *fakeResolver) Resolve(_ context.Context, facts orbycontext.Facts) (*orbycontext.Decision, error) {
	r.calls = append(r.calls, facts)
	return r.decision, r.err
}

func newScopeTestClient(enabled bool, fe *fakeFrontend, res *fakeResolver) *Client {
	cfg := &config.Config{}
	cfg.ApplyDefaults()
	cfg.CustomerContext.Enabled = enabled
	c := &Client{logger: logging.New("test", logging.LevelError), userFrontend: fe, cfg: cfg}
	if res != nil {
		c.contextResolver = res
	}
	return c
}

func guestFrontend() *fakeFrontend {
	return &fakeFrontend{users: map[string]*orbycontext.UserFacts{
		"U0GUEST": {TeamID: "T0ANOMER", IsStranger: true, Name: "Grace Guest", AvatarURL: "https://avatars.slack-edge.com/grace_72.png"},
		"U0STAFF": {TeamID: "T0TOLEMY"},
	}}
}

func TestResolveScope_DisabledNeverCallsEndpoint(t *testing.T) {
	fe, res := guestFrontend(), &fakeResolver{}
	c := newScopeTestClient(false, fe, res)

	ctx, scope, ok := c.resolveConversationScope(context.Background(), "C1", "channel", "1.0", "U0GUEST")

	assert.True(t, ok)
	assert.Nil(t, scope)
	assert.Empty(t, res.calls)
	_, has := orbycontext.FromContext(ctx)
	assert.False(t, has, "no scope, so nothing is injected downstream")
	assert.Empty(t, fe.sent)
}

func TestResolveScope_Customer(t *testing.T) {
	fe := guestFrontend()
	res := &fakeResolver{decision: &orbycontext.Decision{
		Mode:         orbycontext.ModeCustomer,
		Customer:     &orbycontext.Customer{Key: "anomer", Name: "Anomer"},
		AllowedTools: []string{"create_bug"},
	}}
	c := newScopeTestClient(true, fe, res)

	ctx, scope, ok := c.resolveConversationScope(context.Background(), "C0ANOMER", "channel", "1.0", "U0GUEST")

	require.True(t, ok)
	require.NotNil(t, scope)
	assert.Empty(t, fe.sent)
	assert.Equal(t, orbycontext.Facts{
		ChannelID: "C0ANOMER", ChannelType: "channel", UserID: "U0GUEST",
		UserTeamID: "T0ANOMER", HomeTeamID: "T0TOLEMY", UserIsStranger: true,
		UserName: "Grace Guest", UserAvatarURL: "https://avatars.slack-edge.com/grace_72.png",
	}, res.calls[0])
	fromCtx, has := orbycontext.FromContext(ctx)
	require.True(t, has)
	assert.Same(t, scope, fromCtx)
	assert.True(t, scope.AllowsTool("orby", "create_bug", true))
	assert.False(t, scope.AllowsTool("orby", "list_all_issues", true))
	assert.Contains(t, customerPromptLine(scope), "helping Anomer's team")
}

func TestResolveScope_Internal(t *testing.T) {
	fe := guestFrontend()
	res := &fakeResolver{decision: &orbycontext.Decision{Mode: orbycontext.ModeInternal}}
	c := newScopeTestClient(true, fe, res)

	_, scope, ok := c.resolveConversationScope(context.Background(), "C1", "channel", "1.0", "U0STAFF")

	require.True(t, ok)
	assert.Equal(t, "T0TOLEMY", res.calls[0].UserTeamID)
	assert.True(t, scope.AllowsTool("orby", "anything", true))
	assert.Equal(t, "", customerPromptLine(scope), "staff get no customer line")
}

func TestResolveScope_BlockedPostsServerMessage(t *testing.T) {
	fe := guestFrontend()
	msg := "Please ask in #anomer-orbit-beta."
	res := &fakeResolver{decision: &orbycontext.Decision{Mode: orbycontext.ModeBlocked, Message: &msg}}
	c := newScopeTestClient(true, fe, res)

	_, _, ok := c.resolveConversationScope(context.Background(), "C1", "channel", "1.0", "U0GUEST")

	assert.False(t, ok)
	assert.Equal(t, []string{msg}, fe.sent)
}

func TestResolveScope_BlockedWithoutMessage(t *testing.T) {
	fe := guestFrontend()
	res := &fakeResolver{decision: &orbycontext.Decision{Mode: orbycontext.ModeBlocked}}
	c := newScopeTestClient(true, fe, res)

	_, _, ok := c.resolveConversationScope(context.Background(), "C1", "channel", "1.0", "U0GUEST")

	assert.False(t, ok)
	assert.Equal(t, []string{blockedFallbackMessage}, fe.sent)
}

func TestResolveScope_EndpointErrorFailsClosed(t *testing.T) {
	fe := guestFrontend()
	res := &fakeResolver{err: errors.New("HTTP 503")}
	c := newScopeTestClient(true, fe, res)

	_, scope, ok := c.resolveConversationScope(context.Background(), "C1", "channel", "1.0", "U0STAFF")

	assert.False(t, ok)
	assert.Nil(t, scope)
	assert.Equal(t, []string{orbyUnavailableMessage}, fe.sent)
}

func TestResolveScope_MissingResolverFailsClosed(t *testing.T) {
	fe := guestFrontend()
	c := newScopeTestClient(true, fe, nil)

	_, _, ok := c.resolveConversationScope(context.Background(), "C1", "channel", "1.0", "U0STAFF")

	assert.False(t, ok)
	assert.Equal(t, []string{orbyUnavailableMessage}, fe.sent)
}

func TestResolveScope_UsersInfoFailureSendsNoTeam(t *testing.T) {
	fe := guestFrontend()
	fe.usersErr = errors.New("users.info: ratelimited")
	res := &fakeResolver{decision: &orbycontext.Decision{Mode: orbycontext.ModeBlocked}}
	c := newScopeTestClient(true, fe, res)

	c.resolveConversationScope(context.Background(), "C1", "channel", "1.0", "U0STAFF")

	require.Len(t, res.calls, 1)
	assert.Equal(t, "", res.calls[0].UserTeamID, "never guess staff")
	assert.Equal(t, "T0TOLEMY", res.calls[0].HomeTeamID)
	assert.Equal(t, "", res.calls[0].UserName, "no name guessed")
	assert.Equal(t, "", res.calls[0].UserAvatarURL)
}

func TestNewClient_EnabledWithoutResolverFails(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()
	cfg.CustomerContext.Enabled = true
	_, err := NewClient(guestFrontend(), logging.New("test", logging.LevelError), nil, nil, cfg, nil)

	assert.Error(t, err)
}
