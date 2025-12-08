package http

import (
	"crypto/tls"
	"net/http"
	"time"
)

// HTTPClientOptions configures HTTP client creation
type HTTPClientOptions struct {
	// Timeout is the request timeout duration (0 means no timeout)
	Timeout time.Duration
	// SkipSSLVerify disables SSL certificate verification (use with caution)
	SkipSSLVerify bool
}

// NewHTTPClient creates an HTTP client with the specified options
func NewHTTPClient(opts HTTPClientOptions) *http.Client {
	client := &http.Client{
		Timeout: opts.Timeout,
	}

	// Only configure custom transport if SSL verification needs to be skipped
	if opts.SkipSSLVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return client
}
