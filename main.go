package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	targetsFile := flag.String("targets", envOr("TARGETS_FILE", "targets.txt"), "file with URLs to monitor")
	tail := flag.Int("tail", 0, "print last N log lines and exit")
	once := flag.Bool("once", false, "run one check round and exit")
	setup := flag.Bool("setup", false, "interactive TUI to configure targets & notifications")
	flag.Parse()

	cfg := LoadConfig()
	cfg.Timeout = timeoutFromEnv()

	if *setup {
		saved, err := RunSetup(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "setup error:", err)
			os.Exit(1)
		}
		if saved {
			fmt.Println("settings saved (targets.txt / .env updated)")
		}
		return
	}

	if *tail > 0 {
		Tail(cfg.LogFile, *tail)
		return
	}

	targets, err := LoadTargets(*targetsFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	for _, t := range targets {
		if isDBTarget(t.URL) {
			ensureSQLiteParent(t.URL)
		}
	}

	m := NewMonitor(cfg, targets)

	fmt.Printf("TinyUptimeRobot: %d target(s), every %s, log=%s\n", len(targets), cfg.Interval, cfg.LogFile)
	fmt.Println("notifications: mail=", cfg.MailTo != "", " slack=", cfg.SlackURL != "", " gchat=", cfg.GChatURL != "")

	if *once {
		m.RunOnce()
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// simple TUI-ish live view: clear + print each round
	tui := !cfg.AlertOnly && isTerminal()
	for {
		if tui {
			fmt.Print("\033[H\033[2J") // clear screen
		}
		fmt.Printf("== TinyUptimeRobot | %s | %d targets | next in %s ==\n",
			time.Now().Format("15:04:05"), len(targets), cfg.Interval)
		m.RunOnce()
		select {
		case <-ctx.Done():
			fmt.Println("\nbye")
			return
		case <-time.After(cfg.Interval):
		}
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func timeoutFromEnv() time.Duration {
	return LoadConfig().Timeout
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
