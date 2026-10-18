package orbycontext

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sampleFacts = Facts{ChannelID: "C0ANOMER", ChannelType: "channel", UserID: "U0GUEST",
	UserTeamID: "T0ANOMER", HomeTeamID: "T0TOLEMY", UserIsStranger: true}

func serve(t *testing.T, status int, body string) (*HTTPResolver, *http.Request, *[]byte) {
	t.Helper()
	var gotReq http.Request
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = *r
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	r := NewHTTPResolver(srv.URL+"/api/slack-context", map[string]string{"Authorization": "Bearer tok"}, 2*time.Second)
	return r, &gotReq, &gotBody
}

func TestResolve_Internal(t *testing.T) {
	r, req, body := serve(t, 200, `{"mode":"internal","customer":null,"allowed_tools":null,"message":null}`)

	d, err := r.Resolve(context.Background(), sampleFacts)

	require.NoError(t, err)
	assert.Equal(t, ModeInternal, d.Mode)
	assert.Nil(t, d.AllowedTools, "null allowed_tools means every tool")
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "/api/slack-context", req.URL.Path)
	assert.Equal(t, "Bearer tok", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var sent Facts
	require.NoError(t, json.Unmarshal(*body, &sent))
	assert.Equal(t, sampleFacts, sent)
}

func TestResolve_Customer(t *testing.T) {
	r, _, _ := serve(t, 200, `{"mode":"customer","customer":{"key":"anomer","name":"Anomer"},
		"allowed_tools":["create_bug","get_linear_issue"],"message":null}`)

	d, err := r.Resolve(context.Background(), sampleFacts)

	require.NoError(t, err)
	assert.Equal(t, ModeCustomer, d.Mode)
	assert.Equal(t, "Anomer", d.Customer.Name)
	assert.Equal(t, []string{"create_bug", "get_linear_issue"}, d.AllowedTools)
}

func TestResolve_CustomerWithEmptyListIsValid(t *testing.T) {
	r, _, _ := serve(t, 200, `{"mode":"customer","customer":{"key":"anomer","name":"Anomer"},"allowed_tools":[]}`)

	d, err := r.Resolve(context.Background(), sampleFacts)

	require.NoError(t, err)
	assert.NotNil(t, d.AllowedTools)
	assert.Empty(t, d.AllowedTools)
}

func TestResolve_Blocked(t *testing.T) {
	r, _, _ := serve(t, 200, `{"mode":"blocked","customer":null,"allowed_tools":null,"message":"Ask in #anomer-orbit-beta."}`)

	d, err := r.Resolve(context.Background(), sampleFacts)

	require.NoError(t, err)
	assert.Equal(t, ModeBlocked, d.Mode)
	assert.Equal(t, "Ask in #anomer-orbit-beta.", d.MessageText())
}

func TestResolve_Errors(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"5xx":                              {500, `{"detail":"boom"}`},
		"4xx":                              {401, `{"detail":"unauthorized"}`},
		"not json":                         {200, `<html>bad gateway</html>`},
		"unknown mode":                     {200, `{"mode":"admin"}`},
		"missing mode":                     {200, `{}`},
		"customer without allowed_tools":   {200, `{"mode":"customer","customer":{"key":"anomer","name":"Anomer"},"allowed_tools":null}`},
		"customer without customer object": {200, `{"mode":"customer","customer":null,"allowed_tools":["create_bug"]}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, _, _ := serve(t, tc.status, tc.body)
			d, err := r.Resolve(context.Background(), sampleFacts)
			assert.Error(t, err)
			assert.Nil(t, d)
		})
	}
}

func TestResolve_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	d, err := NewHTTPResolver(url, nil, time.Second).Resolve(context.Background(), sampleFacts)

	assert.Error(t, err)
	assert.Nil(t, d)
}

func TestResolve_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)

	d, err := NewHTTPResolver(srv.URL, nil, 100*time.Millisecond).Resolve(context.Background(), sampleFacts)

	assert.Error(t, err)
	assert.Nil(t, d)
}
