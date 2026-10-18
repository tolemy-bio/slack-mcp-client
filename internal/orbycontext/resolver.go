package orbycontext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// maxResponseBytes caps how much of the context endpoint's response is read.
const maxResponseBytes = 1 << 20

// Resolver asks the Orby server which mode applies to a conversation.
type Resolver interface {
	Resolve(ctx context.Context, facts Facts) (*Decision, error)
}

// HTTPResolver calls POST <orby>/api/slack-context. Any failure (network error,
// non-2xx status, malformed or invalid body) is returned as an error, and the
// caller must then not answer: it never assumes "internal".
type HTTPResolver struct {
	url     string
	headers map[string]string
	client  *http.Client
}

// NewHTTPResolver creates a resolver for the given endpoint URL. headers are sent
// with every request (the same auth the client uses for the Orby MCP server).
func NewHTTPResolver(url string, headers map[string]string, timeout time.Duration) *HTTPResolver {
	return &HTTPResolver{
		url:     url,
		headers: headers,
		client:  &http.Client{Timeout: timeout},
	}
}

// Resolve posts the facts and returns the server's validated decision.
func (r *HTTPResolver) Resolve(ctx context.Context, facts Facts) (*Decision, error) {
	body, err := json.Marshal(facts)
	if err != nil {
		return nil, fmt.Errorf("encode slack context facts: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build slack context request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call slack context endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read slack context response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("slack context endpoint returned HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return parseDecision(raw)
}

func parseDecision(raw []byte) (*Decision, error) {
	var decision Decision
	if err := json.Unmarshal(raw, &decision); err != nil {
		return nil, fmt.Errorf("decode slack context response: %w", err)
	}
	if err := decision.Validate(); err != nil {
		return nil, fmt.Errorf("invalid slack context response: %w", err)
	}
	return &decision, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
