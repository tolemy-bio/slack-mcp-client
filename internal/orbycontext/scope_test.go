package orbycontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScope_InternalAllowsEverything(t *testing.T) {
	s := NewScope(Facts{}, Decision{Mode: ModeInternal})
	assert.True(t, s.AllowsTool("orby", "create_bug", true))
	assert.True(t, s.AllowsTool("orby-langfuse", "get_trace", false))
	assert.Equal(t, "", s.CustomerName())
}

func TestScope_CustomerAllowsOnlyListedTools(t *testing.T) {
	s := NewScope(Facts{}, Decision{
		Mode:         ModeCustomer,
		Customer:     &Customer{Key: "anomer", Name: "Anomer"},
		AllowedTools: []string{"create_bug", "orby-langfuse_get_trace"},
	})
	assert.True(t, s.AllowsTool("orby", "create_bug", true))
	assert.True(t, s.AllowsTool("orby-langfuse", "get_trace", false), "qualified name opens another server's tool")
	assert.False(t, s.AllowsTool("orby", "list_linear_projects", true))
	assert.False(t, s.AllowsTool("other", "create_bug", false), "bare name only refers to the Orby server")
	assert.Equal(t, "Anomer", s.CustomerName())
}

func TestScope_EmptyListAllowsNothing(t *testing.T) {
	s := NewScope(Facts{}, Decision{Mode: ModeCustomer, Customer: &Customer{Key: "a", Name: "A"}, AllowedTools: []string{}})
	assert.False(t, s.AllowsTool("orby", "create_bug", true))
}

func TestScope_BlockedAllowsNothing(t *testing.T) {
	s := NewScope(Facts{}, Decision{Mode: ModeBlocked, AllowedTools: []string{"create_bug"}})
	assert.False(t, s.AllowsTool("orby", "create_bug", true))
}

func TestScope_InternalWithListIsRestricted(t *testing.T) {
	s := NewScope(Facts{}, Decision{Mode: ModeInternal, AllowedTools: []string{"create_bug"}})
	assert.True(t, s.AllowsTool("orby", "create_bug", true))
	assert.False(t, s.AllowsTool("orby", "delete_issue", true))
}

func TestScopeContextRoundTrip(t *testing.T) {
	_, ok := FromContext(context.Background())
	assert.False(t, ok)

	s := NewScope(Facts{UserID: "U1"}, Decision{Mode: ModeInternal})
	got, ok := FromContext(WithScope(context.Background(), s))
	assert.True(t, ok)
	assert.Same(t, s, got)

	_, ok = FromContext(WithScope(context.Background(), nil))
	assert.False(t, ok, "a nil scope does not count")
}
