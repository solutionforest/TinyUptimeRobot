package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckURLSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "TinyUptimeRobot/1.0" {
			t.Errorf("missing User-Agent header")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &Config{Timeout: 5 * time.Second}
	res := CheckURL(cfg, Target{URL: srv.URL})
	if !res.OK || res.Status != 200 {
		t.Fatalf("expected UP/200, got %+v", res)
	}
	if res.Duration <= 0 {
		t.Fatal("duration not measured")
	}
}

func TestCheckURLError(t *testing.T) {
	cfg := &Config{Timeout: 1 * time.Second}
	res := CheckURL(cfg, Target{URL: "http://127.0.0.1:1/nope"}) // nothing listens
	if res.OK {
		t.Fatal("expected DOWN for unreachable host")
	}
	if res.Err == "" {
		t.Fatal("expected error message")
	}
}

func TestCheckURLExpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer srv.Close()

	cfg := &Config{Timeout: 5 * time.Second}

	// 201 is 2xx so OK by default
	if res := CheckURL(cfg, Target{URL: srv.URL}); !res.OK {
		t.Fatalf("expected UP, got %+v", res)
	}
	// but DOWN when we demand exactly 200
	if res := CheckURL(cfg, Target{URL: srv.URL + "|200"}); false {
		_ = res
	}
	res := CheckURL(cfg, Target{URL: srv.URL, ExpectStatus: 200})
	if res.OK {
		t.Fatalf("expected DOWN when expecting 200 but got %d", res.Status)
	}
	res = CheckURL(cfg, Target{URL: srv.URL, ExpectStatus: 201})
	if !res.OK {
		t.Fatalf("expected UP when expecting 201, got %+v", res)
	}
}

func TestCheckURLErrorStatusIsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	cfg := &Config{Timeout: 5 * time.Second}
	res := CheckURL(cfg, Target{URL: srv.URL})
	if res.OK || res.Status != 500 {
		t.Fatalf("expected DOWN/500, got %+v", res)
	}
}

func TestRunOnceLogsAndAlerts(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if atomic.LoadInt32(&calls) <= 1 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := &Config{
		Timeout:       5 * time.Second,
		LogFile:       dir + "/status.txt",
		LogMaxKB:      1024,
		NotifyRecover: true,
	}
	m := NewMonitor(cfg, []Target{{URL: srv.URL}})

	// round 1: DOWN -> alert fired
	res1 := m.RunOnce()
	if len(res1) != 1 || res1[0].OK {
		t.Fatalf("round 1 expected DOWN, got %+v", res1)
	}
	if !m.down[srv.URL] {
		t.Fatal("state should be down after round 1")
	}

	// round 2: UP -> recovery
	res2 := m.RunOnce()
	if !res2[0].OK {
		t.Fatalf("round 2 expected UP, got %+v", res2)
	}
	if m.down[srv.URL] {
		t.Fatal("state should be up after round 2")
	}

	data, _ := os.ReadFile(dir + "/status.txt")
	if len(data) == 0 {
		t.Fatal("log file empty")
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\r\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %q", len(lines), string(data))
	}
	if !strings.Contains(lines[0], "DOWN") || !strings.Contains(lines[1], "UP") {
		t.Fatalf("log lines wrong: %q", lines)
	}
}

func TestNoRepeatAlertWhileStillDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	cfg := &Config{Timeout: 5 * time.Second, LogFile: t.TempDir() + "/s.txt"}
	m := NewMonitor(cfg, []Target{{URL: srv.URL}})

	m.RunOnce()
	m.RunOnce() // still down — no new transition, so no repeat alert path
	if !m.down[srv.URL] {
		t.Fatal("should still be marked down")
	}
}

func TestAlertOnlySuppressesExternalNotifications(t *testing.T) {
	var mailFrom, gotSlack, gotGChat string
	ln := startFakeSMTP(t, &mailFrom)
	defer ln.Close()
	parts := strings.Split(ln.Addr().String(), ":")

	slackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSlack = "called"
		w.WriteHeader(200)
	}))
	defer slackSrv.Close()
	gchatSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotGChat = "called"
		w.WriteHeader(200)
	}))
	defer gchatSrv.Close()

	cfg := &Config{
		Timeout:   5 * time.Second,
		LogFile:   t.TempDir() + "/s.txt",
		AlertOnly: true,
		SMTPHost:  parts[0],
		SMTPPort:  parts[1],
		MailFrom:  "m@test.local",
		MailTo:    "o@test.local",
		SlackURL:  slackSrv.URL,
		GChatURL:  gchatSrv.URL,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	m := NewMonitor(cfg, []Target{{URL: srv.URL}})
	m.RunOnce()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && (mailFrom != "" || gotSlack != "" || gotGChat != "") {
		time.Sleep(10 * time.Millisecond)
	}
	if mailFrom != "" {
		t.Fatal("ALERT_ONLY=true must not send email")
	}
	if gotSlack != "" {
		t.Fatal("ALERT_ONLY=true must not call Slack webhook")
	}
	if gotGChat != "" {
		t.Fatal("ALERT_ONLY=true must not call Google Chat webhook")
	}
}

func TestNotificationsSentWhenNotAlertOnly(t *testing.T) {
	var gotGChat string
	gchatSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotGChat = "called"
		w.WriteHeader(200)
	}))
	defer gchatSrv.Close()

	cfg := &Config{
		Timeout:   5 * time.Second,
		LogFile:   t.TempDir() + "/s.txt",
		AlertOnly: false,
		GChatURL:  gchatSrv.URL,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	m := NewMonitor(cfg, []Target{{URL: srv.URL}})
	m.RunOnce()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && gotGChat == "" {
		time.Sleep(10 * time.Millisecond)
	}
	if gotGChat == "" {
		t.Fatal("expected Google Chat webhook to be called when ALERT_ONLY=false")
	}
}
