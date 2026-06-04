// Package httpreq provides the http_request tool: a generic HTTP client for
// calling JSON/REST APIs. It is the machine-API counterpart to web_fetch (which
// reads human web pages as markdown) — http_request returns the raw status,
// headers, and body intact, with arbitrary method/headers/body.
//
// All requests go through an SSRF-guarded transport by default, so a
// prompt-injected URL cannot reach internal services. Named auth profiles are
// resolved by an AuthProfiles provider so secrets are injected server-side and
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

// Doer performs an HTTP request. *http.Client satisfies it; the default is an
// SSRF-guarded client.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// AuthProfiles resolves a named profile (e.g. "github") into request auth,
// applying it to req without exposing the secret to the model. Implementations
// pull credentials from env vars or a secret store.
type AuthProfiles interface {
	Apply(ctx context.Context, profile string, req *http.Request) error
}

// AuthProfilesFunc adapts a plain function to the AuthProfiles interface.
type AuthProfilesFunc func(ctx context.Context, profile string, req *http.Request) error

// Apply implements AuthProfiles.
func (f AuthProfilesFunc) Apply(ctx context.Context, profile string, req *http.Request) error {
	return f(ctx, profile, req)
}

// Config configures the http_request tool.
type Config struct {
	// Doer performs requests. If nil, an SSRF-guarded client is built from
	// Timeout and BlockPrivate.
	Doer Doer
	// Auth resolves named auth profiles. If nil, the auth_profile argument is
	// rejected.
	Auth AuthProfiles
	// Timeout bounds each request when Doer is nil (default 30s).
	Timeout time.Duration
	// BlockPrivate enables the SSRF guard when Doer is nil (SHOULD be true).
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
	if c.Doer == nil {
		c.Doer = safehttp.Client(safehttp.Config{Timeout: c.Timeout, BlockPrivate: c.BlockPrivate})
	}
	return c
}
