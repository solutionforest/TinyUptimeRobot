package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendWritesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.txt")
	s := NewLogStore(path, 1024)

	if err := s.Append("line-one"); err != nil {
		t.Fatal(err)
	}
	if err := s.Append("line-two"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "line-two\r\n") {
		t.Fatalf("file does not end with CRLF line: %q", string(data))
	}
	if got := strings.Count(string(data), "\r\n"); got != 2 {
		t.Fatalf("got %d CRLF lines, want 2", got)
	}
	if strings.Contains(string(data), "\n") && strings.Contains(strings.ReplaceAll(string(data), "\r\n", ""), "\n") {
		t.Fatalf("found bare LF not part of CRLF: %q", string(data))
	}
}

func TestAppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "status.txt")
	s := NewLogStore(path, 1024)
	// parent dir doesn't exist -> expect error, not panic
	if err := s.Append("x"); err == nil {
		t.Fatal("expected error for missing parent dir")
	}
}

func TestRotateKeepsTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.txt")
	s := NewLogStore(path, 1) // 1KB max -> rotate quickly

	// write ~50 lines of ~100 bytes = ~5KB
	for i := 0; i < 50; i++ {
		line := "2026-01-01T00:00:00Z | UP | https://example.com/" + strings.Repeat("a", 50)
		if err := s.Append(line); err != nil {
			t.Fatal(err)
		}
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() > 1024+128*1024 {
		t.Fatalf("file never rotated: %d bytes", st.Size())
	}

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\r\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		t.Fatalf("bad tail content: %q", string(data))
	}
	// rotated content should be much smaller than total written (~5KB)
	if int64(len(data)) > 128*1024 {
		t.Fatalf("rotation kept too much: %d bytes", len(data))
	}
}

func TestNoRotateUnderLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.txt")
	s := NewLogStore(path, 1024) // 1MB limit

	if err := s.Append("tiny"); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	before := st.Size()

	if err := s.Append("tiny2"); err != nil {
		t.Fatal(err)
	}
	st, _ = os.Stat(path)
	if st.Size() <= before {
		t.Fatal("append did not grow file")
	}
}

func TestFormatResult(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	up := CheckResult{
		Target: Target{URL: "https://example.com"}, OK: true, Status: 200,
		Duration: 250 * time.Millisecond, Timestamp: ts,
	}
	line := FormatResult(up)
	for _, want := range []string{"UP", "https://example.com", "http=200", "250ms"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}

	down := CheckResult{
		Target: Target{URL: "https://down.io"}, OK: false, Status: 500,
		Err: "boom", Timestamp: ts,
	}
	line = FormatResult(down)
	for _, want := range []string{"DOWN", "https://down.io", "http=500", "err=boom"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
}
