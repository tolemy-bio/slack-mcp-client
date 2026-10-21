// Package orbycontext carries who is talking to Orby, and where, from the Slack
// event to every MCP tool call, together with the Orby server's decision about
// that conversation (internal staff, a beta customer, or blocked).
//
// The facts come from Slack, never from the model, and the server decides what
// they allow. The client only enforces that decision: it hides tools the server
// did not allow, refuses calls to them, and attaches the facts to every call to
// the Orby MCP server so the server can enforce its own rules.
package orbycontext

import "strings"

// ArgumentName is the tool argument that carries the identity facts to the Orby
// MCP server. It is set after the model's arguments, so the model cannot forge it.
const ArgumentName = "_orby_context"

// Facts identifies one Slack message: the conversation it was posted in and the
// person who posted it. The JSON shape is the contract with the Orby server.
// user_name and user_avatar_url (TOL-830) let the server record who reported a
// ticket from Slack rather than from a name the model wrote; they are always
// sent, empty when users.info failed, and the server treats them as optional.
// Neither decides access: that stays with the team and guest flags.
type Facts struct {
	ChannelID             string `json:"channel_id"`
	ChannelType           string `json:"channel_type"`
	UserID                string `json:"user_id"`
	UserTeamID            string `json:"user_team_id"`
	HomeTeamID            string `json:"home_team_id"`
	UserIsRestricted      bool   `json:"user_is_restricted"`
	UserIsUltraRestricted bool   `json:"user_is_ultra_restricted"`
	UserIsStranger        bool   `json:"user_is_stranger"`
	UserName              string `json:"user_name"`
	UserAvatarURL         string `json:"user_avatar_url"`
}

// UserFacts is what Slack's users.info says about a user's membership, and
// who they are.
type UserFacts struct {
	TeamID            string
	IsRestricted      bool // multi-channel guest
	IsUltraRestricted bool // single-channel guest
	IsStranger        bool // user from another workspace (Slack Connect)
	Name              string
	AvatarURL         string
}

// BuildFacts assembles the facts for one message. When the user lookup failed,
// the user's team is left empty: the server's staff rule (same team as the
// bot's workspace) then fails closed instead of the client guessing "staff",
// and the name and avatar stay empty rather than being guessed.
func BuildFacts(channelID, channelType, userID, homeTeamID string, user *UserFacts, lookupErr error) Facts {
	facts := Facts{
		ChannelID:   channelID,
		ChannelType: channelType,
		UserID:      userID,
		HomeTeamID:  homeTeamID,
	}
	if lookupErr != nil || user == nil {
		return facts
	}
	facts.UserTeamID = user.TeamID
	facts.UserIsRestricted = user.IsRestricted
	facts.UserIsUltraRestricted = user.IsUltraRestricted
	facts.UserIsStranger = user.IsStranger
	facts.UserName = user.Name
	facts.UserAvatarURL = user.AvatarURL
	return facts
}

// ChannelTypeFromID derives the conversation type from a Slack channel ID, for
// events that do not carry channel_type (app_mention). D… is a DM, G… a private
// group or multi-person DM, anything else a channel. Newer private channels also
// start with C, so "channel" does not imply public; the server keys customer
// channels by channel_id, not by type.
func ChannelTypeFromID(channelID string) string {
	switch {
	case strings.HasPrefix(channelID, "D"):
		return "im"
	case strings.HasPrefix(channelID, "G"):
		return "group"
	default:
		return "channel"
	}
}

// AsArgument returns the facts as a tool argument value (a JSON object).
func (f Facts) AsArgument() map[string]interface{} {
	return map[string]interface{}{
		"channel_id":               f.ChannelID,
		"channel_type":             f.ChannelType,
		"user_id":                  f.UserID,
		"user_team_id":             f.UserTeamID,
		"home_team_id":             f.HomeTeamID,
		"user_is_restricted":       f.UserIsRestricted,
		"user_is_ultra_restricted": f.UserIsUltraRestricted,
		"user_is_stranger":         f.UserIsStranger,
		"user_name":                f.UserName,
		"user_avatar_url":          f.UserAvatarURL,
	}
}
