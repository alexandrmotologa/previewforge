# PreviewForge

Self-hosted ephemeral pull request preview supervisor and reverse proxy daemon. Runs on a single Linux or VPS server ($5 to $10/month), receives GitHub webhooks, builds Docker Compose stacks in isolated networks with dynamic port multiplexing, configures HTTPS subdomains via Caddy or the built-in proxy, and publishes sticky status comments on pull requests. On merge or close, it halts containers and reclaims disk space.

```
+-----------------------------------------------------------------------------------+
|                                   PREVIEWFORGE                                    |
+-----------------------------------------+-----------------------------------------+
                                          |
        +---------------------------------+---------------------------------+
        |                                                                   |
+-------v-------------------------+                 +-----------------------v-------+
|    GitHub Webhook Ingestion     |                 |   Docker Compose Orchestrator |
|  * pull_request.opened / sync   |                 |  * Isolated Docker Networks   |
|  * pull_request.closed (clean)  |                 |  * Dynamic Port Multiplexing  |
|  * Webhook Secret Verification  |                 |  * Volume & Image Pruner      |
+-------+-------------------------+                 +-----------------------+-------+
        |                                                                   |
        +---------------------------------+---------------------------------+
                                          |
                        +-----------------v-----------------+
                        |   Dynamic Reverse Proxy & SSL     |
                        |  * Embedded Caddy / ACME Router   |
                        |  * pr-{id}.preview.domain.com     |
                        |  * Automatic Let's Encrypt TLS    |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |    GitHub PR Comment Publisher    |
                        |  * Octokit Deployment Comments    |
                        |  * Build Log Link & Live URL      |
                        |  * Commit SHA Verification Status |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |   Interactive Web Studio (:9850)  |
                        |  * Active Environments Matrix     |
                        |  * Per-Container RAM / CPU Gauges |
                        |  * 1-Click Manual Teardown        |
                        +-----------------------------------+
```

---

## Key Capabilities

- **Single-Binary Deployment**: Shipped as a self-contained Go binary with an embedded React 19 web dashboard and pure-Go SQLite storage engine. No external database or runtime dependencies required.
- **Automated Pull Request Lifecycle**:
  - `pull_request.opened` / `synchronize`: Clones the PR branch, parses `docker-compose.yml`, reserves an unused host port, builds the container stack, and routes an isolated subdomain.
  - `pull_request.closed`: Stops containers, removes networks and ephemeral volumes, prunes locally built images, and frees the allocated host port.
- **Dynamic Port Multiplexing**: Allocates host ports across a designated range (default `31000` to `35000`) with persistent tracking in SQLite to prevent port collisions across daemon restarts.
- **Dual Reverse Proxy Support**:
  - Standalone mode: Built-in Go HTTP reverse proxy with host-based dynamic routing, WebSocket upgrade support, and custom error handling.
  - Caddy mode: Directly manages Caddy reverse proxy routes through the Caddy Admin JSON API (`http://127.0.0.1:2019`).
- **Sticky GitHub PR Comments**: Updates a single markdown comment on each pull request discussion thread with commit SHA, build duration, and live URL, avoiding notification spam.
- **Interactive Web Studio**: Real-time browser dashboard displaying active preview environments, CPU and memory gauges, live Server-Sent Events (SSE) build logs, and manual teardown controls.
- **Automated Resource Reaper**: Periodic garbage collection loop that terminates abandoned preview stacks exceeding a configurable idle timeout (default 24 hours).

---

## Quick Start

### 1. Run with Docker Compose

Create a `docker-compose.yml` on your server:

```yaml
services:
  previewforge:
    image: ghcr.io/alexandrmotologa/previewforge:latest
    container_name: previewforge
    restart: unless-stopped
    ports:
      - "9850:9850"
      - "31000-32000:31000-32000"
    environment:
      - PREVIEWFORGE_DOMAIN=preview.yourdomain.com
      - GITHUB_TOKEN=ghp_yourpersonalaccesstoken
      - WEBHOOK_SECRET=your_webhook_hmac_secret
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/data
```

Start the daemon:

```bash
docker compose up -d
```

### 2. Run Directly from Binary

Download the compiled binary and start the supervisor:

```bash
./previewforge serve \
  --domain preview.yourdomain.com \
  --port 9850 \
  --github-token ghp_yourpersonalaccesstoken \
  --webhook-secret your_webhook_hmac_secret
```

Access the Web Studio dashboard at `http://localhost:9850`.

---

## CLI Reference

PreviewForge provides management subcommands:

```bash
# Start the supervisor daemon and web studio
previewforge serve --domain preview.yourdomain.com --port 9850

# List active preview environments
previewforge list

# Destroy a specific preview environment and clean up containers
previewforge destroy pr-42-storefront

# Prune orphaned workspaces on disk
previewforge prune

# Print application version
previewforge version
```

---

## Pull Request Lifecycle

```
Developer pushes to PR branch
  │
  ▼
GitHub sends webhook to /webhook
  │
  ▼
PreviewForge verifies HMAC signature
  │
  ▼
Allocates free host port (e.g. 31042)
  │
  ▼
Shallow clones PR commit into ephemeral workspace
  │
  ▼
Synthesizes docker-compose.previewforge.override.yml
  │
  ▼
Executes docker compose up -d --build
  │
  ▼
Polls http://127.0.0.1:31042/ for readiness
  │
  ▼
Registers route pr-42.preview.yourdomain.com -> 127.0.0.1:31042
  │
  ▼
Posts sticky comment with live link on GitHub PR
```

---

## In-Repository Configuration (`.previewforge.yml`)

Add an optional `.previewforge.yml` file to the root of your repository to override defaults:

```yaml
# Compose file to execute (default: docker-compose.yml)
compose_file: "docker-compose.yml"

# Primary service to expose via the preview subdomain
service: "web"

# Container internal port (default: 3000)
port: 3000

# Path polled during healthcheck verification (default: /)
health_path: "/api/health"

# Custom environment variables injected into preview containers
env:
  NODE_ENV: "production"
  DATABASE_URL: "postgres://postgres:secret@db:5432/app"
```

---

## Architecture and Documentation

- [Architecture Deep Dive](docs/ARCHITECTURE.md): Container isolation, port ledger, and proxy design.
- [Configuration Guide](docs/CONFIGURATION.md): Complete list of environment variables, CLI flags, and webhook setup.
- [Caddy and Wildcard TLS](docs/CADDY_INTEGRATION.md): Wildcard DNS, Cloudflare DNS challenge, and Caddy Admin API.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
