// Package httpreq provides the http_request tool: a generic HTTP client for
// calling JSON/REST APIs. It is the machine-API counterpart to web_fetch (which
// reads human web pages as markdown) — http_request returns the raw status,
// headers, and body intact, with arbitrary method/headers/body.
//
// All requests go through an SSRF-guarded transport by default, so a
// prompt-injected URL cannot reach internal services. Named auth profiles are
// resolved by an AuthProfile provider so secrets are injected server-side and
// never appear in tool arguments or the model transcript.
package httpreq

import (
	"context"
	"net/http"
	"time"

	"github.com/sho0pi/agenttools/internal/safehttp"
)

const (
	defaultTimeout  = 30 * time.Second
	defaultMaxBytes = 5 << 20 // 5 MiB response cap
)

// Requester performs an HTTP request and returns the response. Wire in
// client.Do to use a custom *http.Client; nil builds an SSRF-guarded default.
type Requester func(req *http.Request) (*http.Response, error)

// AuthProfile resolves a named credential (e.g. "github") and applies it to
// req server-side without exposing the secret to the model. Pull credentials
// from env vars or a secret store; never hard-code them.
type AuthProfile func(ctx context.Context, name string, req *http.Request) error

// Config configures the http_request tool.
type Config struct {
	// Requester performs requests. If nil, an SSRF-guarded client is built
	// from Timeout and BlockPrivate.
	Requester Requester
	// Auth resolves named auth profiles. If nil, auth_profile is rejected.
	Auth AuthProfile
	// Timeout bounds each request when Requester is nil (default 30s).
	Timeout time.Duration
	// BlockPrivate enables the SSRF guard when Requester is nil (SHOULD be true).
	BlockPrivate bool
	// MaxBytes caps the response body read into the result (default 5 MiB).
	MaxBytes int
}

func (c Config) withDefaults() Config {
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = defaultMaxBytes
	}
	if c.Requester == nil {
		client := safehttp.Client(safehttp.Config{Timeout: c.Timeout, BlockPrivate: c.BlockPrivate})
		c.Requester = client.Do
	}
	return c
}
