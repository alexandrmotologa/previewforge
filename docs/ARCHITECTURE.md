# PreviewForge Architecture

PreviewForge is a single-binary daemon that turns GitHub pull request webhooks into isolated Docker Compose preview stacks on a single server.

```
                           GitHub Webhook Event
                                     |
                                     v
                       +---------------------------+
                       |  HMAC-SHA256 Signature    |
                       |  Verification & Filtering |
                       +-------------+-------------+
                                     |
                                     v
                       +---------------------------+
                       |    Port Ledger Service    |
                       | Allocates 31000-35000     |
                       +-------------+-------------+
                                     |
                                     v
                       +---------------------------+
                       |   Git Shallow Checkout    |
                       | Shallow clone PR ref      |
                       +-------------+-------------+
                                     |
                                     v
                       +---------------------------+
                       |   Compose Override Engine |
                       | - Dynamic project name    |
                       | - Host port offset        |
                       | - Network isolation       |
                       +-------------+-------------+
                                     |
                                     v
                       +---------------------------+
                       |    Healthcheck Poller     |
                       | Polls 127.0.0.1:{port}    |
                       +-------------+-------------+
                                     |
                      +--------------+--------------+
                      |                             |
                      v                             v
        +---------------------------+ +---------------------------+
        |  Dynamic Reverse Proxy    | |    GitHub Status & Bot    |
        | Subdomain:                | | - Sets commit status      |
        | pr-{id}.preview.domain    | | - Posts sticky PR comment |
        +---------------------------+ +---------------------------+
```

## 1. Network and Container Isolation

Running multiple preview environments on a single host requires strict separation to prevent port collisions, network crossover, and name conflicts:

1. **Project Name Namespacing**: Every preview stack uses a distinct Docker Compose project name:
   `pf_pr{pr_number}_{repo_name}` (e.g. `pf_pr42_storefront`). This guarantees that Docker Compose creates an isolated Docker network (`pf_pr42_storefront_default`) and isolated volumes for each PR.
2. **Dynamic Port Offset Generation**: Rather than exposing standard container ports (like 80 or 3000) directly to the host, PreviewForge allocates an unused host port from its ledger (`31000` to `35000`) and synthesizes a temporary `docker-compose.previewforge.override.yml`:
   ```yaml
   services:
     web:
       ports:
         - "31042:3000"
       labels:
         previewforge.managed: "true"
         previewforge.pr: "42"
       environment:
         - PREVIEW_URL=https://pr-42.preview.yourdomain.com
   ```
3. **No Cross-Stack Communication**: Containers inside PR #42 cannot communicate with or affect containers inside PR #43 because each stack runs within its own dedicated bridge network.

## 2. Dynamic Reverse Proxy Architecture

PreviewForge supports two reverse proxy modes:

### Mode A: Embedded Standalone Reverse Proxy
In standalone mode, PreviewForge acts as the primary HTTP reverse proxy. It inspects the `Host` header of all incoming requests:
- If `Host` matches a registered preview route (`pr-42.preview.yourdomain.com`), it proxies HTTP traffic and WebSocket connections directly to `127.0.0.1:31042`.
- If `Host` matches the management host or server IP, it serves the Web Studio dashboard and REST API.
- If a preview container is still booting, it returns an informative `502 Starting Up` page. If the PR was closed, it returns a `404 Inactive` page.

### Mode B: Caddy Admin API Integration
When paired with Caddy, PreviewForge pushes dynamic route definitions to Caddy's JSON Admin API (`http://127.0.0.1:2019/config/apps/http/servers/previewforge/routes`). Caddy automatically handles wildcard TLS certificate issuance via Let's Encrypt or ZeroSSL.

## 3. Persistent State Machine

All daemon state is stored in an embedded SQLite database using pure-Go drivers, requiring no external database or CGO compiler:
- `deployments`: Records pull request metadata, branch, commit SHA, allocated host port, preview URL, build duration, and health status.
- `port_allocations`: Tracks in-use and free host ports across daemon reboots to prevent port recycling conflicts.
- `deployment_logs`: Retains historical stdout/stderr lines for inspection from the web dashboard or API.

## 4. Teardown and Resource Reclamation

When a pull request webhook arrives with action `closed` or `unlabeled`:
1. The reverse proxy route is removed from the routing table.
2. `docker compose down -v --rmi local` runs to stop containers, delete PR-specific bridge networks, remove ephemeral volumes, and purge locally built Docker images.
3. The workspace directory on disk is removed.
4. The allocated port is marked free in the SQLite ledger.
5. A background reaper runs every 10 minutes to clean up preview stacks exceeding the configured idle timeout.
