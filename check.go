package main

import (
	"fmt"
	"net/http"
	"time"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	},
}

// CheckResult is the outcome of one check.
type CheckResult struct {
	Target    Target
	OK        bool
	Status    int
	Duration  time.Duration
	Err       string
	Timestamp time.Time
	SSLInfo   *SSLInfo // set for ssl:// checks
}

// CheckURL performs the HTTP check.
func CheckURL(cfg *Config, t Target) CheckResult {
	client := &http.Client{Timeout: cfg.Timeout}
	client.CheckRedirect = httpClient.CheckRedirect

	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, t.URL, nil)
	if err != nil {
		return CheckResult{Target: t, OK: false, Err: err.Error(), Timestamp: time.Now()}
	}
	req.Header.Set("User-Agent", "TinyUptimeRobot/1.0")

	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		return CheckResult{Target: t, OK: false, Err: err.Error(), Duration: dur, Timestamp: time.Now()}
	}
	defer resp.Body.Close()

	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	if t.ExpectStatus != 0 {
		ok = resp.StatusCode == t.ExpectStatus
	}
	return CheckResult{
		Target: t, OK: ok, Status: resp.StatusCode,
		Duration: dur, Timestamp: time.Now(),
	}
}
