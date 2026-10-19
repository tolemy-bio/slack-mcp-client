package slackbot

import (
	"fmt"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/tolemy-bio/slack-mcp-client/internal/orbycontext"
)

// userFactsTTL is short because guest status and membership can change.
const userFactsTTL = 10 * time.Minute

type userFactsEntry struct {
	facts   orbycontext.UserFacts
	expires time.Time
}

// userFactsCache answers "which team is this user in, are they a guest, and
// what are they called?"
// from Slack's users.info (bot scope users:read), caching answers for a short
// time. Failed lookups are not cached, so the next message tries again.
type userFactsCache struct {
	mu      sync.Mutex
	entries map[string]userFactsEntry
	ttl     time.Duration
	now     func() time.Time
	fetch   func(userID string) (*slack.User, error)
}

func newUserFactsCache(fetch func(userID string) (*slack.User, error)) *userFactsCache {
	return &userFactsCache{
		entries: make(map[string]userFactsEntry),
		ttl:     userFactsTTL,
		now:     time.Now,
		fetch:   fetch,
	}
}

func (c *userFactsCache) get(userID string) (*orbycontext.UserFacts, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID must be provided")
	}
	c.mu.Lock()
	entry, ok := c.entries[userID]
	c.mu.Unlock()
	if ok && c.now().Before(entry.expires) {
		facts := entry.facts
		return &facts, nil
	}

	user, err := c.fetch(userID)
	if err != nil {
		return nil, fmt.Errorf("users.info for %s: %w", userID, err)
	}
	if user == nil {
		return nil, fmt.Errorf("users.info for %s returned no user", userID)
	}
	facts := orbycontext.UserFacts{
		TeamID:            user.TeamID,
		IsRestricted:      user.IsRestricted,
		IsUltraRestricted: user.IsUltraRestricted,
		IsStranger:        user.IsStranger,
		Name:              userName(user),
		AvatarURL:         user.Profile.Image72,
	}
	c.mu.Lock()
	c.entries[userID] = userFactsEntry{facts: facts, expires: c.now().Add(c.ttl)}
	c.mu.Unlock()
	return &facts, nil
}

// userName is the name a person reads in Slack: the profile's real name, then
// the display name they chose, then the account handle. users:read is enough
// for all three; email would need another scope, so it is not used.
func userName(user *slack.User) string {
	for _, name := range []string{user.Profile.RealName, user.Profile.DisplayName, user.Name} {
		if name != "" {
			return name
		}
	}
	return ""
}
