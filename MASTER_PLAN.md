# PreviewForge: Master Implementation Plan

Self-hosted ephemeral pull request preview supervisor and reverse proxy daemon. Runs on a single Linux or VPS server ($5-$10/month), receives GitHub webhooks, builds Docker Compose preview stacks in isolated networks with dynamic port multiplexing, configures automatic HTTPS subdomains through Caddy or the built-in proxy, and publishes sticky status comments on pull requests. On merge or close, it halts containers and reclaims disk space.

---

## 1. Architecture and Data Flow

```
                                GitHub Repository
                                       |
                   Webhook: pull_request (opened, sync, closed)
                                       v
                     +-----------------------------------+
                     |      PreviewForge Supervisor      |
                     |             (Port 9850)           |
                     +-----------------+-----------------+
                                       |
         +-----------------------------+-----------------------------+
         |                             |                             |
         v                             v                             v
+------------------+         +-------------------+         +-------------------+
| Webhook Receiver |         | Storage & State   |         | Web Studio & API  |
| HMAC SHA-256     |         | Pure-Go SQLite    |         | React 19 (embed)  |
| Fork Security    |         | Port Ledger       |         | SSE Live Logs     |
+--------+---------+         +---------+---------+         +---------+---------+
         |                             |                             |
         +-----------------------------+-----------------------------+
                                       |
                                       v
                     +-----------------------------------+
                     |   Docker Compose Orchestrator     |
                     |  - Shallow git clone / pull       |
                     |  - Port allocation (30000-35000)  |
                     |  - Dynamic project isolation      |
                     |  - Compose override injection     |
                     |  - Healthcheck readiness poll     |
                     +-----------------+-----------------+
                                       |
         +-----------------------------+-----------------------------+
         |                                                           |
         v                                                           v
+-----------------------------------+               +-----------------------------------+
| Reverse Proxy & TLS Engine        |               | GitHub Status & Sticky Comments   |
| Mode A: Caddy Admin API (2019)    |               | - Commit Status (pending/success) |
| Mode B: Built-in Go Reverse Proxy |               | - Sticky PR Markdown Comment      |
| Host: pr-{id}.preview.domain.com  |               | - Build duration & direct link    |
+-----------------------------------+               +-----------------------------------+
```

---

## 2. Directory Structure

```
previewforge/
├── cmd/
│   └── previewforge/
│       └── main.go                 # CLI entrypoint (serve, list, destroy, prune, version)
├── internal/
│   ├── config/
│   │   └── config.go               # Environment and flag configuration parser
│   ├── model/
│   │   └── models.go               # Domain entities (Deployment, Project, PortMapping, SystemStats)
│   ├── storage/
│   │   ├── sqlite.go               # Pure-Go SQLite database engine and schema migrations
│   │   └── port_store.go           # Persistent port allocation ledger
│   ├── webhook/
│   │   ├── handler.go              # GitHub webhook HTTP handler with HMAC verification
│   │   ├── payload.go              # Pull request webhook payload parser
│   │   └── security.go             # Untrusted fork policy and signature validator
│   ├── orchestrator/
│   │   ├── runner.go               # Deployment workflow coordinator
│   │   ├── git.go                  # Shallow git clone and PR ref checkout
│   │   ├── port_allocator.go       # Concurrent port pool allocator (30000-35000)
│   │   ├── compose_builder.go      # Compose inspector and override generator
│   │   ├── docker_client.go        # Docker CLI and SDK abstraction runner
│   │   ├── healthcheck.go          # HTTP/TCP readiness poller with backoff
│   │   └── reaper.go               # TTL reaper and disk volume garbage collector
│   ├── proxy/
│   │   ├── router.go               # Reverse proxy router interface
│   │   ├── caddy_manager.go        # Caddy Admin API client (JSON config push)
│   │   └── standalone_proxy.go     # Embedded Go reverse proxy with host routing
│   ├── github/
│   │   ├── client.go               # GitHub REST API client (PAT or GitHub App)
│   │   ├── status.go               # Commit status check reporter
│   │   └── commenter.go            # Sticky PR comment publisher and editor
│   └── server/
│       ├── server.go               # HTTP server setup (API + Webhook + Frontend)
│       ├── handlers.go             # REST API endpoints for studio dashboard
│       ├── sse.go                  # Server-Sent Events broker for real-time logs
│       └── static.go               # Embedded React 19 filesystem handler
├── web/
│   ├── index.html                  # HTML entrypoint
│   ├── package.json                # React 19, Lucide, Tailwind, Vite
│   ├── vite.config.ts              # Vite configuration with embedded output
│   ├── tailwind.config.js          # Clean editorial styling
│   └── src/
│       ├── main.tsx
│       ├── App.tsx                 # Main layout and tab router
│       ├── types.ts                # TypeScript interfaces
│       ├── components/
│       │   ├── Header.tsx          # Daemon status and global metrics
│       │   ├── EnvironmentGrid.tsx # Active preview stacks and quick actions
│       │   ├── EnvironmentCard.tsx # Individual PR environment card
│       │   ├── LogViewer.tsx       # Live terminal log viewer with SSE
│       │   ├── ResourceGauges.tsx  # Host memory, CPU, and disk metrics
│       │   ├── WebhookSimulator.tsx# Manual test deployment trigger
│       │   └── SettingsModal.tsx   # Domain and proxy configuration
│       └── hooks/
│           ├── useDeployments.ts   # React hook for deployment state polling
│           └── useLiveLogs.ts      # React hook for SSE stream
├── docs/
│   ├── ARCHITECTURE.md             # Deep dive into container isolation and routing
│   ├── CONFIGURATION.md            # Configuration reference and flags
│   └── CADDY_INTEGRATION.md        # Caddyfile setup and wildcard DNS guide
├── go.mod
├── go.sum
├── Dockerfile                      # Production Docker container for PreviewForge
├── docker-compose.yml              # Single-node deployment stack
├── .gitignore
└── README.md                       # Comprehensive user guide and documentation
```

---

## 3. Implementation Phases

### Phase 1: Foundation, Data Models, and Storage
- Initialize Go module `github.com/alexandrmotologa/previewforge`.
- Create domain models in `internal/model/models.go`:
  - `Deployment`: ID, PRNumber, RepoOwner, RepoName, Branch, CommitSHA, Status, Port, Subdomain, Logs, CreatedAt, UpdatedAt.
  - `PortAllocation`: Port, DeploymentID, AllocatedAt, InUse.
  - `SystemConfig`: BaseDomain, PublicPort, ProxyMode, PortRangeStart, PortRangeEnd, IdleTimeout.
- Implement pure-Go SQLite storage engine in `internal/storage/sqlite.go`:
  - Auto-migration of tables (`deployments`, `port_allocations`, `system_logs`).
  - CRUD operations with transaction safety.
- Write unit tests for storage operations in `internal/storage/sqlite_test.go`.

### Phase 2: GitHub Webhooks and Signature Verification
- Implement `internal/webhook/security.go`:
  - HMAC-SHA256 signature verification matching `X-Hub-Signature-256`.
  - Constant-time comparison to prevent timing attacks.
  - Fork access policy (allow all, allow members, or require manual approval).
- Implement `internal/webhook/handler.go` and `internal/webhook/payload.go`:
  - Support `pull_request` events: `opened`, `synchronize`, `reopened`, `closed`.
  - Support `ping` event for webhook setup tests.
  - Queue deployment or teardown jobs asynchronously.
- Write unit tests verifying payload extraction and signature security.

### Phase 3: Docker Compose Orchestrator and Dynamic Ports
- Implement `internal/orchestrator/port_allocator.go`:
  - Thread-safe port manager across designated range (e.g. 30000-35000).
  - Port ledger saved in SQLite to survive daemon restarts.
- Implement `internal/orchestrator/git.go`:
  - Shallow clone PR branch into `/var/previewforge/workspaces/pr-{id}`.
- Implement `internal/orchestrator/compose_builder.go`:
  - Detect `docker-compose.yml` or repo configuration `.previewforge.yml`.
  - Generate isolated project name `previewforge_pr_{id}_{repo}`.
  - Create dynamic override file mapping primary web service to allocated host port.
- Implement `internal/orchestrator/docker_client.go`:
  - Execute `docker compose up -d --build` capturing stdout and stderr.
  - Stream logs to in-memory broadcast channel and persistent log file.
- Implement `internal/orchestrator/healthcheck.go`:
  - Poll HTTP endpoint `http://127.0.0.1:{port}` with timeout and exponential backoff.
- Implement `internal/orchestrator/reaper.go`:
  - Teardown routine: `docker compose down -v --rmi local` and workspace directory deletion.
  - Idle stack sweeper based on TTL.

### Phase 4: Dynamic Reverse Proxy and Routing
- Implement `internal/proxy/router.go`:
  - Unified interface `ProxyRouter` with `RegisterRoute(subdomain, targetPort)` and `RemoveRoute(subdomain)`.
- Implement `internal/proxy/standalone_proxy.go`:
  - Embedded Go HTTP reverse proxy with host-based dynamic routing.
  - WebSocket upgrade support.
  - Dynamic local/staging routing with fallback error pages.
- Implement `internal/proxy/caddy_manager.go`:
  - Communicate with Caddy Admin API (`POST http://localhost:2019/load`).
  - Push upstream route definitions for `pr-{id}.preview.domain.com`.

### Phase 5: GitHub Bot and Sticky PR Comments
- Implement `internal/github/client.go`:
  - GitHub REST API wrapper using personal access token or installation token.
- Implement `internal/github/status.go`:
  - Post commit status checks (`previewforge/deploy`): `pending`, `success`, `failure`.
- Implement `internal/github/commenter.go`:
  - Query PR comments for existing `<!-- previewforge:status -->` identifier.
  - Update existing comment in place or post a new sticky comment.
  - Include status indicator, live subdomain URL, build duration, commit SHA, and direct dashboard link.

### Phase 6: REST API and Server-Sent Events (SSE)
- Implement `internal/server/server.go` and `internal/server/handlers.go`:
  - `GET /api/deployments` (list all environments)
  - `GET /api/deployments/{id}` (single environment details)
  - `POST /api/deployments/{id}/rebuild` (trigger manual rebuild)
  - `DELETE /api/deployments/{id}` (manual teardown)
  - `GET /api/system/stats` (CPU, memory, disk, port pool usage)
  - `GET /api/system/config` & `POST /api/system/config`
  - `POST /api/simulator/deploy` (trigger test deployment without real GitHub webhook)
- Implement `internal/server/sse.go`:
  - `GET /api/deployments/{id}/logs/stream` (real-time build log streaming)

### Phase 7: Web Studio Frontend (React 19 + TypeScript + Tailwind)
- Scaffold React 19 app in `web/` using Vite and Tailwind CSS.
- Build components:
  - Header with daemon uptime, active environment count, and resource gauge.
  - EnvironmentGrid and EnvironmentCard with live status badges.
  - LogViewer with real-time SSE streaming, auto-scroll, search, and copy.
  - ResourceGauges showing host RAM, CPU, and disk utilization.
  - WebhookSimulator for local testing.
  - Settings modal for domain, port range, and webhook secret.
- Compile production build into `web/dist` and embed into Go binary via `embed.FS`.

### Phase 8: CLI Commands and Production Packaging
- Implement CLI in `cmd/previewforge/main.go`:
  - `previewforge serve`
  - `previewforge list`
  - `previewforge destroy <pr-id>`
  - `previewforge prune`
  - `previewforge version`
- Provide `Dockerfile`, `docker-compose.yml`, and systemd service templates.
- Write full test suite verifying all components.
- Write documentation: `README.md`, `ARCHITECTURE.md`, `CONFIGURATION.md`, `CADDY_INTEGRATION.md`.
