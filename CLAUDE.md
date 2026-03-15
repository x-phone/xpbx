# CLAUDE.md

## Project Overview

xpbx is a Dockerized Asterisk PBX with a web management UI. It provides out-of-the-box extension, trunk, and dialplan management via a Go + HTMX web interface backed by SQLite Realtime.

Part of the [x-phone](https://github.com/x-phone) ecosystem.

## Repository Structure

- **`asterisk/`** — Asterisk configuration files and entrypoint script
- **`server/`** — Go web server (HTMX + templ + SQLite)
  - `cmd/xpbx/` — Entry point
  - `internal/ari/` — Asterisk REST Interface client
  - `internal/config/` — Environment variable configuration
  - `internal/database/` — SQLite layer (migrations, CRUD, cleanup)
  - `internal/handlers/` — HTTP request handlers
  - `internal/router/` — Route definitions + middleware
  - `templates/` — templ HTML templates (layouts, pages, partials)
  - `static/` — CSS
- **`Dockerfile.release`** — All-in-one image (Asterisk + xpbx)
- **`entrypoint-all.sh`** — All-in-one container entrypoint
- **`docker-compose.yml`** — Two-container dev setup

## Build & Run

```bash
make up          # Start Asterisk + xpbx web UI
make down        # Stop everything
make logs        # Follow logs
make asterisk-cli  # Open Asterisk console
make sip-status    # Show SIP endpoint registrations
```

### Server development (inside server/)

```bash
cd server
make generate    # Generate templ files
make build       # Build binary
make run         # Run locally
```

## Architecture

- **Asterisk** runs with PJSIP Realtime backed by SQLite (`/data/asterisk-realtime.db`)
- **xpbx server** manages the same SQLite file (CRUD for extensions, trunks, dialplan)
- Communication with Asterisk is via **ARI** (HTTP REST on port 8088)
- **No external database** — SQLite with WAL mode handles concurrent access
- **Docker image** (`ghcr.io/x-phone/xpbx`) bundles both Asterisk + xpbx in a single container
- **docker-compose.yml** runs them as separate containers for development

## Key Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `XPBX_LISTEN_ADDR` | `:8080` | Web UI listen address |
| `XPBX_DB_PATH` | `/data/asterisk-realtime.db` | SQLite database path |
| `ARI_HOST` | `asterisk` | Asterisk hostname |
| `ARI_PORT` | `8088` | ARI port |
| `ARI_USER` | `xpbx` | ARI auth user |
| `ARI_PASSWORD` | `secret` | ARI auth password |
| `EXTERNAL_IP` | auto-detected | Host IP for SIP NAT |
| `SIP_PORT` | `5060` | Asterisk SIP port |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `VOICEWORKER_HOST` | _(empty/disabled)_ | When set, creates voiceworker trunk at this host:port |
| `VOICEWORKER_EXTEN` | `2000` | Extension number that routes to voiceworker trunk |

## Conventions

- Never add `Co-Authored-By` lines to commit messages
- Go logging uses `logrus`
- Templates use `templ` (compile-time HTML generation)
- Frontend uses HTMX for interactivity + Tailwind CSS (CDN) for styling
- SQLite uses pure Go driver (`modernc.org/sqlite`) — no CGO required
