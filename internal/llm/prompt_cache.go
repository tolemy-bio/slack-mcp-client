package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The conversational agent (langchain.go, CallAgent) sends its whole prompt as
// one user message: prefix with the tool descriptions, format instructions,
// suffix with the history and the input, then "Thought:" and the scratchpad,
// which grows by one step per iteration. A cache breakpoint at the end of that
// message never hits, because the next iteration's message is longer. So the
// message is cut into text blocks at two boundaries that do not move, and each
// of the first two blocks is marked as an Anthropic cache breakpoint:
//
//   - before the history heading: tools and instructions, the same for every
//     run with the same user and scope, so runs within the cache TTL share it;
//   - after the "Thought:" that follows the input: everything but the
//     scratchpad, the same for every iteration of one run.
//
// The blocks concatenate to the original text, so the model reads the same
// prompt. The markers are the agent suffix's own text in langchain.go; a
// prompt without them (the non-agent path) is sent as is.
const (
	historyHeading = "\nPrevious conversation history:\n"
	inputHeading   = "\nNew input: "
	thoughtMarker  = "\n\nThought:"
)

// httpDoer is the shape langchaingo's openai.WithHTTPClient accepts.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// promptCachingDoer marks cache breakpoints in chat-completion request bodies
// and passes every other request through unchanged. Only for a LiteLLM proxy in
// front of Claude: LiteLLM forwards cache_control on text blocks to Anthropic.
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
	body, err = withCacheBreakpoints(body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	req.ContentLength = int64(len(body))
	return d.next.Do(req)
}

type cachedTextBlock struct {
	Type         string            `json:"type"`
	Text         string            `json:"text"`
	CacheControl map[string]string `json:"cache_control,omitempty"`
}

// withCacheBreakpoints rewrites the last message, when it is a user message
// with plain-text content holding the agent markers, into cache-marked blocks.
func withCacheBreakpoints(body []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("prompt caching: chat completion body is not a JSON object: %w", err)
	}
	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(payload["messages"], &messages); err != nil || len(messages) == 0 {
		return nil, fmt.Errorf("prompt caching: chat completion body has no messages array (%v)", err)
	}
	last := messages[len(messages)-1]
	var role, text string
	if json.Unmarshal(last["role"], &role) != nil || role != "user" || json.Unmarshal(last["content"], &text) != nil {
		return body, nil
	}
	blocks := splitAgentPrompt(text)
	if blocks == nil {
		return body, nil
	}
	content, err := json.Marshal(blocks)
	if err != nil {
		return nil, fmt.Errorf("prompt caching: encode content blocks: %w", err)
	}
	last["content"] = content
	if payload["messages"], err = json.Marshal(messages); err != nil {
		return nil, fmt.Errorf("prompt caching: encode messages: %w", err)
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("prompt caching: encode request body: %w", err)
	}
	return out, nil
}

// splitAgentPrompt cuts an agent prompt at the two stable boundaries, or
// returns nil when the text is not an agent prompt. A marker repeated inside
// the history or the input moves a cut earlier, never into the scratchpad, so
// the cached prefix stays the same from one iteration to the next.
func splitAgentPrompt(text string) []cachedTextBlock {
	history := strings.Index(text, historyHeading)
	if history <= 0 {
		return nil
	}
	input := strings.Index(text[history:], inputHeading)
	if input < 0 {
		return nil
	}
	input += history
	thought := strings.Index(text[input:], thoughtMarker)
	if thought < 0 {
		return nil
	}
	scratchpad := input + thought + len(thoughtMarker)
	ephemeral := map[string]string{"type": "ephemeral"}
	blocks := []cachedTextBlock{
		{Type: "text", Text: text[:history], CacheControl: ephemeral},
		{Type: "text", Text: text[history:scratchpad], CacheControl: ephemeral},
	}
	if scratchpad < len(text) {
		blocks = append(blocks, cachedTextBlock{Type: "text", Text: text[scratchpad:]})
	}
	return blocks
}
