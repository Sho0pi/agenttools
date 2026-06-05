package httpreq

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/sho0pi/agenttools/internal/safehttp"
	"github.com/sho0pi/agenttools/tool"
)

var allowedMethods = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodHead: true,
}

// redactedHeaders are never echoed back in the result, to avoid leaking secrets
// (request auth) or session material (response cookies) into the transcript.
var redactedHeaders = map[string]bool{
	"authorization":       true,
	"set-cookie":          true,
	"cookie":              true,
	"proxy-authorization": true,
}

// Args are the http_request arguments.
type Args struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	AuthProfile string            `json:"auth_profile"`
}

// New returns the http_request tool, or an error if construction fails.
func New(cfg Config) (tool.Tool, error) {
	cfg = cfg.withDefaults()
	return tool.NewTypedTool(
		"http_request",
		"Call an HTTP API: choose the method, set headers and a body, and get back the raw "+
			"status, headers, and body (JSON intact, not markdown-converted). Use for REST/JSON "+
			"APIs and webhooks. To read a human web page as text, use web_fetch instead; to find "+
			"pages, use web_search. Internal/private addresses are blocked. Never put secrets in "+
			"headers — use auth_profile to reference a configured credential.",
		tool.Object(map[string]*tool.Property{
			"url":          {Type: "string", Description: "Target URL (http/https). Private addresses are blocked."},
			"method":       {Type: "string", Description: "HTTP method (default GET).", Enum: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}},
			"headers":      {Type: "object", Description: "Request headers, e.g. {\"Content-Type\": \"application/json\"}."},
			"body":         {Type: "string", Description: "Raw request body; for JSON pass a JSON-encoded string."},
			"auth_profile": {Type: "string", Description: "Name of a preconfigured credential to apply (no secrets inline)."},
		}, "url"),
		func(ctx context.Context, args Args) (tool.Result, error) {
			return run(ctx, cfg, args)
		},
	), nil
}

func run(ctx context.Context, cfg Config, args Args) (tool.Result, error) {
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !allowedMethods[method] {
		return tool.Result{}, fmt.Errorf("unsupported method %q", args.Method)
	}

	u, err := safehttp.ValidateURL(args.URL)
	if err != nil {
		return tool.Result{}, err
	}
	if cfg.BlockPrivate && safehttp.BlockedLiteralIP(u) {
		return tool.Result{}, fmt.Errorf("blocked non-public address %s", u.Hostname())
	}

	var bodyReader io.Reader
	if args.Body != "" {
		bodyReader = strings.NewReader(args.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return tool.Result{}, fmt.Errorf("build request: %w", err)
	}
	for k, v := range args.Headers {
		req.Header.Set(k, v)
	}

	if profile := strings.TrimSpace(args.AuthProfile); profile != "" {
		if cfg.Auth == nil {
			return tool.Result{}, fmt.Errorf("auth_profile %q requested but no auth provider is configured", profile)
		}
		if err := cfg.Auth(ctx, profile, req); err != nil {
			return tool.Result{}, fmt.Errorf("apply auth profile %q: %w", profile, err)
		}
	}

	resp, err := cfg.Do(req)
	if err != nil {
		return tool.Result{}, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(cfg.MaxBytes)))
	if err != nil {
		return tool.Result{}, fmt.Errorf("read response body: %w", err)
	}

	return tool.Result{
		Content: render(resp, body),
		Data: map[string]any{
			"status":  resp.StatusCode,
			"headers": safeHeaders(resp.Header),
			"body":    string(body),
		},
	}, nil
}

// render formats the response as status line + safe headers + body.
func render(resp *http.Response, body []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "HTTP %s\n", resp.Status)
	hdrs := safeHeaders(resp.Header)
	keys := make([]string, 0, len(hdrs))
	for k := range hdrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, hdrs[k])
	}
	if len(body) > 0 {
		b.WriteByte('\n')
		b.Write(body)
	}
	return strings.TrimRight(b.String(), "\n")
}

// safeHeaders returns response headers with sensitive ones redacted.
func safeHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if redactedHeaders[strings.ToLower(k)] {
			out[k] = "[redacted]"
			continue
		}
		out[k] = strings.Join(v, ", ")
	}
	return out
}
