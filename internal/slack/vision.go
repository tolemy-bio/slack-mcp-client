package slackbot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tolemy-bio/slack-mcp-client/internal/config"
)

const (
	visionTimeout     = 60 * time.Second
	visionMaxTokens   = 1500
	maxErrorBodyChars = 500
	screenshotPrompt  = `You are helping triage a bug report. Describe this screenshot thoroughly so that an engineer who cannot see it can understand the problem. Include:
- Any error messages, warnings, toasts or dialog text, quoted EXACTLY and verbatim.
- Which application screen/page is shown, and the URL in the address bar if visible.
- The state of any forms (field labels and their values, selected options, disabled buttons).
- Highlighted, selected, annotated (arrows, circles) or focused elements.
- Anything that looks broken, missing, misaligned or unexpected.
Be factual; do not guess beyond what is visible. Plain text, no markdown headings.`
)

// imageMimeByExt maps supported screenshot extensions to their MIME type.
var imageMimeByExt = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"webp": "image/webp",
}

// ImageDescriber turns image bytes into a text description.
type ImageDescriber interface {
	DescribeImage(ctx context.Context, mimetype string, data []byte) (string, error)
}

// VisionClient calls an OpenAI-compatible chat/completions endpoint (the LiteLLM proxy).
type VisionClient struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

// unavailableDescriber reports why vision is not configured on every call.
type unavailableDescriber struct{ reason error }

func (u unavailableDescriber) DescribeImage(context.Context, string, []byte) (string, error) {
	return "", u.reason
}

// NewImageDescriberFromConfig builds a describer from the main LLM provider config.
// It never returns nil: if vision cannot be configured, the returned describer
// reports the reason on every call so it surfaces in the screenshot placeholder.
func NewImageDescriberFromConfig(cfg *config.Config) ImageDescriber {
	if cfg.LLM.Provider != config.ProviderOpenAI {
		return unavailableDescriber{fmt.Errorf("vision requires the OpenAI-compatible provider, current provider is %q", cfg.LLM.Provider)}
	}
	p := cfg.LLM.Providers[cfg.LLM.Provider]
	if p.BaseURL == "" {
		return unavailableDescriber{fmt.Errorf("vision requires llm.providers.openai.baseUrl (LiteLLM proxy) to be set")}
	}
	model := p.VisionModel
	if model == "" {
		model = p.Model
	}
	return NewVisionClient(p.BaseURL, p.APIKey, model)
}

// NewVisionClient creates a client for {baseURL}/chat/completions.
func NewVisionClient(baseURL, apiKey, model string) *VisionClient {
	return &VisionClient{
		endpoint:   strings.TrimRight(baseURL, "/") + "/chat/completions",
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: visionTimeout},
	}
}

type chatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL string `json:"url"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Messages  []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string            `json:"role"`
	Content []chatContentPart `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// DescribeImage sends the image as a base64 data URL and returns the model's description.
func (v *VisionClient) DescribeImage(ctx context.Context, mimetype string, data []byte) (string, error) {
	body, err := json.Marshal(v.buildRequest(mimetype, data))
	if err != nil {
		return "", fmt.Errorf("encode vision request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create vision request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if v.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+v.apiKey)
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vision request failed: %w", err)
	}
	defer resp.Body.Close()
	return parseVisionResponse(resp)
}

func (v *VisionClient) buildRequest(mimetype string, data []byte) chatRequest {
	dataURL := "data:" + mimetype + ";base64," + base64.StdEncoding.EncodeToString(data)
	return chatRequest{
		Model:     v.model,
		MaxTokens: visionMaxTokens,
		Messages: []chatMessage{{
			Role: "user",
			Content: []chatContentPart{
				{Type: "text", Text: screenshotPrompt},
				{Type: "image_url", ImageURL: &chatImageURL{URL: dataURL}},
			},
		}},
	}
}

func parseVisionResponse(resp *http.Response) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read vision response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vision API returned HTTP %d: %s", resp.StatusCode, truncate(string(raw), maxErrorBodyChars))
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode vision response: %w", err)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("vision API returned an empty description")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// imageMimetype returns the MIME type for a supported screenshot, or "" if the file is not one.
func imageMimetype(name, mimetype, filetype string) string {
	for _, m := range imageMimeByExt {
		if strings.EqualFold(mimetype, m) {
			return m
		}
	}
	if m, ok := imageMimeByExt[strings.ToLower(filetype)]; ok {
		return m
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	return imageMimeByExt[ext]
}

// describeScreenshot returns the prompt text for an image attachment.
func describeScreenshot(ctx context.Context, d ImageDescriber, name, slackURL, mimetype string, data []byte) string {
	description, err := d.DescribeImage(ctx, mimetype, data)
	if err != nil {
		return screenshotUnavailable(name, slackURL, "could not be described: "+err.Error())
	}
	return fmt.Sprintf("[Screenshot %q (Slack file URL: %s) — description: %s]", name, slackURL, description)
}

func screenshotUnavailable(name, slackURL, reason string) string {
	return fmt.Sprintf("[Screenshot %q (Slack file URL: %s) — not analysed, %s]", name, slackURL, reason)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// cachingDescriber remembers successful descriptions by image content, so a
// screenshot that is re-read on every follow-up message in a thread is only
// sent to the vision model once. Failures are not cached.
type cachingDescriber struct {
	inner  ImageDescriber
	mu     sync.Mutex
	byHash map[[sha256.Size]byte]string
}

const descriptionCacheMaxEntries = 256

// WithDescriptionCache wraps a describer with an in-memory content-hash cache.
func WithDescriptionCache(inner ImageDescriber) ImageDescriber {
	return &cachingDescriber{inner: inner, byHash: make(map[[sha256.Size]byte]string)}
}

func (c *cachingDescriber) DescribeImage(ctx context.Context, mimetype string, data []byte) (string, error) {
	key := sha256.Sum256(data)
	c.mu.Lock()
	cached, ok := c.byHash[key]
	c.mu.Unlock()
	if ok {
		return cached, nil
	}
	desc, err := c.inner.DescribeImage(ctx, mimetype, data)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	if len(c.byHash) >= descriptionCacheMaxEntries {
		c.byHash = make(map[[sha256.Size]byte]string)
	}
	c.byHash[key] = desc
	c.mu.Unlock()
	return desc, nil
}
