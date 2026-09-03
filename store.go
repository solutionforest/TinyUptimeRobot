package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// LogStore appends check results to a plain txt file (CRLF line endings)
// and rotates (truncates) when it grows past the max size.
type LogStore struct {
	mu    sync.Mutex
	path  string
	maxKB int64
}

func NewLogStore(path string, maxKB int64) *LogStore {
	return &LogStore{path: path, maxKB: maxKB}
}

// Append writes one line, CRLF terminated.
func (s *LogStore) Append(line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\r\n"); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return s.maybeRotate()
}

// maybeRotate cleans up the txt file: when bigger than maxKB, keep only the
// newest keepBytes worth of lines.
func (s *LogStore) maybeRotate() error {
	st, err := os.Stat(s.path)
	if err != nil {
		return err
	}
	if st.Size() <= s.maxKB*1024 {
		return nil
	}

	const keepBytes int64 = 64 * 1024 // keep last ~64KB
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if int64(len(data)) > keepBytes {
		data = data[int64(len(data))-keepBytes:]
		// start at a line boundary
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' {
				data = data[i+1:]
				break
			}
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// FormatResult makes the txt line for a result.
func FormatResult(r CheckResult) string {
	status := "UP"
	if !r.OK {
		status = "DOWN"
	}
	kind := "http"
	if isDBTarget(r.Target.URL) {
		kind = "db"
	} else if isSSLTarget(r.Target.URL) {
		kind = "ssl"
	}
	line := fmt.Sprintf("%s | %s | %s | %s", r.Timestamp.Format(time.RFC3339), status, r.Target.URL, kind)
	if r.Status != 0 {
		line += fmt.Sprintf(" | http=%d", r.Status)
	}
	if r.Duration > 0 {
		line += fmt.Sprintf(" | %dms", r.Duration.Milliseconds())
	}
	if r.Err != "" {
		line += " | err=" + r.Err
	}
	if r.SSLInfo != nil {
		line += fmt.Sprintf(" | cert=%s | issuer=%s | expires=%s (%dd)",
			r.SSLInfo.Subject, r.SSLInfo.Issuer,
			r.SSLInfo.NotAfter.Format("2006-01-02"), r.SSLInfo.DaysLeft)
	}
	return line
}
