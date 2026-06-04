// Package safehttp provides SSRF-resistant HTTP building blocks shared by the
// tools that fetch URLs (web_fetch, http_request). Every URL an agent handles
// is untrusted — it can originate from search results, fetched pages, or
// message text — so a prompt injection must not be able to reach internal
// services or the cloud metadata endpoint. The guards here block non-public
// addresses, embedded credentials, and non-http(s) schemes, and re-validate
// every redirect.
package safehttp

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// IsBlockedIP reports whether dialing ip would reach a non-public address.
// Link-local (169.254.0.0/16, fe80::/10) covers the cloud metadata IP
// 169.254.169.254; IsPrivate covers RFC1918 and fc00::/7; CGNAT 100.64.0.0/10
// is handled explicitly.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// control is a net.Dialer.Control hook. It runs after DNS resolution with the
// concrete ip:port about to be dialed, catching hostnames that resolve to
// internal IPs (DNS rebinding included) at connect time.
func control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("ssrf guard: bad address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("ssrf guard: unresolved host %q", host)
	}
	if IsBlockedIP(ip) {
		return fmt.Errorf("ssrf guard: blocked non-public address %s", ip)
	}
	return nil
}

// ValidateURL checks scheme and embedded-credential rules before any network
// access. Only http(s) is allowed; URLs with userinfo (which can smuggle
// tokens) are rejected. IP-range blocking is handled separately (BlockedLiteralIP
// and the dialer Control hook), so this stays guard-independent.
func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return nil, fmt.Errorf("unsupported scheme %q (only http/https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url has no host")
	}
	if u.User != nil {
		return nil, fmt.Errorf("url must not contain embedded credentials")
	}
	return u, nil
}

// BlockedLiteralIP reports whether the URL's host is a bare IP literal in a
// non-public range. Used to fail early with a clear error before dialing; the
// dialer Control hook is the real enforcement (it also catches hostnames that
// resolve to internal IPs).
func BlockedLiteralIP(u *url.URL) bool {
	ip := net.ParseIP(u.Hostname())
	return ip != nil && IsBlockedIP(ip)
}

// Config configures a guarded Client.
type Config struct {
	Timeout      time.Duration // per-request timeout
	BlockPrivate bool          // enable the SSRF guard (SHOULD be true in production)
	MaxRedirects int           // 0 → 5
}

// Client builds an *http.Client that enforces the SSRF guard (when
// cfg.BlockPrivate) at dial time and re-validates every redirect target.
func Client(cfg Config) *http.Client {
	maxRedirects := cfg.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 5
	}
	dialer := &net.Dialer{Timeout: cfg.Timeout, KeepAlive: 30 * time.Second}
	if cfg.BlockPrivate {
		dialer.Control = control
	}
	tr := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   cfg.Timeout,
		ResponseHeaderTimeout: cfg.Timeout,
		MaxIdleConns:          10,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			u, err := ValidateURL(req.URL.String())
			if err != nil {
				return fmt.Errorf("blocked redirect to %s: %w", req.URL, err)
			}
			if cfg.BlockPrivate && BlockedLiteralIP(u) {
				return fmt.Errorf("blocked redirect to non-public address %s", u.Hostname())
			}
			return nil
		},
	}
}
