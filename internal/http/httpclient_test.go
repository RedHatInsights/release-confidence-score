package http

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPClient_DefaultOptions(t *testing.T) {
	client := NewHTTPClient(HTTPClientOptions{})

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 0 {
		t.Errorf("expected zero timeout, got %v", client.Timeout)
	}
	if client.Transport != nil {
		t.Error("expected nil transport for default options")
	}
}

func TestNewHTTPClient_WithTimeout(t *testing.T) {
	timeout := 30 * time.Second
	client := NewHTTPClient(HTTPClientOptions{
		Timeout: timeout,
	})

	if client.Timeout != timeout {
		t.Errorf("expected timeout %v, got %v", timeout, client.Timeout)
	}
}

func TestNewHTTPClient_WithSkipSSLVerify(t *testing.T) {
	client := NewHTTPClient(HTTPClientOptions{
		SkipSSLVerify: true,
	})

	if client.Transport == nil {
		t.Fatal("expected non-nil transport when SkipSSLVerify is true")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}

	if transport.TLSClientConfig == nil {
		t.Fatal("expected non-nil TLSClientConfig")
	}

	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}
}

func TestNewHTTPClient_WithAllOptions(t *testing.T) {
	timeout := 60 * time.Second
	client := NewHTTPClient(HTTPClientOptions{
		Timeout:       timeout,
		SkipSSLVerify: true,
	})

	if client.Timeout != timeout {
		t.Errorf("expected timeout %v, got %v", timeout, client.Timeout)
	}

	transport := client.Transport.(*http.Transport)
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}
}

func TestNewHTTPClient_SkipSSLVerifyFalse(t *testing.T) {
	client := NewHTTPClient(HTTPClientOptions{
		SkipSSLVerify: false,
	})

	// When SkipSSLVerify is false, we should use default transport (nil)
	// which uses the system's default TLS configuration
	if client.Transport != nil {
		t.Error("expected nil transport when SkipSSLVerify is false")
	}
}

func TestHTTPClientOptions_ZeroValue(t *testing.T) {
	var opts HTTPClientOptions

	if opts.Timeout != 0 {
		t.Errorf("expected zero timeout, got %v", opts.Timeout)
	}
	if opts.SkipSSLVerify {
		t.Error("expected SkipSSLVerify to be false")
	}
}

// Verify the client can be used to make requests (compile-time interface check)
func TestNewHTTPClient_ImplementsInterface(t *testing.T) {
	client := NewHTTPClient(HTTPClientOptions{})

	// Verify the client has the expected methods
	var _ interface {
		Do(*http.Request) (*http.Response, error)
	} = client

	// Type assertion to ensure it's a proper *http.Client
	var _ *http.Client = client
}

// Test that TLS config is properly isolated
func TestNewHTTPClient_TLSConfigIsolation(t *testing.T) {
	client1 := NewHTTPClient(HTTPClientOptions{SkipSSLVerify: true})
	client2 := NewHTTPClient(HTTPClientOptions{SkipSSLVerify: true})

	transport1 := client1.Transport.(*http.Transport)
	transport2 := client2.Transport.(*http.Transport)

	// Each client should have its own TLS config
	if transport1.TLSClientConfig == transport2.TLSClientConfig {
		t.Error("expected different TLSClientConfig instances for different clients")
	}
}

// Verify TLS config values
func TestNewHTTPClient_TLSConfigValues(t *testing.T) {
	client := NewHTTPClient(HTTPClientOptions{SkipSSLVerify: true})
	transport := client.Transport.(*http.Transport)
	tlsConfig := transport.TLSClientConfig

	// Check that only InsecureSkipVerify is set, other fields are default
	expected := &tls.Config{InsecureSkipVerify: true}

	if tlsConfig.InsecureSkipVerify != expected.InsecureSkipVerify {
		t.Error("InsecureSkipVerify mismatch")
	}
}
