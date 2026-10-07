package slackbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slack-go/slack/slackevents"

	"github.com/tolemy-bio/slack-mcp-client/internal/common/logging"
	"github.com/tolemy-bio/slack-mcp-client/internal/config"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\nfake-image-bytes")

func testLogger() *logging.Logger {
	return logging.New("vision-test", logging.LevelError)
}

// newSlackFileServer serves the given body for any request, like Slack's url_private.
func newSlackFileServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer xoxb-test" {
			t.Errorf("missing bot token on Slack download")
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// failingDescriber fails the test if the vision model is called.
type failingDescriber struct{ t *testing.T }

func (f failingDescriber) DescribeImage(_ context.Context, _ string, _ []byte) (string, error) {
	f.t.Fatal("vision should not be called")
	return "", nil
}

func TestDownloadAndParseFile_DescribesScreenshot(t *testing.T) {
	var got map[string]any
	vision := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-litellm" {
			t.Errorf("missing LiteLLM API key")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Error dialog: \"Upload failed (413)\""}}]}`))
	}))
	defer vision.Close()
	slackSrv := newSlackFileServer(t, pngBytes)

	file := slackevents.File{Name: "bug.png", Mimetype: "image/png", Filetype: "png", Size: len(pngBytes), URLPrivate: slackSrv.URL + "/files/bug.png"}
	att, err := DownloadAndParseFile("xoxb-test", file, NewVisionClient(vision.URL+"/v1/", "sk-litellm", "claude-sonnet-5"), testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `[Screenshot "bug.png" (Slack file URL: ` + file.URLPrivate + `) — description: Error dialog: "Upload failed (413)"]`
	if att.Content != want {
		t.Errorf("content = %q\nwant %q", att.Content, want)
	}
	if got["model"] != "claude-sonnet-5" {
		t.Errorf("model = %v", got["model"])
	}
	raw, _ := json.Marshal(got["messages"])
	if !strings.Contains(string(raw), `"image_url":{"url":"data:image/png;base64,`) {
		t.Errorf("request missing base64 image_url part: %s", raw)
	}
}

func TestDownloadAndParseFile_VisionAPIError(t *testing.T) {
	vision := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"model not found"}`, http.StatusBadRequest)
	}))
	defer vision.Close()
	slackSrv := newSlackFileServer(t, pngBytes)

	file := slackevents.File{Name: "shot.jpg", Mimetype: "image/jpeg", Size: len(pngBytes), URLPrivate: slackSrv.URL + "/files/shot.jpg"}
	att, err := DownloadAndParseFile("xoxb-test", file, NewVisionClient(vision.URL, "k", "m"), testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range []string{`"shot.jpg"`, file.URLPrivate, "not analysed", "HTTP 400", "model not found"} {
		if !strings.Contains(att.Content, s) {
			t.Errorf("content %q missing %q", att.Content, s)
		}
	}
}

func TestDownloadAndParseFile_OversizeImageSkipped(t *testing.T) {
	file := slackevents.File{Name: "huge.webp", Filetype: "webp", Size: maxFileSize + 1, URLPrivate: "https://files.slack.com/huge.webp"}
	att, err := DownloadAndParseFile("xoxb-test", file, failingDescriber{t}, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range []string{`"huge.webp"`, file.URLPrivate, "not analysed", "exceeds the 10 MB"} {
		if !strings.Contains(att.Content, s) {
			t.Errorf("content %q missing %q", att.Content, s)
		}
	}
}

func TestDownloadAndParseFile_NonImageUnaffected(t *testing.T) {
	slackSrv := newSlackFileServer(t, []byte("a,b\n1,2\n"))
	file := slackevents.File{Name: "data.csv", Mimetype: "text/csv", Filetype: "csv", Size: 8, URLPrivate: slackSrv.URL + "/data.csv"}
	att, err := DownloadAndParseFile("xoxb-test", file, failingDescriber{t}, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if att.Content != "a,b\n1,2\n" {
		t.Errorf("content = %q", att.Content)
	}
}

func TestNewImageDescriberFromConfig_RequiresBaseURL(t *testing.T) {
	cfg := &config.Config{LLM: config.LLMConfig{Provider: "openai", Providers: map[string]config.LLMProviderConfig{"openai": {Model: "m"}}}}
	_, err := NewImageDescriberFromConfig(cfg).DescribeImage(context.Background(), "image/png", pngBytes)
	if err == nil || !strings.Contains(err.Error(), "baseUrl") {
		t.Errorf("expected baseUrl error, got %v", err)
	}
}

func TestNewImageDescriberFromConfig_VisionModelFallback(t *testing.T) {
	providers := map[string]config.LLMProviderConfig{"openai": {Model: "main", BaseURL: "https://llm.example/v1"}}
	cfg := &config.Config{LLM: config.LLMConfig{Provider: "openai", Providers: providers}}
	if m := NewImageDescriberFromConfig(cfg).(*VisionClient).model; m != "main" {
		t.Errorf("fallback model = %q, want main", m)
	}
	providers["openai"] = config.LLMProviderConfig{Model: "main", VisionModel: "claude-sonnet-5", BaseURL: "https://llm.example/v1"}
	if m := NewImageDescriberFromConfig(cfg).(*VisionClient).model; m != "claude-sonnet-5" {
		t.Errorf("vision model = %q, want claude-sonnet-5", m)
	}
}
