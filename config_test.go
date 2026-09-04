package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetStr(t *testing.T) {
	os.Setenv("TEST_GETSTR", "hello")
	defer os.Unsetenv("TEST_GETSTR")
	if got := getStr("TEST_GETSTR", "def"); got != "hello" {
		t.Fatalf("getStr = %q, want hello", got)
	}
	if got := getStr("TEST_GETSTR_MISSING", "def"); got != "def" {
		t.Fatalf("getStr missing = %q, want def", got)
	}
}

func TestGetDur(t *testing.T) {
	cases := []struct {
		env  string
		want time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"45", 45 * time.Second}, // plain seconds fallback
		{"bogus", 60 * time.Second},
	}
	for _, c := range cases {
		os.Setenv("TEST_GETDUR", c.env)
		if got := getDur("TEST_GETDUR", 60*time.Second); got != c.want {
			t.Fatalf("getDur(%q) = %v, want %v", c.env, got, c.want)
		}
	}
	os.Unsetenv("TEST_GETDUR")
}

func TestGetInt64AndBool(t *testing.T) {
	os.Setenv("TEST_INT", "256")
	if got := getInt64("TEST_INT", 1); got != 256 {
		t.Fatalf("getInt64 = %d, want 256", got)
	}
	os.Setenv("TEST_INT", "x")
	if got := getInt64("TEST_INT", 1); got != 1 {
		t.Fatalf("getInt64 bad = %d, want 1", got)
	}
	os.Unsetenv("TEST_INT")

	for _, v := range []string{"1", "true", "TRUE", "yes", "y"} {
		os.Setenv("TEST_BOOL", v)
		if !getBool("TEST_BOOL", false) {
			t.Fatalf("getBool(%q) = false, want true", v)
		}
	}
	os.Setenv("TEST_BOOL", "no")
	if getBool("TEST_BOOL", true) {
		t.Fatal("getBool(no) = true, want false")
	}
	os.Unsetenv("TEST_BOOL")
}

func TestLoadTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.txt")
	content := "# comment line\n\nhttps://example.com\nhttps://google.com | 200\nhttps://api.example.com | 201 | api@example.com, backup@example.com\n|999\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	targets, err := LoadTargets(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 3 {
		t.Fatalf("got %d targets, want 3: %+v", len(targets), targets)
	}
	if targets[0].URL != "https://example.com" || targets[0].ExpectStatus != 0 {
		t.Fatalf("target[0] = %+v", targets[0])
	}
	if targets[1].URL != "https://google.com" || targets[1].ExpectStatus != 200 {
		t.Fatalf("target[1] = %+v", targets[1])
	}
	if targets[2].URL != "https://api.example.com" || targets[2].ExpectStatus != 201 || targets[2].NotifyTo != "api@example.com, backup@example.com" {
		t.Fatalf("target[2] = %+v", targets[2])
	}
}

func TestLoadTargetsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte("# nothing\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTargets(path); err == nil {
		t.Fatal("expected error for empty targets file")
	}
}

func TestLoadTargetsMissingFile(t *testing.T) {
	if _, err := LoadTargets("/nonexistent/nope.txt"); err == nil {
		t.Fatal("expected error for missing targets file")
	}
}
