package slackbot

import (
	"errors"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tolemy-bio/slack-mcp-client/internal/orbycontext"
)

func TestUserFactsCache_MapsUsersInfo(t *testing.T) {
	cache := newUserFactsCache(func(string) (*slack.User, error) {
		return &slack.User{ID: "U1", TeamID: "T0ANOMER", IsRestricted: true, IsUltraRestricted: true, IsStranger: true}, nil
	})

	got, err := cache.get("U1")

	require.NoError(t, err)
	assert.Equal(t, orbycontext.UserFacts{TeamID: "T0ANOMER", IsRestricted: true, IsUltraRestricted: true, IsStranger: true}, *got)
}

func TestUserFactsCache_CachesForTTL(t *testing.T) {
	calls := 0
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	cache := newUserFactsCache(func(string) (*slack.User, error) {
		calls++
		return &slack.User{TeamID: "T0TOLEMY", IsRestricted: calls > 1}, nil
	})
	cache.now = func() time.Time { return now }

	first, _ := cache.get("U1")
	now = now.Add(9 * time.Minute)
	second, _ := cache.get("U1")
	assert.Equal(t, 1, calls)
	assert.Equal(t, first, second)

	now = now.Add(2 * time.Minute) // past the 10 minute TTL: guest status is re-read
	third, _ := cache.get("U1")
	assert.Equal(t, 2, calls)
	assert.True(t, third.IsRestricted)
}

func TestUserFactsCache_DoesNotCacheFailures(t *testing.T) {
	calls := 0
	cache := newUserFactsCache(func(string) (*slack.User, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("ratelimited")
		}
		return &slack.User{TeamID: "T0TOLEMY"}, nil
	})

	_, err := cache.get("U1")
	require.Error(t, err)
	got, err := cache.get("U1")
	require.NoError(t, err)
	assert.Equal(t, "T0TOLEMY", got.TeamID)
}

func TestUserFactsCache_RejectsEmptyUser(t *testing.T) {
	cache := newUserFactsCache(func(string) (*slack.User, error) { return nil, nil })
	_, err := cache.get("")
	assert.Error(t, err)
	_, err = cache.get("U1")
	assert.Error(t, err, "a nil user is a failure, not an empty team")
}
