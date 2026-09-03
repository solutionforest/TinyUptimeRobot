package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"
)

// SSL certificate check. targets.txt syntax:
//
//	ssl://example.com              (port 443, warn < 30 days)
//	ssl://example.com:8443
//	ssl://example.com|14           (warn when cert expires in < 14 days)
//
// Result is DOWN when: handshake fails, cert invalid/untrusted/expired,
// or the cert expires within the warn threshold.

// isSSLTarget reports whether a target is an SSL certificate check.
func isSSLTarget(raw string) bool {
	return strings.HasPrefix(raw, "ssl://")
}

// parseSSLTarget splits ssl://host[:port][|warnDays].
func parseSSLTarget(raw string) (host string, port string, warnDays int, err error) {
	s := strings.TrimPrefix(raw, "ssl://")
	warnDays = getSSLWarnDays()

	if i := strings.Index(s, "|"); i >= 0 {
		days := strings.TrimSpace(s[i+1:])
		s = s[:i]
		n := 0
		for _, c := range days {
			if c < '0' || c > '9' {
				return "", "", 0, fmt.Errorf("invalid warn days %q", days)
			}
			n = n*10 + int(c-'0')
		}
		warnDays = n
	}

	port = "443"
	host = s
	if h, p, splitErr := net.SplitHostPort(s); splitErr == nil {
		host, port = h, p
	}
	if host == "" {
		return "", "", 0, fmt.Errorf("ssl target missing host")
	}
	return host, port, warnDays, nil
}

func getSSLWarnDays() int {
	return int(getInt64("SSL_WARN_DAYS", 30))
}

// CheckSSL connects with TLS, verifies the cert chain, and checks expiry.
func CheckSSL(cfg *Config, t Target) CheckResult {
	ts := time.Now()
	host, port, warnDays, err := parseSSLTarget(t.URL)
	if err != nil {
		return CheckResult{Target: t, OK: false, Err: err.Error(), Timestamp: ts}
	}

	dialer := &net.Dialer{Timeout: cfg.Timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, port), &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return CheckResult{Target: t, OK: false, Err: err.Error(), Duration: time.Since(ts), Timestamp: ts}
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return CheckResult{Target: t, OK: false, Err: "no peer certificate", Timestamp: ts}
	}
	cert := state.PeerCertificates[0]
	days := int(time.Until(cert.NotAfter).Hours() / 24)

	// expired or expiring soon
	if time.Now().After(cert.NotAfter) {
		return CheckResult{Target: t, OK: false,
			Err:      fmt.Sprintf("certificate EXPIRED on %s", cert.NotAfter.Format("2006-01-02")),
			Duration: time.Since(ts), Timestamp: ts}
	}
	if days < warnDays {
		return CheckResult{Target: t, OK: false,
			Err:      fmt.Sprintf("certificate expires in %d day(s) (%s), warn threshold %d", days, cert.NotAfter.Format("2006-01-02"), warnDays),
			Duration: time.Since(ts), Timestamp: ts}
	}

	return CheckResult{
		Target: t, OK: true,
		Duration:  time.Since(ts),
		Timestamp: ts,
		SSLInfo: &SSLInfo{
			Subject:  cert.Subject.CommonName,
			Issuer:   cert.Issuer.CommonName,
			NotAfter: cert.NotAfter,
			DaysLeft: days,
		},
	}
}

// SSLInfo carries certificate details for logging.
type SSLInfo struct {
	Subject  string
	Issuer   string
	NotAfter time.Time
	DaysLeft int
}

// certChainsValid is a helper kept for clarity in tests: tls.DialWithDialer
// already verifies chains via VerifyPeerCertificate default behaviour.
func certChainsValid(cert *x509.Certificate) bool {
	return cert != nil && time.Now().Before(cert.NotAfter) && time.Now().After(cert.NotBefore)
}
