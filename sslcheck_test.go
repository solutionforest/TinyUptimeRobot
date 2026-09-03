package main

import (
	"strings"
	"testing"
	"time"
)

func defaultTimeout() time.Duration { return time.Second }

func TestIsSSLTarget(t *testing.T) {
	yes := []string{"ssl://example.com", "ssl://example.com:8443", "ssl://example.com|14"}
	no := []string{"https://example.com", "mysql://host/db", "", "sslx://x"}
	for _, u := range yes {
		if !isSSLTarget(u) {
			t.Fatalf("isSSLTarget(%q) = false, want true", u)
		}
	}
	for _, u := range no {
		if isSSLTarget(u) {
			t.Fatalf("isSSLTarget(%q) = true, want false", u)
		}
	}
}

func TestParseSSLTarget(t *testing.T) {
	host, port, days, err := parseSSLTarget("ssl://example.com")
	if err != nil || host != "example.com" || port != "443" {
		t.Fatalf("plain: host=%q port=%q err=%v", host, port, err)
	}
	if days != int(getInt64("SSL_WARN_DAYS", 30)) {
		t.Fatalf("default warn days = %d", days)
	}

	host, port, days, err = parseSSLTarget("ssl://example.com:8443")
	if err != nil || host != "example.com" || port != "8443" {
		t.Fatalf("port: host=%q port=%q err=%v", host, port, err)
	}

	host, port, days, err = parseSSLTarget("ssl://example.com|14")
	if err != nil || host != "example.com" || port != "443" || days != 14 {
		t.Fatalf("warn: host=%q port=%q days=%d err=%v", host, port, days, err)
	}

	if _, _, _, err := parseSSLTarget("ssl://example.com|abc"); err == nil {
		t.Fatal("expected error for non-numeric warn days")
	}
	if _, _, _, err := parseSSLTarget("ssl://|14"); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestCheckSSLBadHost(t *testing.T) {
	cfg := &Config{Timeout: 3 * defaultTimeout()}
	res := CheckSSL(cfg, Target{URL: "ssl://nonexistent.invalid.example:443"})
	if res.OK {
		t.Fatal("expected DOWN for unresolvable host")
	}
	if res.Err == "" {
		t.Fatal("expected error message")
	}
}

func TestCheckSSLPlainHTTPPort(t *testing.T) {
	// TLS handshake against a plain-HTTP server must fail
	cfg := &Config{Timeout: 3 * defaultTimeout()}
	res := CheckSSL(cfg, Target{URL: "ssl://127.0.0.1:1"})
	if res.OK {
		t.Fatal("expected DOWN when nothing listens")
	}
}

func TestFormatResultSSL(t *testing.T) {
	info := &SSLInfo{Subject: "example.com", Issuer: "Test CA", DaysLeft: 42}
	res := CheckResult{
		Target: Target{URL: "ssl://example.com"}, OK: true, SSLInfo: info,
	}
	line := FormatResult(res)
	for _, want := range []string{"ssl", "cert=example.com", "issuer=Test CA", "42d"} {
		if !strings.Contains(line, want) {
			t.Fatalf("ssl line %q missing %q", line, want)
		}
	}
}

func TestCertChainsValid(t *testing.T) {
	if certChainsValid(nil) {
		t.Fatal("nil cert should be invalid")
	}
}
