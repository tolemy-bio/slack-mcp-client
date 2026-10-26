package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type recordingDoer struct {
	body []byte
	req  *http.Request
}

func (r *recordingDoer) Do(req *http.Request) (*http.Response, error) {
	r.req = req
	if req.Body != nil {
		r.body, _ = io.ReadAll(req.Body)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
}

// agentPrompt renders the shape of CallAgent's prompt with a scratchpad.
func agentPrompt(scratchpad string) string {
	return "You are Orby.\nTOOLS:\n------\norby_search: finds issues\n\nBegin!\n" +
		historyHeading + "USER: earlier question\n\n" + inputHeading[1:] + "file a bug" +
		thoughtMarker + scratchpad + "\n"
}

func sendThrough(t *testing.T, path, body string) *recordingDoer {
	t.Helper()
	next := &recordingDoer{}
	req, _ := http.NewRequest(http.MethodPost, "https://llm.example/v1"+path, strings.NewReader(body))
	if _, err := newPromptCachingDoer(next).Do(req); err != nil {
		t.Fatalf("Do: %v", err)
	}
	return next
}

func chatBody(t *testing.T, content string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"model":    "claude-sonnet-4-5",
		"messages": []map[string]any{{"role": "user", "content": content}},
	})
	return string(b)
}

func forwardedBlocks(t *testing.T, next *recordingDoer) []cachedTextBlock {
	t.Helper()
	var got struct {
		Model    string `json:"model"`
		Messages []struct {
			Content []cachedTextBlock `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(next.body, &got); err != nil {
		t.Fatalf("forwarded body: %v\n%s", err, next.body)
	}
	if got.Model != "claude-sonnet-4-5" {
		t.Fatalf("model lost: %s", next.body)
	}
	return got.Messages[0].Content
}

func TestAgentPromptGetsTwoBreakpointsAndKeepsItsText(t *testing.T) {
	prompt := agentPrompt(" I should search.\nAction: orby_search\nObservation: none\nThought:")
	next := sendThrough(t, "/chat/completions", chatBody(t, prompt))

	blocks := forwardedBlocks(t, next)
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3", len(blocks))
	}
	var joined strings.Builder
	for i, b := range blocks {
		joined.WriteString(b.Text)
		if marked := b.CacheControl["type"] == "ephemeral"; marked != (i < 2) {
			t.Errorf("block %d cache_control = %v", i, b.CacheControl)
		}
	}
	if joined.String() != prompt {
		t.Fatalf("blocks do not concatenate to the prompt")
	}
	if !strings.HasPrefix(blocks[2].Text, " I should search.") {
		t.Fatalf("scratchpad block = %q", blocks[2].Text)
	}
	if next.req.ContentLength != int64(len(next.body)) {
		t.Fatalf("ContentLength %d, body %d bytes", next.req.ContentLength, len(next.body))
	}
	again, _ := next.req.GetBody()
	if replayed, _ := io.ReadAll(again); string(replayed) != string(next.body) {
		t.Fatal("GetBody does not replay the rewritten body")
	}
}

func TestCachedPrefixIsTheSameAcrossIterations(t *testing.T) {
	first := forwardedBlocks(t, sendThrough(t, "/chat/completions", chatBody(t, agentPrompt(""))))
	later := forwardedBlocks(t, sendThrough(t, "/chat/completions",
		chatBody(t, agentPrompt(" Do I need a tool? Yes\nAction: orby_search\nObservation: {}\nThought:"))))

	if first[0].Text != later[0].Text || first[1].Text != later[1].Text {
		t.Fatal("cached blocks changed between iterations")
	}
	if len(first) != 3 || strings.TrimSpace(first[2].Text) != "" {
		t.Fatalf("first iteration blocks = %+v", first)
	}
}

func TestMarkerInsideTheInputCutsEarlierNotInTheScratchpad(t *testing.T) {
	prompt := strings.Replace(agentPrompt(" step\nThought:"), "file a bug", "file a bug\n\nThought: it crashed", 1)
	blocks := forwardedBlocks(t, sendThrough(t, "/chat/completions", chatBody(t, prompt)))

	if strings.Contains(blocks[1].Text, " step") {
		t.Fatalf("second cut landed in the scratchpad: %q", blocks[1].Text)
	}
}

func TestNonAgentPromptsAndOtherRequestsPassThrough(t *testing.T) {
	plain := chatBody(t, "Describe this screenshot")
	if next := sendThrough(t, "/chat/completions", plain); string(next.body) != plain {
		t.Fatalf("plain prompt changed: %s", next.body)
	}
	const embeddings = `{"input":"hello","model":"text-embedding-005"}`
	if next := sendThrough(t, "/embeddings", embeddings); string(next.body) != embeddings {
		t.Fatalf("embeddings body changed: %s", next.body)
	}
}

func TestBodyWithoutMessagesFailsTheRequest(t *testing.T) {
	for _, body := range []string{`[1,2]`, `{"model":"m"}`} {
		next := &recordingDoer{}
		req, _ := http.NewRequest(http.MethodPost, "https://llm.example/v1/chat/completions", strings.NewReader(body))
		if _, err := newPromptCachingDoer(next).Do(req); err == nil {
			t.Fatalf("%s: want an error", body)
		}
		if next.req != nil {
			t.Fatalf("%s: forwarded despite the error", body)
		}
	}
}
