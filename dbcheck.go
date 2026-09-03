package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "modernc.org/sqlite"
)

// Supported DB schemes in targets.txt:
//
//	mysql://user:pass@host:3306/dbname
//	postgres://user:pass@host:5432/dbname   (or postgresql://)
//	sqlite:///path/to/file.db               (or sqlite://file.db)
//	sqlserver://user:pass@host:1433?database=master
//
// A simple "SELECT 1" probe is executed; success = UP.

func dbDriverAndDSN(raw string) (driver, dsn string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("parse db url: %w", err)
	}

	switch u.Scheme {
	case "mysql":
		// mysql DSN: user:pass@tcp(host:port)/dbname
		host := u.Host
		if host == "" {
			host = "127.0.0.1:3306"
		}
		dsn := ""
		if u.User != nil {
			dsn = u.User.String() + "@"
		}
		dsn += "tcp(" + host + ")" + u.Path
		if q := u.Query(); len(q) > 0 {
			dsn += "?" + u.RawQuery
		}
		return "mysql", dsn, nil

	case "postgres", "postgresql":
		// pq accepts URL form directly
		return "postgres", raw, nil

	case "sqlite", "sqlite3":
		// sqlite:///path/file.db -> path = /path/file.db
		// sqlite://file.db       -> host = file.db
		path := u.Host + u.Path
		if path == "" {
			return "", "", fmt.Errorf("sqlite url missing file path")
		}
		if u.Query().Get("mode") == "memory" {
			path = ":memory:"
		}
		return "sqlite", path, nil

	case "sqlserver", "mssql":
		// go-mssqldb accepts URL form directly
		return "sqlserver", raw, nil

	default:
		return "", "", fmt.Errorf("unsupported db scheme %q", u.Scheme)
	}
}

// CheckDatabase opens a short-lived connection and runs SELECT 1.
func CheckDatabase(cfg *Config, t Target) CheckResult {
	ts := time.Now()

	driver, dsn, err := dbDriverAndDSN(t.URL)
	if err != nil {
		return CheckResult{Target: t, OK: false, Err: err.Error(), Timestamp: ts}
	}

	timeout := cfg.Timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return CheckResult{Target: t, OK: false, Err: err.Error(), Timestamp: ts}
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	start := time.Now()
	if _, err := db.QueryContext(ctx, "SELECT 1"); err != nil {
		return CheckResult{Target: t, OK: false, Err: err.Error(), Duration: time.Since(start), Timestamp: ts}
	}
	return CheckResult{Target: t, OK: true, Duration: time.Since(start), Timestamp: ts}
}

// isDBTarget reports whether a target URL is a database check.
func isDBTarget(raw string) bool {
	scheme := raw
	if i := strings.Index(raw, "://"); i >= 0 {
		scheme = raw[:i]
	}
	switch scheme {
	case "mysql", "postgres", "postgresql", "sqlite", "sqlite3", "sqlserver", "mssql":
		return true
	}
	return false
}

// ensureSQLiteParent creates parent dirs for sqlite files so first run works.
func ensureSQLiteParent(raw string) {
	if u, err := url.Parse(raw); err == nil && (u.Scheme == "sqlite" || u.Scheme == "sqlite3") {
		if p := u.Host + u.Path; p != "" && p != ":memory:" {
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
		}
	}
}
