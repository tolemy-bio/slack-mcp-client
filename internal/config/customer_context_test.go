package config

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func customerContextConfig() *Config {
	c := &Config{}
	c.ApplyDefaults()
	c.MCPServers["orby"] = MCPServerConfig{URL: "http://127.0.0.1:8081/mcp/orby", Transport: "http"}
	c.CustomerContext.Enabled = true
	return c
}

func TestCustomerContextDefaults(t *testing.T) {
	c := &Config{}
	c.ApplyDefaults()
	if c.CustomerContext.Enabled {
		t.Fatal("customerContext must be off by default")
	}
	if c.CustomerContext.MCPServer != "orby" {
		t.Errorf("default mcpServer = %q, want orby", c.CustomerContext.MCPServer)
	}
	if c.CustomerContext.Timeout() != 10*time.Second {
		t.Errorf("default timeout = %v, want 10s", c.CustomerContext.Timeout())
	}
	if err := c.validateCustomerContext(); err != nil {
		t.Errorf("disabled customerContext must always validate, got %v", err)
	}
}

func TestCustomerContextURL(t *testing.T) {
	c := customerContextConfig()
	got, err := c.CustomerContextURL()
	if err != nil || got != "http://127.0.0.1:8081/api/slack-context" {
		t.Errorf("derived url = %q, %v", got, err)
	}

	c.CustomerContext.URL = "http://orby.internal/api/slack-context"
	got, err = c.CustomerContextURL()
	if err != nil || got != "http://orby.internal/api/slack-context" {
		t.Errorf("explicit url = %q, %v", got, err)
	}
}

func TestValidateCustomerContext(t *testing.T) {
	if err := customerContextConfig().validateCustomerContext(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}

	missing := customerContextConfig()
	missing.CustomerContext.MCPServer = "nope"
	if err := missing.validateCustomerContext(); err == nil {
		t.Error("unknown mcpServer must be rejected")
	}

	rag := customerContextConfig()
	rag.RAG.Enabled = true
	if err := rag.validateCustomerContext(); err == nil {
		t.Error("rag together with customerContext must be rejected: RAG tools bypass the guard")
	}

	stdio := customerContextConfig()
	stdio.MCPServers["orby"] = MCPServerConfig{Command: "orby-mcp"}
	if err := stdio.validateCustomerContext(); err == nil {
		t.Error("an Orby server without a url and no explicit customerContext.url must be rejected")
	}
}

func TestCustomerContextParsesFromConfigFile(t *testing.T) {
	raw := []byte(`{"customerContext": {"enabled": true, "mcpServer": "orby", "url": "http://x/api/slack-context", "timeoutSeconds": 5}}`)
	var c Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		t.Fatalf("customerContext must be a known config key: %v", err)
	}
	if !c.CustomerContext.Enabled || c.CustomerContext.TimeoutSeconds != 5 || c.CustomerContext.URL != "http://x/api/slack-context" {
		t.Errorf("parsed %+v", c.CustomerContext)
	}
}
