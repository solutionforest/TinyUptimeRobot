package main

import (
	"bufio"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// Target is one thing we monitor.
type Target struct {
	URL          string
	ExpectStatus int // 0 = any 2xx/3xx ok
}

// Config holds everything loaded from env / targets file.
type Config struct {
	Targets       []Target
	Interval      time.Duration // check frequency (cron-like via loop)
	LogFile       string
	LogMaxKB      int64 // cleanup: max size of txt log before truncate
	LogKeepMB     int64 // keep last N MB when rotating (we simply truncate, keep light)
	SMTPHost      string
	SMTPPort      string
	SMTPUser      string
	SMTPPass      string
	MailFrom      string
	MailTo        string // comma separated
	SlackURL      string // webhook
	GChatURL      string // webhook
	Timeout       time.Duration
	NotifyRecover bool
	AlertOnly     bool // print only alert lines to console; suppresses external notifications
}

// LoadConfig builds config from environment.
func LoadConfig() *Config {
	c := &Config{
		Interval:      getDur("CHECK_INTERVAL", 60*time.Second),
		LogFile:       getStr("LOG_FILE", "status.txt"),
		LogMaxKB:      getInt64("LOG_MAX_KB", 512),
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      getStr("SMTP_PORT", "587"),
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		MailFrom:      os.Getenv("MAIL_FROM"),
		MailTo:        os.Getenv("MAIL_TO"),
		SlackURL:      getStr("SLACK_WEBHOOK_URL", ""),
		GChatURL:      getStr("GCHAT_WEBHOOK_URL", ""),
		Timeout:       getDur("HTTP_TIMEOUT", 10*time.Second),
		NotifyRecover: getBool("NOTIFY_RECOVER", true),
		AlertOnly:     getBool("ALERT_ONLY", false),
	}
	return c
}

func getStr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getDur(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		// allow plain seconds
		if n, err2 := strconv.Atoi(v); err2 == nil {
			return time.Duration(n) * time.Second
		}
		return def
	}
	return d
}

func getInt64(k string, def int64) int64 {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func getBool(k string, def bool) bool {
	v := strings.ToLower(os.Getenv(k))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes" || v == "y"
}

// LoadTargets reads targets.txt: one URL per line, optional "|expected_status".
func LoadTargets(path string) ([]Target, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var out []Target
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		t := Target{URL: line, ExpectStatus: 0}
		if i := strings.Index(line, "|"); i >= 0 {
			t.URL = strings.TrimSpace(line[:i])
			if n, err := strconv.Atoi(strings.TrimSpace(line[i+1:])); err == nil {
				t.ExpectStatus = n
			}
		}
		if t.URL == "" {
			continue
		}
		out = append(out, t)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no targets found in %s", path)
	}
	return out, nil
}

// SendMail sends a plain email via SMTP (STARTTLS on 587, SSL on 465 not handled — keep simple).
func SendMail(cfg *Config, subject, body string) error {
	if cfg.SMTPHost == "" || cfg.MailTo == "" || cfg.MailFrom == "" {
		return nil // mail not configured
	}
	addr := cfg.SMTPHost + ":" + cfg.SMTPPort
	from := cfg.MailFrom
	if cfg.SMTPUser != "" {
		from = cfg.SMTPUser
	}

	msg := strings.Join([]string{
		"From: " + cfg.MailFrom,
		"To: " + cfg.MailTo,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		host, _, _ := strings.Cut(cfg.SMTPHost, ":")
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, host)
	}

	recipients := strings.Split(cfg.MailTo, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}
	return smtp.SendMail(addr, auth, from, recipients, []byte(msg))
}

// SendSlack posts to a Slack incoming webhook.
func SendSlack(cfg *Config, text string) error {
	if cfg.SlackURL == "" {
		return nil
	}
	payload := `{"text":` + jsonQuote(text) + `}`
	return postJSON(cfg.SlackURL, payload)
}

// SendGChat posts to a Google Chat webhook.
func SendGChat(cfg *Config, text string) error {
	if cfg.GChatURL == "" {
		return nil
	}
	payload := `{"text":` + jsonQuote(text) + `}`
	return postJSON(cfg.GChatURL, payload)
}

func postJSON(url, payload string) error {
	resp, err := httpClient.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

// jsonQuote is a tiny JSON string escaper.
func jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
