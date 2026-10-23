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

func TestPromptCachingDoerAddsInjectionPoints(t *testing.T) {
	next := &recordingDoer{}
	req, _ := http.NewRequest(http.MethodPost, "https://llm.example/v1/chat/completions",
		strings.NewReader(`{"model":"claude-sonnet-4-5","messages":[{"role":"system","content":"x"}]}`))

	if _, err := newPromptCachingDoer(next).Do(req); err != nil {
		t.Fatalf("Do: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(next.body, &got); err != nil {
		t.Fatalf("forwarded body is not JSON: %v", err)
	}
	if got["model"] != "claude-sonnet-4-5" || got["messages"] == nil {
		t.Fatalf("original fields lost: %s", next.body)
	}
	points, ok := got["cache_control_injection_points"].([]any)
	if !ok || len(points) != 2 {
		t.Fatalf("injection points = %v, want 2", got["cache_control_injection_points"])
	}
	if next.req.ContentLength != int64(len(next.body)) {
		t.Fatalf("ContentLength %d, body %d bytes", next.req.ContentLength, len(next.body))
	}
	again, _ := next.req.GetBody()
	replayed, _ := io.ReadAll(again)
	if string(replayed) != string(next.body) {
		t.Fatalf("GetBody does not replay the rewritten body")
	}
}

func TestPromptCachingDoerPassesOtherRequestsThrough(t *testing.T) {
	next := &recordingDoer{}
	const body = `{"input":"hello","model":"text-embedding-005"}`
	req, _ := http.NewRequest(http.MethodPost, "https://llm.example/v1/embeddings", strings.NewReader(body))

	if _, err := newPromptCachingDoer(next).Do(req); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if string(next.body) != body {
		t.Fatalf("embeddings body changed: %s", next.body)
	}
}

func TestPromptCachingDoerRejectsNonObjectBody(t *testing.T) {
	next := &recordingDoer{}
	req, _ := http.NewRequest(http.MethodPost, "https://llm.example/v1/chat/completions", strings.NewReader(`[1,2]`))

	if _, err := newPromptCachingDoer(next).Do(req); err == nil {
		t.Fatal("want an error for a body that is not a JSON object")
	}
	if next.req != nil {
		t.Fatal("request was forwarded despite the error")
	}
}
