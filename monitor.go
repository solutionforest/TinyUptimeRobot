package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Monitor runs the check loop and tracks state to send alerts on
// DOWN transitions and (optionally) recovery.
type Monitor struct {
	cfg     *Config
	targets []Target
	store   *LogStore
	alertMu sync.Mutex
	down    map[string]bool // url -> currently down
}

func NewMonitor(cfg *Config, targets []Target) *Monitor {
	return &Monitor{
		cfg:     cfg,
		targets: targets,
		store:   NewLogStore(cfg.LogFile, cfg.LogMaxKB),
		down:    make(map[string]bool),
	}
}

// RunOnce checks all targets once (parallel), logs, and notifies on changes.
func (m *Monitor) RunOnce() []CheckResult {
	var wg sync.WaitGroup
	results := make([]CheckResult, len(m.targets))
	for i, t := range m.targets {
		wg.Add(1)
		go func(i int, t Target) {
			defer wg.Done()
			if isDBTarget(t.URL) {
				results[i] = CheckDatabase(m.cfg, t)
			} else if isSSLTarget(t.URL) {
				results[i] = CheckSSL(m.cfg, t)
			} else {
				results[i] = CheckURL(m.cfg, t)
			}
		}(i, t)
	}
	wg.Wait()

	for _, r := range results {
		line := FormatResult(r)
		if err := m.store.Append(line); err != nil {
			fmt.Fprintf(os.Stderr, "log write failed: %v\n", err)
		}
		if !m.cfg.AlertOnly || !r.OK {
			fmt.Println(line)
		}
		m.handleStateChange(r)
	}
	return results
}

// Run loops forever (acts like cron with CHECK_INTERVAL frequency).
func (m *Monitor) Run(ctx context.Context) {
	for {
		m.RunOnce()
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.cfg.Interval):
		}
	}
}

func (m *Monitor) handleStateChange(r CheckResult) {
	m.alertMu.Lock()
	wasDown := m.down[r.Target.URL]
	m.down[r.Target.URL] = !r.OK
	m.alertMu.Unlock()

	var msg string
	switch {
	case !r.OK && !wasDown:
		msg = fmt.Sprintf("🔴 DOWN: %s\nstatus=%d err=%s at %s",
			r.Target.URL, r.Status, r.Err, r.Timestamp.Format(time.RFC3339))
	case r.OK && wasDown && m.cfg.NotifyRecover:
		msg = fmt.Sprintf("🟢 RECOVERED: %s at %s",
			r.Target.URL, r.Timestamp.Format(time.RFC3339))
	default:
		return
	}

	fmt.Println(">>> " + strings.ReplaceAll(msg, "\n", " "))
	m.notify(msg)
}

func (m *Monitor) notify(text string) {
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); _ = SendMail(m.cfg, "Uptime Alert", text) }()
	go func() { defer wg.Done(); _ = SendSlack(m.cfg, text) }()
	go func() { defer wg.Done(); _ = SendGChat(m.cfg, text) }()
	wg.Wait()
}

// Tail prints the last n lines of the log (CLI helper).
func Tail(path string, n int) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Println("no log yet:", err)
		return
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, l := range lines {
		fmt.Println(l)
	}
}
