package main

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startFakeSMTP spins up a minimal SMTP-like server that speaks just enough
// of the protocol for net/smtp.SendMail and captures the MAIL FROM envelope.
func startFakeSMTP(t *testing.T, mailFrom *string, mailRecipients *[]string) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleFakeSMTP(conn, mailFrom, mailRecipients)
		}
	}()
	return ln
}

func handleFakeSMTP(conn net.Conn, mailFrom *string, mailRecipients *[]string) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	writeLine := func(s string) {
		w.WriteString(s + "\r\n")
		w.Flush()
	}

	writeLine("220 fake ESMTP")
	dataMode := false
	var body strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(line)

		switch {
		case dataMode:
			if line == "." {
				dataMode = false
				writeLine("250 OK queued")
				body.Reset()
			} else {
				body.WriteString(line + "\n")
			}
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			writeLine("250-fake")
			writeLine("250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "AUTH"):
			writeLine("235 ok")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			*mailFrom = strings.Trim(strings.TrimPrefix(line[len("MAIL FROM:"):], " "), "<>")
			writeLine("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			if mailRecipients != nil {
				*mailRecipients = append(*mailRecipients, strings.Trim(strings.TrimPrefix(line[len("RCPT TO:"):], " "), "<>"))
			}
			writeLine("250 OK")
		case strings.HasPrefix(cmd, "DATA"):
			writeLine("354 go")
			dataMode = true
		case strings.HasPrefix(cmd, "QUIT"):
			writeLine("221 bye")
			return
		default:
			writeLine("250 OK")
		}
	}
}

func TestJsonQuote(t *testing.T) {
	got := jsonQuote("he said \"hi\"\nline2\tend\\path")
	want := `"he said \"hi\"\nline2\tend\\path"`
	if got != want {
		t.Fatalf("jsonQuote = %s, want %s", got, want)
	}
	var s string
	if err := json.Unmarshal([]byte(got), &s); err != nil {
		t.Fatalf("jsonQuote output is not valid JSON: %v", err)
	}
}

func TestSendSlack(t *testing.T) {
	var received map[string]any
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &Config{}
	if err := SendSlack(cfg, srv.URL, "🔴 DOWN: https://x.com"); err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" {
		t.Fatalf("content type = %q", contentType)
	}
	if received["text"] != "🔴 DOWN: https://x.com" {
		t.Fatalf("payload = %+v", received)
	}
}

func TestSendSlackNotConfigured(t *testing.T) {
	if err := SendSlack(&Config{}, "", "hi"); err != nil {
		t.Fatal("expected no-op when SlackURL empty")
	}
}

func TestSendSlackServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	if err := SendSlack(&Config{}, srv.URL, "hi"); err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestSendGChat(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &Config{}
	if err := SendGChat(cfg, srv.URL, "🟢 RECOVERED: https://x.com"); err != nil {
		t.Fatal(err)
	}
	if received["text"] != "🟢 RECOVERED: https://x.com" {
		t.Fatalf("payload = %+v", received)
	}
}

func TestSendMailNotConfigured(t *testing.T) {
	if err := SendMail(&Config{}, "", "subj", "body"); err != nil {
		t.Fatal("expected no-op when SMTP not configured")
	}
}

func TestSendMailViaFakeSMTP(t *testing.T) {
	var mailFrom, mailData string
	var recipients []string
	ln := startFakeSMTP(t, &mailFrom, &recipients)
	defer ln.Close()
	parts := strings.Split(ln.Addr().String(), ":")

	cfg := &Config{
		SMTPHost: parts[0],
		SMTPPort: parts[1],
		MailFrom: "monitor@test.local",
		MailTo:   "ops@test.local, backup@test.local",
		SMTPUser: "",
		SMTPPass: "",
	}
	if err := SendMail(cfg, "", "Uptime Alert", "site is down"); err != nil {
		t.Fatal(err)
	}

	// fetch captured data
	data := func() string { return mailData }
	_ = data
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if mailFrom != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mailFrom != "monitor@test.local" {
		t.Fatalf("MAIL FROM = %q, want monitor@test.local", mailFrom)
	}
	if got, want := strings.Join(recipients, ","), "ops@test.local,backup@test.local"; got != want {
		t.Fatalf("RCPT TO = %q, want %q", got, want)
	}
}

func TestNotifyUsesPerTargetMailList(t *testing.T) {
	var mailFrom string
	var recipients []string
	ln := startFakeSMTP(t, &mailFrom, &recipients)
	defer ln.Close()
	parts := strings.Split(ln.Addr().String(), ":")

	cfg := &Config{
		SMTPHost: parts[0],
		SMTPPort: parts[1],
		MailFrom: "monitor@test.local",
		MailTo:   "global@test.local",
	}
	m := NewMonitor(cfg, nil)
	m.notify(Target{URL: "https://api.example.com", NotifyTo: "api@test.local, backup@test.local"}, "site is down")

	if got, want := strings.Join(recipients, ","), "api@test.local,backup@test.local"; got != want {
		t.Fatalf("RCPT TO = %q, want %q", got, want)
	}
}

func TestNotifyFallsBackToGlobalWhenNoOverride(t *testing.T) {
	var mailFrom string
	var recipients []string
	ln := startFakeSMTP(t, &mailFrom, &recipients)
	defer ln.Close()
	parts := strings.Split(ln.Addr().String(), ":")

	cfg := &Config{
		SMTPHost: parts[0],
		SMTPPort: parts[1],
		MailFrom: "monitor@test.local",
		MailTo:   "global@test.local",
	}
	m := NewMonitor(cfg, nil)
	m.notify(Target{URL: "https://api.example.com"}, "site is down")

	if got, want := strings.Join(recipients, ","), "global@test.local"; got != want {
		t.Fatalf("RCPT TO = %q, want %q", got, want)
	}
}

func TestNotifyPerTargetSlackOverride(t *testing.T) {
	var globalHit, targetHit string
	globalSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalHit = "called"
		w.WriteHeader(200)
	}))
	defer globalSrv.Close()
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit = "called"
		w.WriteHeader(200)
	}))
	defer targetSrv.Close()

	cfg := &Config{SlackURL: globalSrv.URL}
	m := NewMonitor(cfg, nil)
	m.notify(Target{URL: "https://x.com", NotifyTo: "slack:" + targetSrv.URL}, "site is down")

	if globalHit != "" {
		t.Fatal("global slack webhook must not be called when target overrides it")
	}
	if targetHit == "" {
		t.Fatal("per-target slack webhook not called")
	}
}

func TestNotifyPerTargetGChatOverride(t *testing.T) {
	var globalHit, targetHit string
	globalSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalHit = "called"
		w.WriteHeader(200)
	}))
	defer globalSrv.Close()
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit = "called"
		w.WriteHeader(200)
	}))
	defer targetSrv.Close()

	cfg := &Config{GChatURL: globalSrv.URL}
	m := NewMonitor(cfg, nil)
	m.notify(Target{URL: "https://x.com", NotifyTo: "gchat:" + targetSrv.URL}, "site is down")

	if globalHit != "" {
		t.Fatal("global gchat webhook must not be called when target overrides it")
	}
	if targetHit == "" {
		t.Fatal("per-target gchat webhook not called")
	}
}

func TestNotifyPerTargetAllChannelsOverride(t *testing.T) {
	var mailFrom string
	var recipients []string
	ln := startFakeSMTP(t, &mailFrom, &recipients)
	defer ln.Close()
	parts := strings.Split(ln.Addr().String(), ":")

	var slackHit, gchatHit string
	slackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slackHit = "called"
		w.WriteHeader(200)
	}))
	defer slackSrv.Close()
	gchatSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gchatHit = "called"
		w.WriteHeader(200)
	}))
	defer gchatSrv.Close()

	cfg := &Config{
		SMTPHost: parts[0],
		SMTPPort: parts[1],
		MailFrom: "monitor@test.local",
		MailTo:   "global@test.local",
	}
	m := NewMonitor(cfg, nil)
	spec := "api@test.local, slack:" + slackSrv.URL + ", gchat:" + gchatSrv.URL
	m.notify(Target{URL: "https://x.com", NotifyTo: spec}, "site is down")

	if got, want := strings.Join(recipients, ","), "api@test.local"; got != want {
		t.Fatalf("RCPT TO = %q, want %q", got, want)
	}
	if slackHit == "" || gchatHit == "" {
		t.Fatalf("slack called=%q gchat called=%q, want both", slackHit, gchatHit)
	}
}

func TestParseNotifySpec(t *testing.T) {
	cases := []struct {
		spec string
		want NotifyOverrides
	}{
		{"", NotifyOverrides{}},
		{"a@x.com", NotifyOverrides{Mail: "a@x.com"}},
		{"a@x.com, b@y.com", NotifyOverrides{Mail: "a@x.com, b@y.com"}},
		{"email:a@x.com", NotifyOverrides{Mail: "a@x.com"}},
		{"email: a@x.com", NotifyOverrides{Mail: "a@x.com"}},
		{"email:a@x.com, b@y.com", NotifyOverrides{Mail: "a@x.com, b@y.com"}},
		{"slack:https://s.example", NotifyOverrides{Slack: "https://s.example"}},
		{"gchat:https://g.example", NotifyOverrides{GChat: "https://g.example"}},
		{
			"a@x.com, slack:https://s.example, gchat:https://g.example",
			NotifyOverrides{Mail: "a@x.com", Slack: "https://s.example", GChat: "https://g.example"},
		},
		{
			"email:a@x.com, b@y.com, slack:https://s.example, gchat:https://g.example",
			NotifyOverrides{Mail: "a@x.com, b@y.com", Slack: "https://s.example", GChat: "https://g.example"},
		},
	}
	for _, c := range cases {
		got := parseNotifySpec(c.spec)
		if got != c.want {
			t.Fatalf("parseNotifySpec(%q) = %+v, want %+v", c.spec, got, c.want)
		}
	}
}
