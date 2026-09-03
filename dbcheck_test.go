package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func time_Second() time.Duration { return time.Second }

func TestIsDBTarget(t *testing.T) {
	yes := []string{
		"mysql://user:pass@localhost:3306/app",
		"postgres://u:p@db.host:5432/prod",
		"postgresql://u:p@db.host:5432/prod",
		"sqlite:///tmp/monitor.db",
		"sqlite3://./data/local.db",
		"sqlserver://sa:pass@host:1433?database=master",
		"mssql://sa:pass@host:1433",
	}
	no := []string{
		"https://example.com",
		"http://example.com|200",
		"ftp://example.com",
		"example.com",
		"",
	}
	for _, u := range yes {
		if !isDBTarget(u) {
			t.Fatalf("isDBTarget(%q) = false, want true", u)
		}
	}
	for _, u := range no {
		if isDBTarget(u) {
			t.Fatalf("isDBTarget(%q) = true, want false", u)
		}
	}
}

func TestDBDriverAndDSN(t *testing.T) {
	driver, dsn, err := dbDriverAndDSN("mysql://user:pass@dbhost:3307/mydb?charset=utf8")
	if err != nil {
		t.Fatal(err)
	}
	if driver != "mysql" {
		t.Fatalf("driver = %q", driver)
	}
	want := "user:pass@tcp(dbhost:3307)/mydb?charset=utf8"
	if dsn != want {
		t.Fatalf("dsn = %q, want %q", dsn, want)
	}

	driver, _, err = dbDriverAndDSN("postgres://u:p@h:5432/d")
	if err != nil || driver != "postgres" {
		t.Fatalf("postgres: driver=%q err=%v", driver, err)
	}

	driver, dsn, err = dbDriverAndDSN("sqlite:///tmp/x/y.db")
	if err != nil || driver != "sqlite" || dsn != "/tmp/x/y.db" {
		t.Fatalf("sqlite abs: driver=%q dsn=%q err=%v", driver, dsn, err)
	}

	driver, dsn, err = dbDriverAndDSN("sqlite://./data/local.db")
	if err != nil || driver != "sqlite" || dsn != "./data/local.db" {
		t.Fatalf("sqlite rel: driver=%q dsn=%q err=%v", driver, dsn, err)
	}

	if _, _, err := dbDriverAndDSN("sqlite://"); err == nil {
		t.Fatal("expected error for sqlite without path")
	}
	if _, _, err := dbDriverAndDSN("oracle://x"); err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestCheckDatabaseSQLiteUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	cfg := &Config{Timeout: 5 * time_Second()}

	res := CheckDatabase(cfg, Target{URL: "sqlite://" + path})
	if !res.OK {
		t.Fatalf("expected UP, got %+v", res)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("sqlite file not created: %v", err)
	}
}

func TestCheckDatabaseSQLiteDown(t *testing.T) {
	// a directory cannot be opened as a sqlite database
	dir := t.TempDir()
	cfg := &Config{Timeout: 5 * time_Second()}

	res := CheckDatabase(cfg, Target{URL: "sqlite://" + dir})
	if res.OK {
		t.Fatalf("expected DOWN for invalid sqlite path, got %+v", res)
	}
	if res.Err == "" {
		t.Fatal("expected error message")
	}
}

func TestCheckDatabaseBadScheme(t *testing.T) {
	cfg := &Config{Timeout: time_Second()}
	res := CheckDatabase(cfg, Target{URL: "oracle://user@host/db"})
	if res.OK {
		t.Fatal("expected DOWN for unsupported scheme")
	}
	if res.Err == "" {
		t.Fatal("expected error message")
	}
}

func TestCheckDatabaseMySQLUnreachable(t *testing.T) {
	// port 1 on localhost — nothing listens, should fail fast with DOWN
	cfg := &Config{Timeout: 3 * time_Second()}
	res := CheckDatabase(cfg, Target{URL: "mysql://u:p@127.0.0.1:1/nope"})
	if res.OK {
		t.Fatal("expected DOWN for unreachable mysql")
	}
}

func TestCheckDatabasePostgresUnreachable(t *testing.T) {
	cfg := &Config{Timeout: 3 * time_Second()}
	res := CheckDatabase(cfg, Target{URL: "postgres://u:p@127.0.0.1:1/nope"})
	if res.OK {
		t.Fatal("expected DOWN for unreachable postgres")
	}
}

func TestEnsureSQLiteParent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "a.db")
	ensureSQLiteParent("sqlite://" + p)
	if _, err := os.Stat(filepath.Join(dir, "sub")); err != nil {
		t.Fatalf("parent dir not created: %v", err)
	}
}
