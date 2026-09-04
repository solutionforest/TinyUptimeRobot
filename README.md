<div align="center">

<img src="assets/icon.svg" width="128" alt="TinyUptimeRobot logo"/>

# TinyUptimeRobot

[![CI](https://github.com/solutionforest/TinyUptimeRobot/actions/workflows/ci.yml/badge.svg)](https://github.com/solutionforest/TinyUptimeRobot/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![Docker](https://img.shields.io/badge/ghcr.io-solutionforest%2Ftinyuptimerobot-2496ED?logo=docker&logoColor=white)

**A tiny self-hosted website, SSL-certificate and database liveness monitor in one Docker image.**

No database required — results go to a plain `.txt` file (CRLF line endings) with automatic cleanup/rotation. Alerts via **Email (SMTP)**, **Slack**, or **Google Chat**. Interactive **terminal TUI** for setup.

`docker pull ghcr.io/solutionforest/tinyuptimerobot:latest`

</div>

## Why use TinyUptimeRobot?

- **One tiny binary, zero infrastructure** — no database, no queue, no control plane. A single ~15MB Docker image or one Go binary. If you can run `docker run`, you have a monitoring system.
- **Everything in one plain text file** — status history in `status.txt`, readable with `cat`/`tail`/`grep`. No lock-in: your monitoring data is yours, exportable by definition.
- **Not just websites** — monitors HTTP endpoints, **SSL certificate validity & expiry**, and **live database connections** (MySQL, PostgreSQL, SQLite, SQL Server) out of the box.
- **Alerts where your team already is** — Slack, Google Chat, and email fire **in parallel** on DOWN and recovery — globally or **per target**. No vendor SaaS, no per-seat pricing, no data leaving your network.
- **Self-hosted & private** — your target URLs may contain database passwords. TinyUptimeRobot runs inside your infra; nothing phones home.
- **5-minute setup** — `docker run` one command, or launch the built-in `--setup` TUI to configure targets and notification channels interactively in the terminal.
- **Actually tested** — 48 unit tests covering checks, rotation, alerts and notification delivery, wired into GitHub Actions CI with automatic GHCR image publishing.

<img width="1376" height="768" alt="image" src="https://github.com/user-attachments/assets/ad406df1-ad8f-4d09-96ed-d698446f3290" />

## How it works

```
                        ┌──────────────────────────────────────────────────────────┐
                        │                    TinyUptimeRobot                        │
                        │                                                          │
   targets.txt ───────▶ │  ┌─────────┐   every CHECK_INTERVAL   ┌───────────────┐  │
   (http / ssl / db)    │  │ Loader │ ────────────────────────▶ │  Check Engine  │  │
                        │  └─────────┘                          │  (parallel)    │  │
   .env ──────────────▶ │  config: interval, SMTP,              └───────┬───────┘  │
   (channels, timeouts) │  webhooks, timeouts                           │          │
                        │                                               ▼          │
                        │                          ┌─────────────────────────────┐  │
                        │                          │  dispatch by URL scheme      │  │
                        │                          ├─────────────────────────────┤  │
                        │                          │ http://  → HTTP GET probe    │  │
                        │                          │ ssl://   → TLS handshake,    │  │
                        │                          │            cert expiry check │  │
                        │                          │ mysql://  ┐                  │  │
                        │                          │ postgres://├→ SELECT 1 probe │  │
                        │                          │ sqlite:// │                  │  │
                        │                          │ sqlserver://┘                 │  │
                        │                          └──────────────┬──────────────┘  │
                        │                                         │                  │
                        │                        state changed?   │                  │
                        │                       (UP→DOWN, DOWN→UP)│                  │
                        │                                         ▼                  │
                        │                          ┌─────────────────────────────┐  │
                        │                          │  🔔 Notify (parallel)        │  │
                        │                          │  📧 SMTP  💬 Slack  💬 GChat │  │
                        │                          └─────────────────────────────┘  │
                        │                                         │                  │
                        │                                         ▼                  │
                        │                          ┌─────────────────────────────┐  │
                        │                          │  📄 status.txt (CRLF)        │  │
                        │                          │  auto-rotate at LOG_MAX_KB   │  │
                        │                          └─────────────────────────────┘  │
                        └──────────────────────────────────────────────────────────┘
```

```
  every CHECK_INTERVAL
  ─────────────────────▶  check all targets in parallel
                              │
                 ┌────────────┼────────────┐
                 ▼            ▼            ▼
              UP ✓         DOWN ✗      was DOWN, now UP
                 │            │            │
                 │            ▼            ▼
                 │        🔴 alert      🟢 recover alert
                 │            │            │
                 ▼            ▼            ▼
           append one line to status.txt  (never spam on steady UP)
```

## Features

- HTTP uptime checks (`UP`/`DOWN`, optional expected status code per target)
- SSL certificate checks (`ssl://host`) — valid chain, expiry warning threshold
- Database checks (`mysql://`, `postgres://`, `sqlite://`, `sqlserver://`) — live `SELECT 1` probe
- Config-driven entirely by environment variables
- Plain text log (`status.txt`) with auto rotation — no DB
- Notifications on DOWN and recovery: SMTP mail, Slack webhook, Google Chat webhook
- Cron-like frequency via `CHECK_INTERVAL`
- Interactive **TUI setup screen** (`--setup`): add/remove targets, set interval & channels, saves to files
- TUI-style live view in the terminal; `-once`, `-tail N` CLI flags
- 35+ unit tests (`go test ./...`), CI via GitHub Actions

## ⚠️ Security warning — READ THIS FIRST

> **`targets.txt` is a RAW file.** It can contain database connection strings with
> **plaintext usernames and passwords**, internal hostnames, and webhook URLs.
>
> **NEVER commit `targets.txt` (or `.env`) to your git repository.**
>
> - Both files are already listed in `.gitignore` — keep it that way
> - Use `targets.example.txt` as a safe template and copy it to `targets.txt`
> - If you must share config, use secrets management (Docker secrets, K8s secrets,
>   CI variables) — not your repo
> - A leaked DB password in git history stays in git history forever
>
> **RAW username/password should never go to your repo. This is serious.**

## Files

| File | Purpose |
|---|---|
| `targets.txt` | **RAW, gitignored** — URLs/DBs/SSL hosts to monitor, one per line |
| `targets.example.txt` | Safe template — copy to `targets.txt` and edit |
| `.env` | **RAW, gitignored** — notification & runtime config |
| `.env.example` | Safe template of all options |
| `status.txt` | Output log (created automatically, auto-rotated) |

## Quick start (Docker — recommended)

Pull the free prebuilt image from GitHub Container Registry (public, no login needed):

```bash
mkdir -p data
docker run -d --name tinyuptimerobot \
  --env-file .env \
  -v "$(pwd)/targets.txt:/app/targets.txt:ro" \
  -v "$(pwd)/data:/app/data" \
  ghcr.io/solutionforest/tinyuptimerobot:latest
```

Or build it yourself:

```bash
cp .env.example .env
cp targets.example.txt targets.txt
$EDITOR targets.txt .env

docker build -t tinyuptimerobot .
docker run -d --name tinyuptimerobot \
  --env-file .env \
  -v "$(pwd)/data:/app/data" \
  tinyuptimerobot
```

Logs land in `./data/status.txt` inside your mounted volume.

### docker-compose (optional)

```yaml
services:
  tinyuptimerobot:
    image: ghcr.io/solutionforest/tinyuptimerobot:latest
    env_file: .env
    volumes:
      - ./targets.txt:/app/targets.txt:ro
      - ./data:/app/data
    restart: unless-stopped
```

> **Important:** in your `.env`, set `LOG_FILE=/app/data/status.txt` (the absolute
> container path). If you leave it as the default relative `status.txt`, the file
> is written to `/app/status.txt`, which is **not writable** by the container's
> non-root user — you'll get `log write failed: permission denied`. The
> `.env.example` and image default already use the correct path.

## Quick start (local, no Docker)

```bash
go build -o TinyUptimeRobot .
./TinyUptimeRobot                 # live loop (clears screen each round)
./TinyUptimeRobot -once           # single round, exit
./TinyUptimeRobot -tail 20        # show last 20 log lines, exit
./TinyUptimeRobot -setup          # interactive TUI configuration
./TinyUptimeRobot -targets myurls.txt
```

## TUI setup screen

Launch the interactive terminal setup:

```bash
./TinyUptimeRobot --setup
```

```
 ⬆ TinyUptimeRobot setup

Targets (2) — ←/→ select, d delete:
  https://example.com
> mysql://user:****@db.internal:3306/app

> Add target (url|status|emails, or mysql:// / postgres:// / sqlite:// / ssl://host)
  Check interval (e.g. 60s, 5m)        [now: 1m0s]
  SMTP host (email alerts)             [now: -]
  Mail to (comma separated)            [now: -]
  Slack webhook URL                    [now: -]
  Google Chat webhook URL              [now: -]
  Save & exit
  Quit without saving

↑/↓ move · enter apply · type to input · ctrl+c quit
```

Saving writes `targets.txt` and updates `.env` automatically.

## Configuration (env)

### Monitoring

| Variable | Default | Description |
|---|---|---|
| `TARGETS_FILE` | `targets.txt` | Path to targets file |
| `CHECK_INTERVAL` | `60s` | Check frequency (`30s`, `5m`, `1h`, or plain seconds `45`) |
| `HTTP_TIMEOUT` | `10s` | Per-request / per-probe timeout |
| `LOG_FILE` | `status.txt` | Text log file path |
| `LOG_MAX_KB` | `512` | Rotate (cleanup) the txt file when it exceeds this size |
| `ALERT_ONLY` | `false` | Only print alert lines to console |
| `NOTIFY_RECOVER` | `true` | Also notify when a target comes back up |
| `SSL_WARN_DAYS` | `30` | `ssl://` targets fail when cert expires within N days |

### Email (all optional — skip to disable)

| Variable | Example |
|---|---|
| `SMTP_HOST` | `smtp.gmail.com` |
| `SMTP_PORT` | `587` |
| `SMTP_USER` | `you@gmail.com` |
| `SMTP_PASS` | app password |
| `MAIL_FROM` | `you@gmail.com` |
| `MAIL_TO` | `ops@example.com, backup@example.com` (global fallback, comma separated) |

> `SMTP_PASS` is a raw credential — it lives in `.env` (gitignored) or your
> secrets manager. Never put it in the repo.

### Chat webhooks (optional)

```bash
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXX/YYY/ZZZ
GCHAT_WEBHOOK_URL=https://chat.googleapis.com/v1/spaces/AAAA/messages?key=...&token=...
```

Slack: create an [Incoming Webhook](https://api.slack.com/messaging/webhooks).
Google Chat: Space → Apps & Integrations → Webhooks.

## targets.txt format

```
# comment
# --- HTTP checks ---
https://example.com
https://www.google.com|200        # expect exactly HTTP 200
https://api.example.com|200|api-oncall@example.com,backup@example.com

# --- SSL certificate checks (valid chain + expiry) ---
ssl://example.com                 # port 443, warn < SSL_WARN_DAYS (30)
ssl://example.com:8443            # custom port
ssl://example.com|14              # DOWN when cert expires in <14 days

# --- Database checks (live SELECT 1 probe) ---
mysql://user:pass@db.host:3306/app
postgres://user:pass@db.host:5432/prod
sqlite:///app/data/local.db
sqlserver://sa:pass@db.host:1433?database=master
```

⚠ **The DB lines contain raw passwords.** `targets.txt` is gitignored — never commit it (see security warning above).

Any HTTP 2xx/3xx counts as UP unless an expected status is specified. SSL targets fail on invalid/untrusted/expired certs or when inside the warn threshold. DB targets fail when connection or the probe query fails.

## SSL certificate checks (`ssl://`)

TinyUptimeRobot performs a real **TLS handshake** against the host, verifies the certificate chain (untrusted/self-signed/expired certs fail), and checks **how many days are left before expiry**.

```
ssl://example.com              # port 443, warn at SSL_WARN_DAYS (default 30)
ssl://example.com:8443         # custom TLS port
ssl://example.com|14           # per-target warn threshold: DOWN if <14 days left
```

**When is it UP?**
- TLS handshake succeeds with a valid, trusted certificate chain
- Certificate is not expired
- Days-to-expiry ≥ warn threshold (`SSL_WARN_DAYS` env or per-target `|N`)

**When is it DOWN?**
- Handshake fails (host down, wrong port, TLS version too old)
- Certificate is self-signed / untrusted / hostname mismatch
- Certificate **expired**, or expires within the warn threshold

Example log line with cert details:

```
2026-09-03T16:30:36+08:00 | UP | ssl://example.com | ssl | cert=example.com | issuer=R3 | expires=2026-11-14 (72d)
```

Global threshold via env:

```bash
SSL_WARN_DAYS=30     # default — all ssl:// targets fail when expiry is closer
```

Typical use: run daily (`CHECK_INTERVAL=24h`) and get an email/Slack ping weeks before a cert expires — no more surprise expired-certificate incidents.

## Database liveness probes (`mysql://`, `postgres://`, `sqlite://`, `sqlserver://`)

A DB target opens a **real connection** and runs a lightweight `SELECT 1` probe. This verifies the full stack: network reachable → auth works (user/password valid) → server accepting queries. A ping or port-check alone does not tell you any of that.

```
mysql://user:pass@db.host:3306/app
postgres://user:pass@db.host:5432/prod          # postgresql:// also accepted
sqlite:///app/data/local.db                     # file must exist, open & be readable
sqlserver://sa:pass@db.host:1433?database=master
```

**When is it UP?**
- Connection + authentication succeed within `HTTP_TIMEOUT`
- `SELECT 1` executes successfully

**When is it DOWN?**
- Host unreachable / wrong port / firewall
- **Wrong username or password** (auth failure)
- Server up but refusing queries (max_connections exhausted, disk full, locked)
- SQLite file corrupted or unreadable

Example log lines:

```
2026-09-03T16:30:37+08:00 | UP | postgres://db:5432/prod | db | 12ms
2026-09-03T16:30:38+08:00 | DOWN | mysql://db:3306/app | db | err=dial tcp: connection refused
```

Each probe opens a fresh short-lived connection (max 1 conn) — no pooling, no lingering sessions on your database.

> ⚠ **DB targets contain raw passwords.** `targets.txt` is gitignored — never commit it (see security warning above).

## Log format

```
2026-09-03T16:30:34+08:00 | UP | https://example.com | http | http=200 | 67ms
2026-09-03T16:30:35+08:00 | DOWN | http://x.com | http | http=500 | 463ms
2026-09-03T16:30:36+08:00 | UP | ssl://example.com | ssl | cert=example.com | issuer=R3 | expires=2026-11-14 (72d)
2026-09-03T16:30:37+08:00 | UP | postgres://db:5432/prod | db | 12ms
```

Lines are `\r\n` terminated. When the file exceeds `LOG_MAX_KB`, the oldest
lines are trimmed automatically (keeps roughly the last 64KB).

## Cron usage

`CHECK_INTERVAL` acts as your cron frequency. For a real crontab, run once per invocation:

```cron
*/5 * * * * cd /app && ./TinyUptimeRobot -once >> /dev/null 2>&1
```

## Alerts

A notification is sent (to all configured channels in parallel) when:

- a target goes **DOWN** (was UP, now failing), or
- a target **RECOVERS** (was down, now OK — if `NOTIFY_RECOVER=true`).

Steady-state UP checks do **not** spam notifications.

### Per-target notification lists

Add a third pipe-separated field to a target when its alerts should go to
specific channels instead of (or in addition to) the global ones:

```
# emails only (comma separated) — overrides MAIL_TO
https://api.example.com|200|api-oncall@example.com,backup@example.com

# emails, explicit form (equivalent to bare text above)
https://api.example.com|200|email:api-oncall@example.com,backup@example.com

# route to a specific Slack webhook
https://api.example.com|200|slack:https://hooks.slack.com/services/XXX/YYY

# route to a specific Google Chat webhook
https://api.example.com|200|gchat:https://chat.googleapis.com/v1/spaces/BBB/...

# mix all three, comma separated
https://api.example.com|200|api-oncall@example.com,slack:https://hooks.slack.com/services/XXX/YYY
```

Each channel in the third field overrides the global setting for that target
only: bare text (or `email:` prefixed) is treated as email addresses,
`slack:<url>` overrides `SLACK_WEBHOOK_URL`, `gchat:<url>` overrides
`GCHAT_WEBHOOK_URL`. Targets without the third field use the global config
(`MAIL_TO`, `SLACK_WEBHOOK_URL`, `GCHAT_WEBHOOK_URL`) for all channels.

## CI & registry

GitHub Actions (`.github/workflows/ci.yml`):

- runs `go vet` + full test suite on every push/PR
- builds & pushes the Docker image to **GHCR** (`ghcr.io/solutionforest/tinyuptimerobot`) on every push to `main` (tagged `latest`) — free public registry, `docker pull` with no login
- **on a version tag** (`v1.2.3` style), additionally pushes the image tagged with that version (`:1.2.3`) and creates a GitHub Release whose notes come from the **tag annotation message**, plus auto-generated commit list and the `docker pull` commands

Release a new version (the tag annotation becomes the release notes — write it well):

```bash
git tag -a v1.2.3 -m "TinyUptimeRobot v1.2.3

Added:
- ...

Fixed:
- ..."

git push origin v1.2.3
# CI runs: test → docker image (ghcr.io/...:1.2.3) → GitHub Release with those notes
```

## Development

```bash
go vet ./...
go test ./... -count=1
```

## License

MIT — see [LICENSE](LICENSE).
