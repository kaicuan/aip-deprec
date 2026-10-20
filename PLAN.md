# AIP - Project & Implementation Plan

A lightweight, dedicated OpenCode Free proxy gateway with proxy pooling, multi-usage failover, and a modern Web UI dashboard built with Go, React, and shadcn/ui (Base UI + Rhea preset).

---

## 1. System Overview & Technology Stack

* **Backend & Core Engine:**
  * **Language:** Go (Golang) compiled to a single standalone binary.
  * **Database:** Pure-Go SQLite in `~/.aip/aip.db` (`modernc.org/sqlite`, zero CGO/GCC dependencies).
  * **Port:** Unified single port `9100` serving both the Web UI and API.
  * **Asset Embedding:** Go embeds compiled React static assets (`//go:embed all:web/dist`).
* **Frontend (`web/`):**
  * **Framework:** Vite + React (TypeScript) + React Router v6.
  * **Design System:** Tailwind CSS + **shadcn/ui** with **Base UI (`@base-ui/react`)** primitives and the **Rhea preset**
  * **Icons & Notifications:** Lucide React icons, native Base UI Toast notifications.
* **Global CLI (`aip`):**
  * Installed to `~/.local/bin/aip` (or in `$PATH`), usable from **any terminal folder**.
  * Centralized runtime state stored in `~/.aip/`.

---

## 2. Project Directory Layout

```text
ai-proxy/
├── cmd/
│   └── aip/
│       └── main.go              # Global CLI: aip start | stop | status
├── internal/
│   ├── config/                  # Global paths & defaults (~/.aip/ directory)
│   ├── daemon/                  # Background process management (PID & log redirection)
│   ├── db/                      # Pure-Go SQLite (~/.aip/aip.db)
│   ├── proxy/                   # SOCKS5/HTTP/Direct pool, round-robin, failover, curl-style tester
│   ├── ratelimit/               # Smart 429 reset time parser (headers + regex body text)
│   ├── upstream/                # Transparent reverse proxy to opencode.ai/zen/v1 (Bearer public)
│   ├── discovery/               # Manual free model fetcher (models.dev/api.json)
│   ├── opencode/                # OpenCode config sync (~/.config/opencode/opencode.json)
│   └── server/                  # HTTP router (OpenAI routes + dashboard REST API)
├── web/                         # Vite + React (TS) + Base UI (Rhea preset) + React Router
│   ├── src/
│   │   ├── components/
│   │   │   ├── app-sidebar.tsx    # Inset sidebar navigation (Overview, Integration, Proxies, Usage, Logs)
│   │   │   ├── site-header.tsx    # Header with SidebarTrigger, breadcrumb title, ModeToggle
│   │   │   ├── theme-provider.tsx # Official shadcn Vite theme provider (dark/light/system)
│   │   │   ├── mode-toggle.tsx    # Theme switch toggle component
│   │   │   └── ui/              # shadcn/ui components (Base UI + Rhea preset)
│   │   ├── pages/
│   │   │   ├── Overview.tsx     # Stats cards with jump links, model insights/trends, recent requests + "View more"
│   │   │   ├── Integration.tsx  # OpenCode link/sync, copyable snippet, model catalog, fetch button
│   │   │   ├── Proxies.tsx      # Proxy table, selectable rows for batch delete, live countdown badges, Add Drawer
│   │   │   ├── Usage.tsx        # Detailed request logs table (tokens, cache, latency) + row click inspect Drawer
│   │   │   └── Logs.tsx         # Real-time console log viewer (errors, warnings, daemon events)
│   │   ├── App.tsx              # SidebarProvider + AppSidebar (variant="inset") + SidebarInset layout
│   │   └── main.tsx
│   ├── components.json          # shadcn configuration (base-rhea preset)
│   ├── package.json
│   ├── vite.config.ts           # Development proxy (/api and /v1 -> :9100)
│   └── dist/                    # Compiled static assets (embedded into Go binary)
├── ~/.aip/                      # Runtime data (created in user's home directory)
│   ├── aip.db                   # SQLite database
│   ├── aip.pid                  # Daemon PID file
│   └── aip.log                  # Daemon stdout/stderr logs
├── Makefile                     # Build & dev shortcuts (make build, make dev)
├── PLAN.md                      # This plan
└── go.mod
```

---

## 3. Core Functional Architecture

### A. The 3-Command Global CLI
Run from any directory on your computer:
* `aip start` — Spawns the background daemon on `http://localhost:9100`, writes PID to `~/.aip/aip.pid`.
* `aip stop` — Gracefully terminates the background daemon via PID.
* `aip status` — Displays running state, PID, uptime, # active proxies

### B. Transparent Reverse Proxy Engine (`internal/upstream`)
* **Target:** `https://opencode.ai/zen/v1/...`
* **Zero Payload Tampering:** Forwards OpenCode's raw HTTP request bytes (native tools, session headers `x-opencode-session`, streaming parameters).
* **Token Swap:** Injects `Authorization: Bearer public`.
* **Zero Buffering:** Pipes raw SSE stream directly back to OpenCode CLI for instant token streaming.
* **Instant Failover:** On HTTP `429`, marks the proxy in cooldown and immediately retries the request on the next `online` proxy.

### C. Proxy Pool & Smart 429 Tracking (`internal/proxy`, `internal/ratelimit`)
* Supports: `socks5://`, `socks5h://`, `http://`, `https://`, and `direct`.
* **Flexible Input Formats & Auto-Normalization:**
  * Standard URIs: `http://user:pass@host:port`, `socks5://user:pass@host:port`, `direct`
  * Raw Vendor Formats (automatically normalized with default protocol selector, e.g., HTTP or SOCKS5):
    * `host:port:username:password` (e.g., `31.59.20.176:6754:glzkmuga:qg5wcyihw7os`)
    * `username:password@host:port`
    * `host:port` (unauthenticated)
  * Bulk import parser normalizes line-delimited proxy lists into standard URLs upon adding.
* **States:**
  * 🟢 **`online`**: Ready in round-robin pool.
  * 🟡 **`rate_limited`**: Hit 429 during real traffic. Parses `Retry-After`, `x-ratelimit-reset`, and error body text (*"resets in X hours Y minutes"*). Displays live countdown in the UI; auto-recovers when timer hits zero.
  * 🔴 **`error`**: Connection timeout or proxy offline.
* **Curl-Style Test in UI:** Fast probe against `https://cloudflare.com/cdn-cgi/trace` to measure latency (ms) and verify egress IP. Never calls OpenCode during manual tests.

### D. Manual Model Discovery (`internal/discovery`)
* Starts **completely empty** (no background polling or hidden timers).
* **Fetch button in UI:**
  1. Queries `https://models.dev/api.json` (OpenCode's official model catalog, with fallback to `https://models.opencode.ai/api.json`).
  2. Filters specifically for active free OpenCode models:
     * **Provider:** `providerID == "opencode"` (targeted at `https://opencode.ai/zen/v1`)
     * **Cost:** `cost.input == 0` AND `cost.output == 0`
     * **Status:** `status == "active"` (ignores `"deprecated"` models)
  3. Extracts complete model metadata:
     * ID & Name (e.g., `big-pickle`, `nemotron-3.5-lightning-free`)
     * Context & Output limits (e.g., 200k, 1M context)
     * Modalities (`modalities: { input: ["text", "image"], output: ["text"] }`)
  4. Saves verified free models to SQLite (`models` table).
  5. Exposes them immediately at `http://localhost:9100/v1/models` where OpenCode CLI auto-discovers them.

### E. OpenCode Integration & Model Sync (`internal/opencode`)
* **Target File:** Reads and updates `~/.config/opencode/opencode.json`.
* **Provider Definition:** Injects or updates `config.provider["aip"]`:
  * Sets `npm: "@ai-sdk/openai-compatible"`.
  * Sets `options: { baseURL: "http://localhost:9100/v1", apiKey: "public" }`.
  * Sets `models: { ... }` populated from the verified free models in SQLite.
* **Sync Strategy:**
  * **Dedicated Link / Sync Button:** On the Integration page, users can click "Link to OpenCode" or "Sync Models" at any time, plus an "Unlink" button to safely remove `provider["aip"]`.
  * **Auto-Sync on Fetch (When Linked):** Whenever the user fetches/refreshes models, if `aip` is already linked in `opencode.json`, `aip` automatically updates `provider["aip"].models` in `opencode.json` seamlessly.
  * **Manual Snippet:** A copyable JSON snippet is always available in the UI for users preferring manual setup in `opencode.jsonc`.

### F. Modern shadcn/ui Dashboard (Base UI + Rhea Preset)
* **Layout Reference (`dashboard-01` with `SidebarInset`):**
  * **Interactive Preview:** `https://ui.shadcn.com/view/new-york-v4/dashboard-01`
  * **Official Source Code:** `https://github.com/shadcn-ui/ui/tree/main/apps/v4/app/(examples)/dashboard`
  * **Architecture:** `<SidebarProvider>` wrapping `<AppSidebar variant="inset" />` and `<SidebarInset>` (containing `<SiteHeader />` and the active page view).
* **Dark / Light Mode Support (Official shadcn Vite Guide):**
  * `ThemeProvider` (`theme-provider.tsx`) with `defaultTheme="system"` and persistence in `localStorage` under `vite-ui-theme`.
  * `ModeToggle` component (`mode-toggle.tsx`) in the `SiteHeader` supporting Light, Dark, and System modes.
  * All views styled using semantic CSS variables (`bg-background`, `text-foreground`, `border-border`, etc.) ensuring flawless appearance in both themes.
* **Pages (React Router v6):**
  1. **`/` (Overview):**
     * Metric cards (Total Requests, Success Rate, Tokens, Active Proxies) with quick-navigation jump links (e.g. clicking the Active Proxies card jumps directly to `/proxies`).
     * Usage insights: Model usage trends and distribution metrics.
     * Recent requests table: Concise preview with a prominent **"View more"** link redirecting directly to `/usage`.
  2. **`/integration` (Integration & Models):**
     * OpenCode connection management: Connection status, 1-click **"Link to OpenCode"** / **"Sync Models"**, **"Unlink"**, and copyable JSON snippet for manual `.jsonc` users.
     * Active Free Models catalog: On-demand **"Fetch Models"** button (querying `models.dev/api.json` with the active free filter), context window limits, and supported modalities.
  3. **`/proxies` (Proxies):**
     * Table of proxies with live status badges (`online`, `rate_limited` with countdown timers, `error`), egress IP, and latency (ms).
     * **Selectable Rows (Checkboxes) & Batch Actions:** Enables individual or select-all checkboxes with batch actions: **"Test Selected"** (fast latency & egress probe) and **"Delete Selected"** (batch deletion). Each row also has an individual test icon.
     * **"Add Proxy" Drawer:** Slide-over Drawer supporting single proxy input and bulk text import (parsing raw `ip:port:user:pass`, `host:port`, and standard URLs with default protocol selection).
  4. **`/usage` (Usage & Request Logs):**
     * Dedicated request audit table displaying: Timestamp, Model, Status (HTTP code), Latency (ms), Input tokens, Output tokens, Cache hit rate / read, and Proxy node used.
     * **Interactive Row Inspection:** Clicking any request row opens a full detail Drawer/Sheet showing complete request/response payloads, headers, session affinity, and error details if any.
  5. **`/logs` (Console Logs):**
     * Real-time streaming system console log viewer showing daemon events, errors, warnings, proxy rotations, and model refresh logs with level filters.

---

## 4. Build & Development Workflow (Makefile)

A standardized `Makefile` automates both development and production workflows:

### Makefile Targets:
* **`make build`**: 
  1. Installs frontend dependencies if missing (`npm --prefix web install`).
  2. Compiles the Vite + React frontend into static assets in `web/dist/`.
  3. Compiles the Go binary embedding `web/dist/` into a single standalone executable `./aip`.
* **`make install`**:
  * Runs `make build` and copies the compiled `aip` binary into `$(HOME)/.local/bin/aip` so it is immediately accessible anywhere in your shell `$PATH`.
* **`make dev`**:
  * Starts the full local development environment with one command:
    * Launches the Go backend daemon on `http://localhost:9100`.
    * Launches the Vite frontend dev server on `http://localhost:5173` with instant Hot Module Replacement (HMR).
    * Vite automatically proxies `/api` and `/v1` requests to the Go backend.
* **`make clean`**:
  * Removes compiled binaries, `web/dist/`, and local test artifacts.

### Typical Developer Workflows:
1. **Developing UI & Backend:** Run `make dev`. Edit React components or Go code with live hot-reloading.
2. **Deploying / Updating Binary:** Run `make install`. Now run `aip start`, `aip status`, or `aip stop` from any directory.

---

## 5. Step-by-Step Implementation Roadmap

1. **Step 1: Go Module & Dependencies** — Initialize `go.mod` with pure-Go SQLite (`modernc.org/sqlite`) and SOCKS5 proxy dependencies.
2. **Step 2: Frontend Scaffolding** — Setup Vite + React (TypeScript) + React Router + Tailwind CSS + shadcn/ui (Base UI + Rhea preset) + Dark/Light Theme (`ThemeProvider` & `ModeToggle`).
3. **Step 3: Database Layer (`internal/db`)** — Create SQLite schema in `~/.aip/aip.db` (proxies, models, request logs).
4. **Step 4: Proxy Pool & Rate-Limit Tracking (`internal/proxy`, `internal/ratelimit`)** — Implement multi-format proxy parser (URIs, `host:port:user:pass`, `host:port`), round-robin routing, 429 reset parser, and curl-style testing.
5. **Step 5: Transparent Reverse Proxy (`internal/upstream`)** — Implement raw stream passthrough with `Bearer public` and automatic failover.
6. **Step 6: Model Discovery & OpenCode Sync (`internal/discovery`, `internal/opencode`)** — Implement on-demand free model fetcher and `~/.config/opencode/opencode.json` configuration manager.
7. **Step 7: HTTP Server (`internal/server`)** — Implement OpenAI API routes (`/v1/models`, `/v1/chat/completions`), OpenCode link/sync endpoints (`/api/opencode/*`), dashboard REST endpoints, and embedded SPA file server.
8. **Step 8: Dashboard Pages** — Implement the 5 React Router pages (Overview, Integration, Proxies, Usage, Logs) with shadcn Base UI + Rhea components.
9. **Step 9: CLI & Daemon (`cmd/aip`, `internal/daemon`)** — Implement `aip start`, `aip stop`, and `aip status`.
10. **Step 10: Verification** — Build executable, test daemon management, proxy rotation, model fetching, and OpenCode connectivity.
