package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTPClientOptions configures HTTP client creation
type HTTPClientOptions struct {
	// Timeout is the request timeout duration (0 means no timeout)
	Timeout time.Duration
	// SkipSSLVerify disables SSL certificate verification (use with caution)
	SkipSSLVerify bool
	// BlockPrivateIPs rejects connections that resolve to private, loopback,
	// link-local, or otherwise non-public addresses (including cloud metadata
	// endpoints). Use for requests to attacker-influenced URLs to prevent SSRF.
	BlockPrivateIPs bool
	// AllowedPrivateHost, if set, exempts this exact hostname (case-insensitive)
	// from the private-IP block -- e.g. a configured internal GitLab instance
	// that may legitimately live on a private IP. The exemption is checked per
	// dial, so a redirect away from this host to any other host is still blocked.
	AllowedPrivateHost string
}

// NewHTTPClient creates an HTTP client with the specified options
func NewHTTPClient(opts HTTPClientOptions) *http.Client {
	client := &http.Client{
		Timeout: opts.Timeout,
	}

	// Only configure custom transport if SSL verification needs to be skipped
	// or private IPs need to be blocked
	if opts.SkipSSLVerify || opts.BlockPrivateIPs {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		}

		if opts.SkipSSLVerify {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}

		if opts.BlockPrivateIPs {
			transport.DialContext = safeDialContext(opts.AllowedPrivateHost)
		}

		client.Transport = transport
	}

	return client
}

// safeDialContext returns a DialContext that resolves addr and dials only
// public IP addresses, rejecting private, loopback, link-local, and
// unspecified ranges, except for an exact match on allowedHost. Validation
// happens at dial time rather than URL-parse time, so it also covers HTTP
// redirects (each hop is dialed and checked independently) and cannot be
// bypassed by DNS rebinding between resolution and connection.
func safeDialContext(allowedHost string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q: %w", addr, err)
		}

		dialer := &net.Dialer{}

		if allowedHost != "" && strings.EqualFold(host, allowedHost) {
			return dialer.DialContext(ctx, network, addr)
		}

		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve host %q: %w", host, err)
		}

		var lastErr error
		for _, ip := range ips {
			if isBlockedIP(ip) {
				lastErr = fmt.Errorf("connection to %q blocked: resolves to non-public address %s", host, ip)
				continue
			}

			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}

		if lastErr == nil {
			lastErr = fmt.Errorf("no addresses found for host %q", host)
		}
		return nil, lastErr
	}
}

// carrierGradeNAT is the RFC 6598 shared address space (100.64.0.0/10), used
// internally by cloud NAT gateways; net.IP does not classify it as private.
var carrierGradeNAT = net.IPNet{IP: net.IPv4(100, 64, 0, 0).To4(), Mask: net.CIDRMask(10, 32)}

// nat64Prefix (RFC 6052, 64:ff9b::/96) and ipv4CompatiblePrefix (the
// deprecated ::a.b.c.d form) both embed an IPv4 address in their low 32 bits,
// letting an attacker hide a blocked IPv4 target behind an IPv6 literal that
// net.IP's helpers don't unwrap the way they do for ::ffff:a.b.c.d.
var (
	nat64Prefix          = net.IPNet{IP: net.ParseIP("64:ff9b::"), Mask: net.CIDRMask(96, 128)}
	ipv4CompatiblePrefix = net.IPNet{IP: net.ParseIP("::"), Mask: net.CIDRMask(96, 128)}
)

// isBlockedIP reports whether ip is a private, loopback, link-local,
// carrier-grade-NAT, or otherwise non-public address. This includes cloud
// metadata endpoints (169.254.0.0/16) and IPv4 addresses embedded in an IPv6
// literal via NAT64 or the deprecated IPv4-compatible form.
func isBlockedIP(ip net.IP) bool {
	if isNonPublic(ip) {
		return true
	}
	if nat64Prefix.Contains(ip) || ipv4CompatiblePrefix.Contains(ip) {
		return isNonPublic(ip.To16()[12:16])
	}
	return false
}

func isNonPublic(ip net.IP) bool {
	return ip.IsPrivate() ||
		ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast() ||
		carrierGradeNAT.Contains(ip)
}
