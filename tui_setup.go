package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Interactive TUI setup screen: add/remove targets, set check interval,
// configure notification channels, save to targets.txt / .env.

var (
	tuiTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(0, 1)
	tuiSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	tuiNormalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	tuiHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	tuiInputStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	tuiErrStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	tuiOkStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
)

type setupModel struct {
	step      int // which field is being edited
	fields    []string
	input     string
	targets   []string // current targets from file
	interval  string
	smtpHost  string
	mailTo    string
	slackURL  string
	gchatURL  string
	errMsg    string
	okMsg     string
	quitting  bool
	targetIdx int // which target line selected for delete
}

func newSetupModel(cfg *Config) *setupModel {
	m := &setupModel{
		fields: []string{
			"Add target (url|status|a@x.com,slack:URL,gchat:URL, or mysql:// / ssl://host)",
			"Check interval (e.g. 60s, 5m)",
			"SMTP host (email alerts)",
			"Mail to (comma separated)",
			"Slack webhook URL",
			"Google Chat webhook URL",
			"Save & exit",
			"Quit without saving",
		},
		interval: cfg.Interval.String(),
		smtpHost: cfg.SMTPHost,
		mailTo:   cfg.MailTo,
		slackURL: cfg.SlackURL,
		gchatURL: cfg.GChatURL,
	}
	if existing, err := LoadTargets(envOr("TARGETS_FILE", "targets.txt")); err == nil {
		for _, t := range existing {
			m.targets = append(m.targets, t.URL)
		}
	}
	return m
}

func (m *setupModel) Init() tea.Cmd { return nil }

func (m *setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.step > 0 {
				m.step--
			}
			m.errMsg, m.okMsg = "", ""
		case "down", "j", "tab":
			if m.step < len(m.fields)-1 {
				m.step++
			}
			m.errMsg, m.okMsg = "", ""
		case "enter":
			return m.apply()
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case "d":
			// delete selected target
			if len(m.targets) > 0 {
				m.targets = append(m.targets[:m.targetIdx], m.targets[m.targetIdx+1:]...)
				if m.targetIdx >= len(m.targets) && m.targetIdx > 0 {
					m.targetIdx--
				}
			}
		case "left":
			if m.targetIdx > 0 {
				m.targetIdx--
			}
		case "right":
			if m.targetIdx < len(m.targets)-1 {
				m.targetIdx++
			}
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}
	}
	return m, nil
}

func (m *setupModel) apply() (tea.Model, tea.Cmd) {
	m.errMsg, m.okMsg = "", ""
	switch m.step {
	case 0: // add target
		v := strings.TrimSpace(m.input)
		if v == "" {
			m.errMsg = "empty target"
			return m, nil
		}
		m.targets = append(m.targets, v)
		m.targetIdx = len(m.targets) - 1
		m.okMsg = "target added: " + v
		m.input = ""
	case 1:
		m.interval = strings.TrimSpace(m.input)
		m.input = ""
		m.okMsg = "interval set: " + m.interval
	case 2:
		m.smtpHost = strings.TrimSpace(m.input)
		m.input = ""
		m.okMsg = "smtp host set: " + m.smtpHost
	case 3:
		m.mailTo = strings.TrimSpace(m.input)
		m.input = ""
		m.okMsg = "mail to set: " + m.mailTo
	case 4:
		m.slackURL = strings.TrimSpace(m.input)
		m.input = ""
		m.okMsg = "slack webhook set"
	case 5:
		m.gchatURL = strings.TrimSpace(m.input)
		m.input = ""
		m.okMsg = "google chat webhook set"
	case 6: // save
		if err := m.save(); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.okMsg = "saved! bye"
		m.quitting = true
		return m, tea.Quit
	case 7:
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *setupModel) save() error {
	tf := envOr("TARGETS_FILE", "targets.txt")
	var b strings.Builder
	b.WriteString("# managed by TinyUptimeRobot setup TUI\r\n")
	for _, t := range m.targets {
		b.WriteString(t + "\r\n")
	}
	if err := os.WriteFile(tf, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tf, err)
	}

	// persist notification settings into .env (append/update)
	lines := []string{
		"CHECK_INTERVAL=" + m.interval,
		"SMTP_HOST=" + m.smtpHost,
		"MAIL_TO=" + m.mailTo,
		"SLACK_WEBHOOK_URL=" + m.slackURL,
		"GCHAT_WEBHOOK_URL=" + m.gchatURL,
	}
	if err := upsertEnv(".env", lines); err != nil {
		return err
	}
	return nil
}

// upsertEnv writes key=value pairs into .env, replacing existing keys.
func upsertEnv(path string, pairs []string) error {
	existing := map[string]string{}
	var order []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if i := strings.Index(line, "="); i > 0 {
				k := strings.TrimSpace(line[:i])
				if _, seen := existing[k]; !seen {
					order = append(order, k)
				}
				existing[k] = strings.TrimSpace(line[i+1:])
			}
		}
	}
	for _, p := range pairs {
		i := strings.Index(p, "=")
		k, v := p[:i], p[i+1:]
		if _, seen := existing[k]; !seen {
			order = append(order, k)
		}
		existing[k] = v
	}
	var b strings.Builder
	b.WriteString("# managed by TinyUptimeRobot setup TUI\r\n")
	for _, k := range order {
		b.WriteString(k + "=" + existing[k] + "\r\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func (m *setupModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(tuiTitleStyle.Render("⬆ TinyUptimeRobot setup") + "\n\n")

	b.WriteString(tuiNormalStyle.Render("Targets (" + fmt.Sprint(len(m.targets)) + ") — ←/→ select, d delete:"))
	b.WriteString("\n")
	if len(m.targets) == 0 {
		b.WriteString(tuiHelpStyle.Render("  (none yet — add one below)") + "\n")
	}
	for i, t := range m.targets {
		cursor := "  "
		style := tuiNormalStyle
		if i == m.targetIdx {
			cursor = "> "
			style = tuiSelectedStyle
		}
		b.WriteString(cursor + style.Render(t) + "\n")
	}
	b.WriteString("\n")

	for i, f := range m.fields {
		cursor, style := "  ", tuiNormalStyle
		if i == m.step {
			cursor, style = "> ", tuiSelectedStyle
		}
		line := cursor + style.Render(f)
		// show current values
		switch i {
		case 1:
			line += tuiHelpStyle.Render("  [now: " + m.interval + "]")
		case 2:
			line += tuiHelpStyle.Render("  [now: " + orDash(m.smtpHost) + "]")
		case 3:
			line += tuiHelpStyle.Render("  [now: " + orDash(m.mailTo) + "]")
		case 4:
			line += tuiHelpStyle.Render("  [now: " + orDash(short(m.slackURL)) + "]")
		case 5:
			line += tuiHelpStyle.Render("  [now: " + orDash(short(m.gchatURL)) + "]")
		}
		b.WriteString(line + "\n")
	}

	if m.step < 6 {
		b.WriteString("\n" + tuiInputStyle.Render(m.input) + "\n")
	}
	if m.errMsg != "" {
		b.WriteString("\n" + tuiErrStyle.Render("✗ "+m.errMsg))
	}
	if m.okMsg != "" {
		b.WriteString("\n" + tuiOkStyle.Render("✓ "+m.okMsg))
	}
	b.WriteString("\n\n" + tuiHelpStyle.Render("↑/↓ move · enter apply · type to input · ctrl+c quit"))
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func short(s string) string {
	if len(s) > 40 {
		return s[:37] + "..."
	}
	return s
}

// RunSetup launches the interactive TUI. Returns true if settings were saved.
func RunSetup(cfg *Config) (bool, error) {
	m := newSetupModel(cfg)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		return false, err
	}
	return true, nil
}
