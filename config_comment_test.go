package main

import (
	"os"
	"testing"
	"time"
)

// Reproduce: .env.example ships inline comments. docker run --env-file does
// NOT strip them, so the app receives the raw value including " # comment".
func TestGetDurWithInlineComment(t *testing.T) {
	os.Setenv("TEST_IC", "30s        # cron-like frequency (e.g. 30s, 5m, 1h)")
	got := getDur("TEST_IC", 60*time.Second)
	if got != 30*time.Second {
		t.Fatalf("getDur with inline comment = %v, want 30s (silently fell back to default!)", got)
	}
	os.Unsetenv("TEST_IC")
}

func TestGetBoolWithInlineComment(t *testing.T) {
	os.Setenv("TEST_ICB", "true          # only print alerts")
	if !getBool("TEST_ICB", false) {
		t.Fatal("getBool with inline comment silently ignored true")
	}
	os.Unsetenv("TEST_ICB")
}

func TestGetInt64WithInlineComment(t *testing.T) {
	os.Setenv("TEST_ICI", "1024            # rotate threshold")
	if got := getInt64("TEST_ICI", 512); got != 1024 {
		t.Fatalf("getInt64 with inline comment = %d, want 1024", got)
	}
	os.Unsetenv("TEST_ICI")
}
