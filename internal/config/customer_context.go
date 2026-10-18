package config

import (
	"fmt"
	"net/url"
	"time"
)

const (
	defaultCustomerContextServer  = "orby"
	defaultCustomerContextTimeout = 10
	customerContextPath           = "/api/slack-context"
)

// CustomerContextConfig controls per-conversation scoping by the Orby server
// (beta customers only reach their own organisation's issues).
//
// When Enabled is false (the default) nothing changes: the endpoint is never
// called and no tool call carries _orby_context. When true, before every model
// call the client POSTs the conversation's Slack facts to the endpoint and obeys
// the answer: "blocked" posts the server's message, "customer" restricts the
// model to the allowed tools, "internal" keeps today's behaviour. If the endpoint
// cannot be reached or answers badly, Orby replies that it is unavailable and
// does not call the model. Every tool call is checked against the allowed tools,
// and calls to the Orby MCP server carry the facts as _orby_context.
type CustomerContextConfig struct {
	// Enabled turns the feature on (default false).
	Enabled bool `json:"enabled,omitempty"`
	// MCPServer is the mcpServers entry of the Orby MCP server (default "orby").
	// Only its tool calls carry _orby_context, its httpHeaders authenticate the
	// context endpoint, and bare names in allowed_tools refer to its tools.
	MCPServer string `json:"mcpServer,omitempty"`
	// URL of the context endpoint. Default: the scheme and host of the MCP
	// server's url + /api/slack-context (e.g. http://127.0.0.1:8081/api/slack-context).
	URL string `json:"url,omitempty"`
	// TimeoutSeconds bounds each call to the context endpoint (default 10).
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

func (c *Config) applyCustomerContextDefaults() {
	if c.CustomerContext.MCPServer == "" {
		c.CustomerContext.MCPServer = defaultCustomerContextServer
	}
	if c.CustomerContext.TimeoutSeconds <= 0 {
		c.CustomerContext.TimeoutSeconds = defaultCustomerContextTimeout
	}
}

// Timeout returns the context endpoint timeout.
func (cc CustomerContextConfig) Timeout() time.Duration {
	return time.Duration(cc.TimeoutSeconds) * time.Second
}

// IsContextServer reports whether serverName is the Orby MCP server.
func (cc CustomerContextConfig) IsContextServer(serverName string) bool {
	return serverName == cc.MCPServer
}

// CustomerContextURL returns the context endpoint URL: the configured one, or
// one derived from the Orby MCP server's URL.
func (c *Config) CustomerContextURL() (string, error) {
	if c.CustomerContext.URL != "" {
		return c.CustomerContext.URL, nil
	}
	server, ok := c.MCPServers[c.CustomerContext.MCPServer]
	if !ok {
		return "", fmt.Errorf("customerContext.mcpServer %q is not in mcpServers", c.CustomerContext.MCPServer)
	}
	if server.URL == "" {
		return "", fmt.Errorf("mcpServers.%s has no url to derive the customerContext url from", c.CustomerContext.MCPServer)
	}
	parsed, err := url.Parse(server.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("mcpServers.%s url %q is not an absolute URL", c.CustomerContext.MCPServer, server.URL)
	}
	return parsed.Scheme + "://" + parsed.Host + customerContextPath, nil
}

// validateCustomerContext checks an enabled customerContext can be enforced.
func (c *Config) validateCustomerContext() error {
	if !c.CustomerContext.Enabled {
		return nil
	}
	server, ok := c.MCPServers[c.CustomerContext.MCPServer]
	if !ok || server.Disabled {
		return fmt.Errorf("customerContext.mcpServer %q must name an enabled entry in mcpServers", c.CustomerContext.MCPServer)
	}
	// RAG tools are answered in-process, not through an MCP client, so the
	// per-conversation guard cannot cover them.
	if c.RAG.Enabled {
		return fmt.Errorf("customerContext cannot be enabled together with rag: RAG tools bypass the per-conversation tool guard")
	}
	if _, err := c.CustomerContextURL(); err != nil {
		return err
	}
	return nil
}
