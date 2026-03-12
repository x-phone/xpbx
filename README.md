# xpbx

Self-hosted PBX with a web UI. Manages Asterisk via SQLite Realtime and ARI.

> **Note:** This project is in early development and intended for **development and testing purposes only**. It is not yet hardened for production use.

![License](https://img.shields.io/badge/license-MIT-blue)

## What it does

- **Extensions** — Create SIP extensions, register softphones/IP phones
- **Call routing** — Ring, voicemail, ring+voicemail patterns per extension
- **Trunks** — Connect SIP trunk providers for inbound/outbound PSTN calls
- **Dialplan** — Visual dialplan editor (simple + advanced mode)
- **Voicemail** — Per-extension voicemail with PIN and email notification
- **Dashboard** — Live system status, active calls, SIP registrations

## Quick start

```bash
git clone https://github.com/x-phone/xpbx.git
cd xpbx
make up
```

Open **http://localhost:8080** — the web UI is ready.

Register a SIP phone (Obi200, Obi2182, Linphone, Obi100, etc.) to `your-ip:5060` with one of the seeded extensions (1001/1001, 1002/1002, 1003/1003).

### Network / NAT

`EXTERNAL_IP` tells Asterisk what IP address SIP phones should send media to. It's auto-detected via STUN by default, which works for cloud/VPS deployments.

Override for specific network setups:

```bash
# Tailscale / VPN
EXTERNAL_IP=100.96.49.117 make up

# LAN-only
EXTERNAL_IP=192.168.1.50 make up
```

Or set it in a `.env` file:

```
EXTERNAL_IP=100.96.49.117
```

## Architecture

```
┌──────────────┐     SQLite      ┌──────────────┐
│   xpbx       │───(Realtime)───▸│   Asterisk   │
│   Web UI     │                 │   PBX        │
│   Go/templ   │◂───(ARI)───────│   pjsip      │
└──────┬───────┘                 └──────┬───────┘
       │ :8080                          │ :5060
       │                                │
   Browser                         SIP Phones
```

- **xpbx** writes SIP configuration and dialplan rules to a shared SQLite database
- **Asterisk** reads them via [Realtime](https://docs.asterisk.org/Configuration/Interfaces/Asterisk-Realtime-Architecture/) (`res_config_sqlite3`)
- **ARI** (Asterisk REST Interface) provides live system info, channel management, and module control
- Changes take effect immediately — xpbx checkpoints the WAL and reloads the SQLite module

## Services

| Service | Port | Description |
|---------|------|-------------|
| **xpbx** | 8080 | Web UI and API |
| **Asterisk** | 5060/udp,tcp | SIP signaling |
| **Asterisk** | 10000-10099/udp | RTP media |
| **Asterisk** | 8088 | ARI (internal, not exposed) |

## Configuration

All configuration is via environment variables in `docker-compose.yml`:

| Variable | Default | Description |
|----------|---------|-------------|
| `EXTERNAL_IP` | *(STUN auto-detect)* | Host IP for SIP NAT/RTP |
| `XPBX_LISTEN_ADDR` | `:8080` | Web UI listen address |
| `XPBX_DB_PATH` | `/data/asterisk-realtime.db` | SQLite database path |
| `ARI_HOST` | `asterisk` | Asterisk hostname |
| `ARI_PORT` | `8088` | ARI port |
| `ARI_USER` | `xpbx` | ARI username |
| `ARI_PASSWORD` | `secret` | ARI password |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `SIP_PORT` | `5060` | SIP port shown in dashboard |

## Make targets

```
make up            # Start xpbx (Asterisk + web UI)
make down          # Stop everything
make build         # Rebuild containers
make logs          # Follow logs
make restart       # Restart all services
make asterisk-cli  # Open Asterisk console
make sip-status    # Show SIP endpoint registrations
```

## Development

### Prerequisites

- Docker and Docker Compose
- Go 1.25+ (for local development only)
- [templ](https://templ.guide/) CLI (`go install github.com/a-h/templ/cmd/templ@latest`)

### Local development

```bash
cd server
templ generate
go run ./cmd/xpbx
```

Requires a running Asterisk instance and shared SQLite database.

### Project structure

```
xpbx/
├── asterisk/
│   ├── config/           # Asterisk configuration files
│   └── scripts/          # Entrypoint with NAT/sound auto-setup
├── server/
│   ├── cmd/xpbx/         # Entry point
│   ├── internal/
│   │   ├── ari/          # Asterisk REST Interface client
│   │   ├── config/       # Environment-based configuration
│   │   ├── database/     # SQLite layer (extensions, trunks, dialplan, voicemail)
│   │   ├── dialplan/     # Dialplan pattern recognition
│   │   ├── handlers/     # HTTP handlers
│   │   └── router/       # Routes and middleware
│   ├── templates/        # templ HTML templates
│   ├── static/           # CSS and JS assets
│   └── Dockerfile
├── docker-compose.yml
└── Makefile
```

### Stack

- **Go** with [templ](https://templ.guide/) for server-rendered HTML
- **HTMX** + **Alpine.js** for interactive UI
- **Tailwind CSS** via CDN
- **SQLite** in WAL mode (shared between xpbx and Asterisk)
- **Asterisk** with pjsip and Realtime architecture

## Part of x-phone

xpbx is the PBX component of the [x-phone](https://github.com/x-phone) ecosystem:

- **xpbx** — Office PBX (this project)
- **xbridge** — Programmable voice gateway (Twilio-compatible API)
- **xphone-go** — Go SIP client library

Connect xpbx to xbridge via SIP trunks for programmable voice, external PSTN routing, and webhook-driven call control.

## License

MIT
