package httpreq

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sho0pi/agenttools/tool"
)

// newTool builds an http_request tool with the SSRF guard off so tests can hit
// httptest's loopback server.
func newTool(t *testing.T, cfg Config) tool.Tool {
	t.Helper()
	cfg.BlockPrivate = false
	tr, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tr
}

func call(t *testing.T, tr tool.Tool, args Args) (tool.Result, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	return tr.Execute(context.Background(), raw)
}

func TestHTTPRequest_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"method":%q}`, r.Method)
	}))
	defer srv.Close()

	res, err := call(t, newTool(t, Config{}), Args{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["status"] != 200 {
		t.Fatalf("status = %v", res.Data["status"])
	}
	if !strings.Contains(res.Data["body"].(string), `"method":"GET"`) {
		t.Fatalf("body = %q", res.Data["body"])
	}
}

func TestHTTPRequest_PostBodyHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		_, _ = fmt.Fprintf(w, "method=%s ctype=%s body=%s", r.Method, r.Header.Get("Content-Type"), body)
	}))
	defer srv.Close()

	res, err := call(t, newTool(t, Config{}), Args{
		URL: srv.URL, Method: "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    `{"x":1}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Data["body"].(string)
	if !strings.Contains(got, "method=POST") || !strings.Contains(got, "ctype=application/json") || !strings.Contains(got, `body={"x":1}`) {
		t.Fatalf("server saw: %q", got)
	}
}

func TestHTTPRequest_MethodAndURLValidation(t *testing.T) {
	tr := newTool(t, Config{})
	if _, err := call(t, tr, Args{URL: "http://x.test", Method: "TRACE"}); err == nil {
		t.Fatal("unsupported method should error")
	}
	if _, err := call(t, tr, Args{URL: "ftp://x.test"}); err == nil {
		t.Fatal("non-http scheme should error")
	}
	if _, err := call(t, tr, Args{URL: "http://user:pass@x.test"}); err == nil {
		t.Fatal("embedded credentials should error")
	}
}

func TestHTTPRequest_AuthProfile(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	auth := AuthProfilesFunc(func(_ context.Context, profile string, req *http.Request) error {
		if profile == "demo" {
			req.Header.Set("Authorization", "Bearer secret-token")
		}
		return nil
	})

	res, err := call(t, newTool(t, Config{Auth: auth}), Args{URL: srv.URL, AuthProfile: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer secret-token" {
		t.Fatalf("auth not applied; server saw %q", sawAuth)
	}
	// The secret must not leak into the rendered output.
	if strings.Contains(res.Content, "secret-token") {
		t.Fatalf("auth secret leaked into content:\n%s", res.Content)
	}

	// profile requested but no provider configured → error
	if _, err := call(t, newTool(t, Config{}), Args{URL: srv.URL, AuthProfile: "demo"}); err == nil {
		t.Fatal("auth_profile without provider should error")
	}
}

func TestHTTPRequest_RedactsResponseCookies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Set-Cookie", "session=abc123; HttpOnly")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res, err := call(t, newTool(t, Config{}), Args{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Content, "abc123") {
		t.Fatalf("response cookie leaked:\n%s", res.Content)
	}
}

func TestHTTPRequest_BodyCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 10000)))
	}))
	defer srv.Close()

	res, err := call(t, newTool(t, Config{MaxBytes: 100}), Args{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Data["body"].(string)) > 100 {
		t.Fatalf("body not capped: %d bytes", len(res.Data["body"].(string)))
	}
}
