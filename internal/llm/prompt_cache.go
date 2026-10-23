package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// cacheControlInjectionPoints asks a LiteLLM proxy to mark two Anthropic cache
// breakpoints on every chat completion. Claude reads tools before the system
// prompt, so the system breakpoint caches the tool schemas with it; the one on
// the last message caches the conversation so far, so each agent iteration pays
// full price only for what it added. Plain OpenAI rejects the field, which is
// why it is opt-in (llm.providers.openai.promptCaching).
var cacheControlInjectionPoints = []map[string]any{
	{"location": "message", "role": "system"},
	{"location": "message", "index": -1},
}

// httpDoer is the shape langchaingo's openai.WithHTTPClient accepts.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// promptCachingDoer adds cache_control_injection_points to chat-completion
// request bodies and passes every other request through unchanged.
type promptCachingDoer struct {
	next httpDoer
}

func newPromptCachingDoer(next httpDoer) *promptCachingDoer {
	return &promptCachingDoer{next: next}
}

func (d *promptCachingDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost || req.Body == nil || !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return d.next.Do(req)
	}
	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("prompt caching: read request body: %w", err)
	}
	body, err = withInjectionPoints(body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	req.ContentLength = int64(len(body))
	return d.next.Do(req)
}

func withInjectionPoints(body []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("prompt caching: chat completion body is not a JSON object: %w", err)
	}
	points, err := json.Marshal(cacheControlInjectionPoints)
	if err != nil {
		return nil, fmt.Errorf("prompt caching: encode injection points: %w", err)
	}
	payload["cache_control_injection_points"] = points
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("prompt caching: encode request body: %w", err)
	}
	return out, nil
}
