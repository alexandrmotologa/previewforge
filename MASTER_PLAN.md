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
                     |  - Scale-to-zero hibernation      |
                     |  - Disk space guard auto-pruner   |
                     +-----------------+-----------------+
                                       |
         +-----------------------------+-----------------------------+
         |                                                           |
         v                                                           v
+-----------------------------------+               +-----------------------------------+
| Reverse Proxy & TLS Engine        |               | GitHub Status & Sticky Comments   |
| Mode A: Caddy Admin API (2019)    |               | - Commit Status (pending/success) |
| Mode B: Built-in Go Reverse Proxy |               | - Native GitHub Deployments API   |
| Host: pr-{id}.preview.domain.com  |               | - Sticky PR Markdown Comment      |
| Auto-Wake Interstitial Screen     |               | - Multi-channel alerts (Discord)  |
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
│   │   └── models.go               # Domain entities (Deployment, PortAllocation, SystemStats, SystemConfig)
│   ├── storage/
│   │   ├── sqlite.go               # Pure-Go SQLite database engine and schema migrations
│   │   └── sqlite_test.go          # Transactional unit tests
│   ├── webhook/
│   │   ├── handler.go              # GitHub webhook HTTP handler with HMAC verification
│   │   ├── handler_test.go         # Webhook receiver test suite
│   │   ├── payload.go              # Pull request webhook payload parser
│   │   └── security.go             # Untrusted fork policy and signature validator
│   ├── orchestrator/
│   │   ├── runner.go               # Deployment workflow coordinator (deploy, teardown, sleep, wake, exec)
│   │   ├── runner_test.go          # Orchestrator unit tests
│   │   ├── git.go                  # Shallow git clone and PR ref checkout with GitFetcher interface
│   │   ├── port_allocator.go       # Concurrent port pool allocator (30000-35000)
│   │   ├── compose_builder.go      # Compose inspector, override generator, and hook extractor
│   │   ├── compose_builder_test.go # Compose builder test suite
│   │   ├── docker_client.go        # Docker CLI runner (up, down, stop, start, exec, prune, stats)
│   │   ├── healthcheck.go          # HTTP/TCP readiness poller with backoff
│   │   ├── broadcaster.go          # SSE log broker
│   │   ├── disk.go                 # Host disk space checker abstraction
│   │   ├── disk_windows.go         # Windows disk space implementation
│   │   ├── disk_unix.go            # Linux/Unix disk space implementation
│   │   └── reaper.go               # Idle sweeper, auto-sleep hibernation, and disk space guard
│   ├── proxy/
│   │   ├── router.go               # Reverse proxy router interface
│   │   ├── caddy_manager.go        # Caddy Admin API client (JSON config push)
│   │   ├── standalone_proxy.go     # Embedded Go reverse proxy with host routing, noindex, auth, and wake screen
│   │   └── proxy_test.go           # Proxy unit tests
│   ├── github/
│   │   ├── client.go               # GitHub REST API client (Deployments, statuses, comments)
│   │   ├── commenter.go            # Sticky PR comment publisher and editor
│   │   └── commenter_test.go       # GitHub integration tests
│   ├── notification/
│   │   ├── notifier.go             # Discord and Slack webhook broadcaster
│   │   └── notifier_test.go        # Notification test suite
│   └── server/
│       ├── server.go               # HTTP server setup (API + Webhook + Frontend + Proxy Host router)
│       ├── server_test.go          # Server test suite
│       ├── handlers.go             # REST API endpoints (deployments, sleep, wake, exec, simulator, stats)
│       ├── sse.go                  # Server-Sent Events broker for real-time logs
│       └── static.go               # Embedded React 19 filesystem handler
├── web/
│   ├── index.html                  # HTML entrypoint
│   ├── package.json                # React 19, Lucide, Tailwind, Vite
│   ├── vite.config.ts              # Vite configuration with embedded output
│   ├── tailwind.config.js          # Clean editorial styling
│   ├── embed.go                    # Go embed directive for dist bundle
│   └── src/
│       ├── main.tsx
│       ├── App.tsx                 # Main layout and tab router
│       ├── types.ts                # TypeScript interfaces
│       └── components/
│           ├── Header.tsx          # Daemon status and global metrics
│           ├── EnvironmentGrid.tsx # Active preview stacks and quick actions
│           ├── EnvironmentCard.tsx # Individual PR environment card (CPU/RAM, sleep, wake, exec)
│           ├── LogDrawer.tsx       # Live terminal log viewer with SSE and container exec console
│           ├── MetricsOverview.tsx # Host memory, CPU, sleeping counts, and disk utilization
│           ├── SimulatorModal.tsx  # Manual test deployment trigger
│           └── SettingsModal.tsx   # Domain and proxy configuration
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

### Phase 1: Foundation, Data Models, and Storage [COMPLETED]
- [x] Initialize Go module `github.com/alexandrmotologa/previewforge`.
- [x] Create domain models in `internal/model/models.go`:
  - `Deployment`: ID, PRNumber, RepoOwner, RepoName, Branch, CommitSHA, Status, Port, Subdomain, Logs, CreatedAt, UpdatedAt, Hibernation fields, CPU/RAM metrics.
  - `PortAllocation`: Port, DeploymentID, AllocatedAt, InUse.
  - `SystemConfig`: BaseDomain, PublicPort, ProxyMode, PortRangeStart, PortRangeEnd, IdleTimeout.
- [x] Implement pure-Go SQLite storage engine in `internal/storage/sqlite.go`:
  - Auto-migration of tables (`deployments`, `port_allocations`, `system_logs`).
  - CRUD operations with transaction safety.
- [x] Write unit tests for storage operations in `internal/storage/sqlite_test.go`.

### Phase 2: GitHub Webhooks and Signature Verification [COMPLETED]
- [x] Implement `internal/webhook/security.go`:
  - HMAC-SHA256 signature verification matching `X-Hub-Signature-256`.
  - Constant-time comparison to prevent timing attacks.
  - Fork access policy (allow all, allow members, or require manual approval).
- [x] Implement `internal/webhook/handler.go` and `internal/webhook/payload.go`:
  - Support `pull_request` events: `opened`, `synchronize`, `reopened`, `closed`.
  - Support `ping` event for webhook setup tests.
  - Queue deployment or teardown jobs asynchronously.
- [x] Write unit tests verifying payload extraction and signature security in `internal/webhook/handler_test.go`.

### Phase 3: Docker Compose Orchestrator and Dynamic Ports [COMPLETED]
- [x] Implement `internal/orchestrator/port_allocator.go`:
  - Thread-safe port manager across designated range (30000-35000).
  - Port ledger saved in SQLite to survive daemon restarts.
- [x] Implement `internal/orchestrator/git.go`:
  - Shallow clone PR branch into workspaces with `GitFetcher` interface.
- [x] Implement `internal/orchestrator/compose_builder.go`:
  - Detect `docker-compose.yml` or repo configuration `.previewforge.yml`.
  - Generate isolated project name `pf_pr{id}_{repo}`.
  - Create dynamic override file mapping primary web service to allocated host port.
- [x] Implement `internal/orchestrator/docker_client.go`:
  - Execute `docker compose up -d --build` capturing stdout and stderr.
  - Stream logs to in-memory broadcast channel and persistent log file.
- [x] Implement `internal/orchestrator/healthcheck.go`:
  - Poll HTTP endpoint `http://127.0.0.1:{port}` with timeout and exponential backoff.
- [x] Implement `internal/orchestrator/reaper.go`:
  - Teardown routine: `docker compose down -v --rmi local` and workspace directory deletion.
  - Idle stack sweeper based on TTL.

### Phase 4: Dynamic Reverse Proxy and Routing [COMPLETED]
- [x] Implement `internal/proxy/router.go`:
  - Unified interface `ProxyRouter` with `RegisterRoute(subdomain, targetPort)` and `RemoveRoute(subdomain)`.
- [x] Implement `internal/proxy/standalone_proxy.go`:
  - Embedded Go HTTP reverse proxy with host-based dynamic routing.
  - WebSocket upgrade support.
  - Dynamic local/staging routing with fallback error pages.
- [x] Implement `internal/proxy/caddy_manager.go`:
  - Communicate with Caddy Admin API (`POST http://localhost:2019/load`).
  - Push upstream route definitions for `pr-{id}.preview.domain.com`.

### Phase 5: GitHub Bot and Sticky PR Comments [COMPLETED]
- [x] Implement `internal/github/client.go`:
  - GitHub REST API wrapper using personal access token or installation token.
- [x] Implement `internal/github/commenter.go`:
  - Query PR comments for existing `<!-- previewforge:status -->` identifier.
  - Update existing comment in place or post a new sticky comment.
  - Include status indicator, live subdomain URL, build duration, commit SHA, and direct dashboard link.

### Phase 6: REST API and Server-Sent Events (SSE) [COMPLETED]
- [x] Implement `internal/server/server.go` and `internal/server/handlers.go`:
  - `GET /api/deployments` (list all environments)
  - `GET /api/deployments/{id}` (single environment details)
  - `POST /api/deployments/{id}/rebuild` (trigger manual rebuild)
  - `DELETE /api/deployments/{id}` (manual teardown)
  - `GET /api/system/stats` (CPU, memory, disk, port pool usage)
  - `GET /api/system/config` & `POST /api/system/config`
  - `POST /api/simulator/deploy` (trigger test deployment without real GitHub webhook)
- [x] Implement `internal/server/sse.go`:
  - `GET /api/deployments/{id}/logs/stream` (real-time build log streaming)

### Phase 7: Web Studio Frontend (React 19 + TypeScript + Tailwind) [COMPLETED]
- [x] Scaffold React 19 app in `web/` using Vite and Tailwind CSS.
- [x] Build components:
  - Header with daemon uptime, active environment count, and resource gauge.
  - EnvironmentGrid and EnvironmentCard with live status badges.
  - LogDrawer with real-time SSE streaming, auto-scroll, search, and container console.
  - MetricsOverview showing host RAM, CPU, sleeping counts, and disk utilization.
  - SimulatorModal for local testing.
  - SettingsModal for domain, port range, and webhook secret.
- [x] Compile production build into `web/dist` and embed into Go binary via `embed.FS`.

### Phase 8: CLI Commands and Production Packaging [COMPLETED]
- [x] Implement CLI in `cmd/previewforge/main.go`:
  - `previewforge serve`
  - `previewforge list`
  - `previewforge destroy <pr-id>`
  - `previewforge prune`
  - `previewforge version`
- [x] Provide `Dockerfile`, `docker-compose.yml`, and systemd service templates.
- [x] Write full test suite verifying all components.
- [x] Write documentation: `README.md`, `ARCHITECTURE.md`, `CONFIGURATION.md`, `CADDY_INTEGRATION.md`.

### Phase 9: Enterprise Upgrades and Reliability Enhancements [COMPLETED]
- [x] **Auto-Sleep and Scale-to-Zero Hibernation**:
  - Automatically pauses inactive containers after configurable idle timeout (e.g. 60m).
  - Preserves container state and port reservations while freeing VPS memory and CPU.
- [x] **Wake-on-Request Reverse Proxy Interstitial**:
  - Intercepts incoming HTTP requests to sleeping preview environments.
  - Renders a clean auto-reloading waking screen while spinning up the Docker stack in the background.
- [x] **Anti-SEO Search Engine Indexing Guard**:
  - Injects `X-Robots-Tag: noindex, nofollow, noarchive, nosnippet` on all proxied preview responses to protect production SEO rankings.
- [x] **Ephemeral Basic Authentication and Preview Key Bypass**:
  - Optional password protection for staging environments with URL bypass token support (`?preview_key=token`).
- [x] **Post-Deploy Migration Hooks**:
  - Executes repo-level post-deploy commands (e.g., database seeds, prisma migrations) configured via `.previewforge.yml`.
- [x] **GitHub Native Deployments API**:
  - Integrates GitHub Deployments API (`in_progress`, `success`, `failure`, `inactive`) alongside commit statuses and sticky PR comments.
- [x] **Multi-Channel Webhook Notifications**:
  - Native Discord and Slack webhook broadcast support for ready, failed, and destroyed events.
- [x] **Disk Space Guard and Auto-Pruning**:
  - Cross-platform host disk space checker that proactively initiates Docker prune when host disk space falls below safety thresholds.
- [x] **Interactive Web Studio Container Exec Console**:
  - Embedded web terminal in the Web Studio to run ad-hoc commands inside live containers directly from the dashboard.
