package orbycontext

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const home = "T0TOLEMY"

func TestBuildFacts(t *testing.T) {
	cases := []struct {
		name      string
		user      *UserFacts
		lookupErr error
		want      Facts
	}{
		{
			name: "staff: same team, no guest flags",
			user: &UserFacts{TeamID: home},
			want: Facts{ChannelID: "C1", ChannelType: "channel", UserID: "U1", UserTeamID: home, HomeTeamID: home},
		},
		{
			name: "multi-channel guest",
			user: &UserFacts{TeamID: home, IsRestricted: true},
			want: Facts{ChannelID: "C1", ChannelType: "channel", UserID: "U1", UserTeamID: home, HomeTeamID: home, UserIsRestricted: true},
		},
		{
			name: "single-channel guest",
			user: &UserFacts{TeamID: home, IsRestricted: true, IsUltraRestricted: true},
			want: Facts{ChannelID: "C1", ChannelType: "channel", UserID: "U1", UserTeamID: home, HomeTeamID: home,
				UserIsRestricted: true, UserIsUltraRestricted: true},
		},
		{
			name: "stranger from another workspace",
			user: &UserFacts{TeamID: "T0ANOMER", IsStranger: true},
			want: Facts{ChannelID: "C1", ChannelType: "channel", UserID: "U1", UserTeamID: "T0ANOMER", HomeTeamID: home, UserIsStranger: true},
		},
		{
			name:      "users.info failed: no team, so not staff",
			user:      &UserFacts{TeamID: home},
			lookupErr: errors.New("ratelimited"),
			want:      Facts{ChannelID: "C1", ChannelType: "channel", UserID: "U1", UserTeamID: "", HomeTeamID: home},
		},
		{
			name: "no user returned: no team",
			want: Facts{ChannelID: "C1", ChannelType: "channel", UserID: "U1", UserTeamID: "", HomeTeamID: home},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildFacts("C1", "channel", "U1", home, tc.user, tc.lookupErr)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFactsJSONMatchesContract(t *testing.T) {
	facts := Facts{ChannelID: "C0123", ChannelType: "channel", UserID: "U0123", UserTeamID: "T0123", HomeTeamID: "T0999"}
	raw, err := json.Marshal(facts)
	require.NoError(t, err)
	assert.JSONEq(t, `{"channel_id":"C0123","channel_type":"channel","user_id":"U0123",
		"user_team_id":"T0123","home_team_id":"T0999","user_is_restricted":false,
		"user_is_ultra_restricted":false,"user_is_stranger":false}`, string(raw))

	// The tool argument has exactly the same shape as the endpoint body.
	var asMap map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &asMap))
	assert.Equal(t, asMap, facts.AsArgument())
}

func TestChannelTypeFromID(t *testing.T) {
	assert.Equal(t, "im", ChannelTypeFromID("D0123"))
	assert.Equal(t, "group", ChannelTypeFromID("G0123"))
	assert.Equal(t, "channel", ChannelTypeFromID("C0123"))
}
