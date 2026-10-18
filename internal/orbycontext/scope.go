package orbycontext

import "context"

// Scope is everything the client knows about one conversation: the Slack facts
// and the server's decision. It travels in the context.Context of the request,
// from the Slack handler down to mcp.Client.CallTool.
type Scope struct {
	Facts    Facts
	Decision Decision
	allowed  map[string]struct{} // nil: every tool is allowed
}

// NewScope builds the scope for a validated decision. A blocked decision allows
// no tool at all, so even a code path that forgot to stop still cannot call one.
func NewScope(facts Facts, decision Decision) *Scope {
	scope := &Scope{Facts: facts, Decision: decision}
	if decision.Mode == ModeBlocked {
		scope.allowed = map[string]struct{}{}
		return scope
	}
	if decision.AllowedTools != nil {
		scope.allowed = make(map[string]struct{}, len(decision.AllowedTools))
		for _, name := range decision.AllowedTools {
			scope.allowed[name] = struct{}{}
		}
	}
	return scope
}

// AllowsTool reports whether a tool may be offered to the model and called.
// toolName is the MCP server's own name for the tool (without the client's
// "<server>_" prefix). A bare name in allowed_tools refers to a tool of the
// Orby server (contextServer true); a tool of any other MCP server must be
// listed as "<server>_<tool>", so a bare "create_bug" cannot open up a tool of
// the same name on another server.
func (s *Scope) AllowsTool(serverName, toolName string, contextServer bool) bool {
	if s.allowed == nil {
		return true
	}
	if _, ok := s.allowed[serverName+"_"+toolName]; ok {
		return true
	}
	if !contextServer {
		return false
	}
	_, ok := s.allowed[toolName]
	return ok
}

// CustomerName returns the customer organisation's name in customer mode, "" otherwise.
func (s *Scope) CustomerName() string {
	if s.Decision.Mode != ModeCustomer || s.Decision.Customer == nil {
		return ""
	}
	return s.Decision.Customer.Name
}

type scopeKey struct{}

// WithScope returns a context that carries the conversation scope.
func WithScope(ctx context.Context, scope *Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

// FromContext returns the conversation scope carried by ctx, if any.
func FromContext(ctx context.Context) (*Scope, bool) {
	scope, ok := ctx.Value(scopeKey{}).(*Scope)
	return scope, ok && scope != nil
}
