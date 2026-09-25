# AIP - Dedicated OpenCode Free Proxy Gateway & Web UI

A lightweight, dedicated OpenCode Free proxy gateway with proxy pooling, multi-usage failover, and a modern Web UI dashboard built with Go, React, and shadcn/ui (Base UI + Rhea preset).

---

## Features

- **Transparent Upstream Reverse Proxy (`opencode.ai/zen/v1`):**
  - Zero payload tampering: forwards raw OpenCode request bytes, session affinity (`x-opencode-session`), and tool calls.
  - Token injection: replaces client authorization with `Bearer public`.
  - Zero-buffering Server-Sent Events (SSE) token streaming.
  - Automatic 429 failover to the next available proxy in pool.

- **Proxy Pooling & Smart 429 Tracking:**
  - Protocols: `socks5://`, `socks5h://`, `http://`, `https://`, and `direct`.
  - Auto-normalization for raw vendor formats (`host:port:user:pass`, `username:password@host:port`, and `host:port`).
  - Real-time countdown timer for rate-limited proxies with automatic recovery upon cooldown expiration.
  - Curl-style non-intrusive latency and egress IP tester against Cloudflare CDN edge.

- **Automated OpenCode Configuration Sync:**
  - 1-click Link & Sync with `~/.config/opencode/opencode.json`.
  - Auto-sync on model catalog refresh.
  - Copyable JSON snippet for manual `.jsonc` users.

- **Active Free Model Discovery:**
  - Queries `https://models.dev/api.json` (with fallback to `https://models.opencode.ai/api.json`).
  - Filters strictly for active, non-deprecated models with 0 input and 0 output cost.
  - Auto-exposes models at `/v1/models` in OpenAI-compatible format.

- **Modern Web Dashboard (shadcn/ui + Base UI + Rhea Preset):**
  - Built with Vite, React 19, TypeScript, React Router, Tailwind CSS v4, and `@base-ui/react`.
  - **Overview (`/`):** Metric cards with jump links, model distribution metrics, and recent requests table with "View more".
  - **Integration & Models (`/integration`):** Link status, manual JSON snippet, and active free models catalog.
  - **Proxies (`/proxies`):** Node table with live countdown badges, batch actions (test & delete), and slide-over Add Proxy Drawer.
  - **Usage & Audit Logs (`/usage`):** Detailed request telemetry (latency, prompt, completion, cache read tokens) and slide-over row inspection Drawer.
  - **Console Logs (`/logs`):** Real-time Server-Sent Events (SSE) system daemon event stream with log level filtering and auto-scroll.

---

## 3-Command Global CLI

AIP provides a 3-command CLI installed directly into `~/.local/bin/aip`:

```bash
# Start background daemon on port 9100
aip start

# Check running status, PID, uptime, and active proxies
aip status

# Gracefully stop the background daemon
aip stop
```

---

## Build & Installation

### Prerequisites
- Go 1.22+
- Node.js 20+ & npm

### Commands
```bash
# Build standalone executable with embedded static web assets
make build

# Install to ~/.local/bin/aip
make install

# Start development mode with hot module reloading (HMR)
make dev

# Clean artifacts
make clean
```

---

## Configuration & Storage

AIP stores all runtime data centrally in the user's home directory:
- `~/.aip/aip.db`: Pure-Go SQLite database (`modernc.org/sqlite`, zero CGO dependencies).
- `~/.aip/aip.pid`: Daemon PID file.
- `~/.aip/aip.log`: Daemon stdout and stderr logs.
