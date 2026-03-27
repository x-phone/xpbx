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

### Docker

```bash
docker run -d --name xpbx \
  -v xpbx-data:/data \
  -p 5060:5060/udp -p 5060:5060/tcp \
  -p 8080:8080 \
  -p 10000-10099:10000-10099/udp \
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
# Docker
docker run -d --name xpbx \
  -v xpbx-data:/data \
  -p 5060:5060/udp -p 5060:5060/tcp \
  -p 8080:8080 \
  -p 10000-10099:10000-10099/udp \
  -e EXTERNAL_IP=192.168.1.50 \
  ghcr.io/x-phone/xpbx:latest

# Docker Compose
EXTERNAL_IP=100.96.49.117 make up
```

Or set it in a `.env` file (docker-compose only):

```
EXTERNAL_IP=100.96.49.117
```

### Asterisk CLI

Access the Asterisk console for debugging:

```bash
# Docker
docker exec -it xpbx asterisk -rvvv

# Docker Compose
make asterisk-cli
```

## Architecture

```
┌─────────────────────────────────┐
│         xpbx container          │
│                                 │
│  ┌──────────┐    ┌───────────┐  │
│  │  xpbx    │───▸│ Asterisk  │  │
│  │  Web UI  │    │ PBX       │  │
│  │  Go/templ│◂───│ pjsip     │  │
│  └────┬─────┘    └─────┬─────┘  │
│       │ :8080          │ :5060  │
│       │    SQLite ◂──▸ │        │
└───────┼────────────────┼────────┘
        │                │
    Browser          SIP Phones
```

- **xpbx** writes SIP configuration and dialplan rules to a shared SQLite database
- **Asterisk** reads them via [Realtime](https://docs.asterisk.org/Configuration/Interfaces/Asterisk-Realtime-Architecture/) (`res_config_sqlite3`)
- **ARI** (Asterisk REST Interface) provides live system info, channel management, and module control
- Changes take effect immediately — xpbx checkpoints the WAL and reloads the SQLite module
- The Docker image bundles both services; `docker-compose.yml` runs them as separate containers for development

## Services

| Service | Port | Description |
|---------|------|-------------|
| **xpbx** | 8080 | Web UI and API |
| **Asterisk** | 5060/udp,tcp | SIP signaling |
| **Asterisk** | 10000-10099/udp | RTP media |
| **Asterisk** | 8088 | ARI (internal, not exposed) |

## Configuration

All configuration is via environment variables:

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

```bash
# Docker
docker run -d --name xpbx \
  -v xpbx-data:/data \
  -v ./my-pjsip.conf:/etc/asterisk/pjsip.conf:ro \
  -p 5060:5060/udp -p 5060:5060/tcp \
  -p 8080:8080 \
  -p 10000-10099:10000-10099/udp \
  ghcr.io/x-phone/xpbx:latest
```

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
│   └── Dockerfile        # Server-only image (for docker-compose)
├── Dockerfile.release    # All-in-one image (Asterisk + xpbx)
├── entrypoint-all.sh     # All-in-one entrypoint
├── docker-compose.yml    # Two-container dev setup
└── Makefile
```

### Stack

- **Go** with [templ](https://templ.guide/) for server-rendered HTML
- **HTMX** + **Alpine.js** for interactive UI
- **Tailwind CSS** via CDN
- **SQLite** in WAL mode (shared between xpbx and Asterisk)
- **Asterisk** with pjsip and Realtime architecture

## API Reference

xpbx exposes two sets of endpoints:

- **JSON API** (`/api/...`) — for programmatic automation. Accepts and returns `application/json`.
- **HTML endpoints** — for the web UI (HTMX). Accept `application/x-www-form-urlencoded`, return HTML.

### JSON API — Trunks

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/trunks` | List all trunks |
| `GET` | `/api/trunks/{id}` | Get single trunk |
| `POST` | `/api/trunks` | Create trunk |
| `PUT` | `/api/trunks/{id}` | Update trunk |
| `DELETE` | `/api/trunks/{id}` | Delete trunk |

```bash
# List trunks
curl http://localhost:8080/api/trunks

# Create a trunk
curl -X POST http://localhost:8080/api/trunks \
  -H "Content-Type: application/json" \
  -d '{"name":"my-trunk","host":"sip.provider.com","port":5060,"context":"from-trunk","codecs":"ulaw","auth_user":"myuser","auth_pass":"mypass"}'

# Update a trunk
curl -X PUT http://localhost:8080/api/trunks/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"my-trunk","host":"sip2.provider.com","port":5060,"context":"from-trunk"}'

# Delete a trunk
curl -X DELETE http://localhost:8080/api/trunks/1
```

### JSON API — Dialplan

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/dialplan` | List all rules |
| `GET` | `/api/dialplan/{id}` | Get single rule |
| `POST` | `/api/dialplan` | Create rule |
| `PUT` | `/api/dialplan/{id}` | Update rule |
| `DELETE` | `/api/dialplan/{id}` | Delete rule |

```bash
# List dialplan rules
curl http://localhost:8080/api/dialplan

# Create a dialplan rule
curl -X POST http://localhost:8080/api/dialplan \
  -H "Content-Type: application/json" \
  -d '{"context":"from-internal","exten":"_3XXX","priority":1,"app":"Dial","appdata":"PJSIP/${EXTEN}@my-trunk,30"}'

# Delete a rule
curl -X DELETE http://localhost:8080/api/dialplan/10
```

### JSON API — System

| Method | Path | Description |
|--------|------|-------------|
| `DELETE` | `/api/calls/{channelId}` | Hang up an active call |
| `POST` | `/api/asterisk/reload` | Reload Asterisk PJSIP module |

### HTML Endpoints (Web UI)

The web UI uses form-encoded HTML endpoints. These can also be called programmatically but the JSON API above is preferred for automation.

| Resource | List | Create | Update | Delete |
|----------|------|--------|--------|--------|
| Extensions | `GET /extensions` | `POST /extensions` | `PUT /extensions/{id}` | `DELETE /extensions/{id}` |
| Trunks | `GET /trunks` | `POST /trunks` | `PUT /trunks/{id}` | `DELETE /trunks/{id}` |
| Dialplan | `GET /dialplan` | `POST /dialplan` | `PUT /dialplan/{id}` | `DELETE /dialplan/{id}` |
| Dashboard | `GET /dashboard` | — | — | — |

## Part of x-phone

xpbx is the PBX component of the [x-phone](https://github.com/x-phone) ecosystem:

- **xpbx** — Office PBX (this project)
- **xbridge** — Programmable voice gateway (Twilio-compatible API)
- **xphone-go** — Go SIP client library

### xbridge integration

To connect xpbx to [xbridge](https://github.com/x-phone/xbridge) for voice AI, set `VOICEWORKER_HOST`:

```bash
# Docker
docker run -d --name xpbx \
  -v xpbx-data:/data \
  -p 5060:5060/udp -p 5060:5060/tcp \
  -p 8080:8080 \
  -p 10000-10099:10000-10099/udp \
  -e VOICEWORKER_HOST=xbridge:5080 \
  ghcr.io/x-phone/xpbx:latest
```

```yaml
# docker-compose.yml
environment:
  - VOICEWORKER_HOST=xbridge:5080
  - VOICEWORKER_EXTEN=2000        # optional, default is 2000
```

This auto-generates a voiceworker SIP trunk and a dialplan route so that dialing extension 2000 from any registered phone reaches xbridge. No config file overrides needed.

## License

MIT
