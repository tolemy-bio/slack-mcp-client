package orbycontext

import "fmt"

// Mode is the Orby server's verdict on a conversation.
type Mode string

const (
	// ModeInternal is Tolemy staff: today's behaviour, every tool available.
	ModeInternal Mode = "internal"
	// ModeCustomer is a beta customer: only the allowed tools, only their issues.
	ModeCustomer Mode = "customer"
	// ModeBlocked means Orby must not answer; the server's message is posted instead.
	ModeBlocked Mode = "blocked"
)

// Customer names the organisation a customer-mode conversation belongs to.
type Customer struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Decision is the response of the Orby server's /api/slack-context endpoint.
// AllowedTools nil (JSON null) means every tool; a list, even an empty one,
// restricts the conversation to exactly those tools.
type Decision struct {
	Mode         Mode      `json:"mode"`
	Customer     *Customer `json:"customer"`
	AllowedTools []string  `json:"allowed_tools"`
	Message      *string   `json:"message"`
}

// Validate rejects decisions the client cannot enforce safely. An invalid
// decision is treated like an unreachable server: Orby does not answer.
func (d *Decision) Validate() error {
	switch d.Mode {
	case ModeInternal, ModeBlocked:
		return nil
	case ModeCustomer:
		if d.Customer == nil || d.Customer.Key == "" || d.Customer.Name == "" {
			return fmt.Errorf("customer mode without a customer key and name")
		}
		if d.AllowedTools == nil {
			return fmt.Errorf("customer mode without an allowed_tools list")
		}
		return nil
	default:
		return fmt.Errorf("unknown mode %q", d.Mode)
	}
}

// MessageText returns the server's message, or "" when it sent none.
func (d *Decision) MessageText() string {
	if d.Message == nil {
		return ""
	}
	return *d.Message
}
