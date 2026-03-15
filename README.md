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

### Docker (no clone needed)

```bash
docker run -d --name xpbx \
  -p 5060:5060/udp -p 5060:5060/tcp \
  -p 8080:8080 \
  -p 10000-10099:10000-10099/udp \
  -e EXTERNAL_IP=$(tailscale ip -4) \
  ghcr.io/x-phone/xpbx:latest
```

### Docker Compose (for development)

```bash
git clone https://github.com/x-phone/xpbx.git
cd xpbx
make up
```

---

Open **http://localhost:8080** — the web UI is ready.

Register a SIP phone (Zoiper, Linphone, Obi200, etc.) to `your-ip:5060` with one of the seeded extensions (1001/1002/1003, password: `password123`).

A sample SIP trunk (`my-provider` → `sip.example.com`) and outbound route (`9 + 10 digits`) are also seeded — edit them in the UI with your real provider details.

### Network / NAT

`EXTERNAL_IP` tells Asterisk what IP address SIP phones should send media to. It's auto-detected via STUN by default, which works for cloud/VPS deployments.

Override for specific network setups:

```bash
# Docker — Tailscale / VPN
docker run -d --name xpbx \
  -p 5060:5060/udp -p 5060:5060/tcp \
  -p 8080:8080 \
  -p 10000-10099:10000-10099/udp \
  -e EXTERNAL_IP=192.168.1.50 \
  ghcr.io/x-phone/xpbx:latest

# Docker Compose — Tailscale / VPN
EXTERNAL_IP=100.96.49.117 make up

# Docker Compose — LAN-only
EXTERNAL_IP=192.168.1.50 make up
```

Or set it in a `.env` file (docker-compose only):

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
| `VOICEWORKER_HOST` | *(empty/disabled)* | When set, creates voiceworker trunk pointed at this host:port |
| `VOICEWORKER_EXTEN` | `2000` | Extension number that routes to voiceworker trunk |

### Asterisk config overrides

The default Asterisk configuration works out of the box. For advanced use cases, you can override any config file via volume mounts:

```yaml
# docker-compose.override.yml
services:
  asterisk:
    volumes:
      - ./my-pjsip.conf:/etc/asterisk/pjsip.conf:ro
      - ./my-extensions.conf:/etc/asterisk/extensions.conf:ro
```

> **Note:** Most common customizations (NAT, voiceworker trunk, xbridge integration) are handled by environment variables and don't require config overrides.

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

## API Reference

xpbx exposes a web UI and a set of HTTP endpoints for managing extensions, trunks, and dialplan rules. Most endpoints return HTML (designed for HTMX), but can be called from any HTTP client.

All write operations use **form-encoded** request bodies (`application/x-www-form-urlencoded`).

### Extensions

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/extensions` | List all extensions |
| `GET` | `/extensions/new` | New extension form |
| `GET` | `/extensions/{id}/edit` | Edit extension form |
| `POST` | `/extensions` | Create extension |
| `PUT` | `/extensions/{id}` | Update extension |
| `DELETE` | `/extensions/{id}` | Delete extension |

**Create / Update fields:**

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `extension` | yes | — | Extension number (e.g. `1001`) |
| `password` | yes | — | SIP auth password |
| `context` | yes | `from-internal` | Dialplan context |
| `display_name` | no | — | Friendly name |
| `codecs` | no | `ulaw` | Comma-separated codec list |
| `max_contacts` | no | `10` | Max simultaneous registrations |
| `routing_enabled` | no | `on` | Enable call routing rules |
| `routing_pattern` | no | `ring_voicemail` | One of: `ring_only`, `ring_voicemail`, `voicemail_only` |
| `routing_timeout` | no | `20` | Ring timeout in seconds |
| `vm_enabled` | no | `on` | Enable voicemail |
| `vm_pin` | no | `0000` | Voicemail access PIN |
| `vm_email` | no | — | Email for voicemail notifications |

**Example — create an extension:**

```bash
curl -X POST http://localhost:8080/extensions \
  -d "extension=1004&password=secret4&context=from-internal&display_name=Office+Phone&codecs=ulaw,g722"
```

### Trunks

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/trunks` | List all trunks |
| `GET` | `/trunks/new` | New trunk form |
| `GET` | `/trunks/{id}/edit` | Edit trunk form |
| `POST` | `/trunks` | Create trunk |
| `PUT` | `/trunks/{id}` | Update trunk |
| `DELETE` | `/trunks/{id}` | Delete trunk |

**Create / Update fields:**

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | yes | — | Trunk identifier (unique) |
| `host` | yes | — | SIP server hostname or IP |
| `context` | yes | `from-trunk` | Dialplan context for inbound calls |
| `display_name` | no | — | Friendly name |
| `provider` | no | — | Provider name (informational) |
| `port` | no | `5060` | SIP port |
| `auth_user` | no | — | SIP authentication username |
| `auth_pass` | no | — | SIP authentication password |
| `codecs` | no | `ulaw` | Comma-separated codec list |

**Example — create a trunk:**

```bash
curl -X POST http://localhost:8080/trunks \
  -d "name=my-provider&host=sip.provider.com&context=from-trunk&auth_user=myuser&auth_pass=mypass"
```

### Dialplan

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/dialplan` | List rules (simple view) |
| `GET` | `/dialplan?mode=advanced` | List rules (raw table view) |
| `GET` | `/dialplan/new` | New rule form |
| `GET` | `/dialplan/{id}/edit` | Edit rule form |
| `POST` | `/dialplan` | Create rule |
| `PUT` | `/dialplan/{id}` | Update rule |
| `DELETE` | `/dialplan/{id}` | Delete rule |

**Create / Update fields:**

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `context` | yes | `from-internal` | Dialplan context |
| `exten` | yes | — | Extension pattern (e.g. `1001`, `_2XXX`, `_NXXXXXX`) |
| `priority` | yes | `1` | Execution order |
| `app` | yes | — | Asterisk application (`Dial`, `VoiceMail`, `Hangup`, etc.) |
| `appdata` | no | — | Application arguments (e.g. `PJSIP/1001,20`) |

**Example — create a dialplan rule:**

```bash
curl -X POST http://localhost:8080/dialplan \
  -d "context=from-internal&exten=_9NXXXXXX&priority=1&app=Dial&appdata=PJSIP/my-provider/\${EXTEN:1}"
```

### Dashboard & System

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/dashboard` | Dashboard page |
| `GET` | `/partials/system-info` | Asterisk system info (HTML partial, polled every 10s) |
| `GET` | `/partials/registrations` | Registered SIP endpoints (HTML partial, polled every 5s) |
| `GET` | `/partials/active-calls` | Active call channels (HTML partial, polled every 5s) |
| `GET` | `/partials/sip-config/{id}` | SIP configuration modal for extension |
| `DELETE` | `/api/calls/{channelId}` | Hang up an active call |
| `POST` | `/api/asterisk/reload` | Reload Asterisk PJSIP module |

**Example — hang up a call:**

```bash
curl -X DELETE http://localhost:8080/api/calls/1234567890.42
```

**Example — reload PJSIP:**

```bash
curl -X POST http://localhost:8080/api/asterisk/reload
```

## Part of x-phone

xpbx is the PBX component of the [x-phone](https://github.com/x-phone) ecosystem:

- **xpbx** — Office PBX (this project)
- **xbridge** — Programmable voice gateway (Twilio-compatible API)
- **xphone-go** — Go SIP client library

### xbridge integration

To connect xpbx to [xbridge](https://github.com/x-phone/xbridge) for voice AI, set `VOICEWORKER_HOST` in your `.env` or `docker-compose.yml`:

```yaml
environment:
  - VOICEWORKER_HOST=xbridge:5080
  - VOICEWORKER_EXTEN=2000        # optional, default is 2000
```

This auto-generates a voiceworker SIP trunk and a dialplan route so that dialing extension 2000 from any registered phone reaches xbridge. No config file overrides needed.

## License

MIT
