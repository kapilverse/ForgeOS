# ForgeOS

**Self-hosted Platform-as-a-Service. Push code, get a URL.**

ForgeOS is a lightweight PaaS that runs on your own infrastructure.
Deploy any Docker image (and later, any Git repo) — ForgeOS handles
routing, scaling, and monitoring.

## What's Implemented (Sprint 1-2)

This increment delivers the **complete runnable foundation**:

- ✅ PostgreSQL-backed API server (Go / chi)
- ✅ JWT + API-key authentication
- ✅ App CRUD (create, list, update, delete)
- ✅ Deploy a Docker image → containers on shared network
- ✅ Traefik reverse proxy with auto-discovery (by `Host` header)
- ✅ Apps accessible at `<slug>.forgeos.local`
- ✅ App lifecycle: start, stop, restart, scale
- ✅ Deployment history with version tracking
- ✅ Embedded database migrations (no external migrate CLI needed)

## What's Deferred (future sprints)

These are explicitly **not** in this increment; they arrive as planned in
the 12-week roadmap:

- 🔜 Git-clone + auto-Dockerfile build pipeline (Week 3)
- 🔜 Zero-downtime health-gated rollout + rollback (Week 4)
- 🔜 Environment variable encryption at rest (Week 5)
- 🔜 Custom domains + Let's Encrypt SSL (Week 5)
- 🔜 Real-time log streaming via WebSocket (Week 6)
- 🔜 CPU/memory/network metrics collection (Week 7)
- 🔜 React dashboard UI (Week 8)
- 🔜 CLI tool (`forge`) (Week 9)
- 🔜 GitHub webhooks for auto-deploy (Week 10)

## Quick Start

### Prerequisites

- Go 1.22+
- Docker + Docker Compose
- (Windows) or any Unix host

### 1. Start infrastructure

```bash
docker compose up -d
```

This starts **Postgres** (`:5432`) and **Traefik** (`:80` for app traffic, `:8080` for the Traefik dashboard).

### 2. Configure

```bash
cp .env.example .env
# Edit .env — at minimum set JWT_SECRET to a random string
```

### 3. Run the API server

```bash
go run ./cmd/server
```

Migrations run automatically on startup. The API listens on `:8081`.

### 4. Smoke test

```bash
# Health check
curl http://localhost:8081/api/v1/system/health

# Register
curl -X POST http://localhost:8081/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"mypassword123","name":"You"}'

# Login (save the token)
TOKEN=$(curl -s -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"mypassword123"}' | jq -r .token)

# Create an app
curl -X POST http://localhost:8081/api/v1/apps \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"my-app","docker_image":"traefik/whoami"}'

# Deploy it
APP_ID=$(curl -s http://localhost:8081/api/v1/apps \
  -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')

curl -X POST http://localhost:8081/api/v1/apps/$APP_ID/deploy \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"image":"traefik/whoami"}'

# Test routing (no hosts-file edit needed!)
curl -H "Host: my-app.forgeos.local" http://localhost/
```

### 5. Browser access (Windows caveat)

On Windows the hosts file doesn't support wildcards, so you need a per-subdomain entry:

```
127.0.0.1  my-app.forgeos.local
```

Add the line to `C:\Windows\System32\drivers\etc\hosts` (run editor as admin), then visit `http://my-app.forgeos.local`.

On Linux/macOS you can add a single wildcard DNS resolver instead.

## Architecture

```
                    ┌──────────────┐
                    │   Internet    │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   Traefik     │  :80  (Host-based routing)
                    └──────┬───────┘
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
   ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
   │ Container 1 │ │ Container 2 │ │ Container 3 │   (user app replicas)
   └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
          │               │               │
   ┌──────┴───────────────┴───────────────┴──────┐
   │          ForgeOS Control Plane               │
   │  ┌────────┐ ┌──────────┐ ┌────────────────┐  │
   │  │ API    │ │ Deployer │ │ Container Mgr  │  │
   │  │ :8081  │ │          │ │ (Docker SDK)   │  │
   │  └───┬────┘ └────┬─────┘ └───────────────┘  │
   │      └───────────┤                         │
   │        ┌──────────▼──┐                     │
   │        │  PostgreSQL  │                    │
   │        └─────────────┘                     │
   └────────────────────────────────────────────┘
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/system/health` | No | Health check |
| `POST` | `/api/v1/auth/register` | No | Create account |
| `POST` | `/api/v1/auth/login` | No | Login (returns JWT) |
| `GET` | `/api/v1/auth/me` | Yes | Current user info |
| `POST` | `/api/v1/auth/regenerate-key` | Yes | Rotate API key |
| `GET` | `/api/v1/apps` | Yes | List apps |
| `POST` | `/api/v1/apps` | Yes | Create app |
| `GET` | `/api/v1/apps/{id}` | Yes | App details |
| `PATCH` | `/api/v1/apps/{id}` | Yes | Update app |
| `DELETE` | `/api/v1/apps/{id}` | Yes | Delete app |
| `POST` | `/api/v1/apps/{id}/stop` | Yes | Stop app |
| `PATCH` | `/api/v1/apps/{id}/scale` | Yes | Scale replicas |
| `POST` | `/api/v1/apps/{id}/deploy` | Yes | Deploy image |
| `GET` | `/api/v1/apps/{id}/deployments` | Yes | Deployment history |

## Tech Stack

- **Go** — control plane API, container manager, deployer
- **chi** — HTTP router + middleware
- **PostgreSQL** — persistent state (apps, deployments, containers)
- **pgx** — native Postgres driver (no ORM, typed queries)
- **Docker SDK** — container lifecycle management
- **Traefik v3** — reverse proxy with Docker auto-discovery
- **JWT** — auth tokens for dashboard; API keys for CLI

## Project Structure

```
cmd/server/main.go          # Entry point
internal/
  config/config.go         # Env-based configuration
  models/                   # DB row structs + request/response DTOs
  store/                    # Postgres persistence (pgx)
  container/manager.go      # Docker SDK wrapper
  router/traefik.go         # Traefik label generation
  deployer/deployer.go      # Deploy-from-image orchestration
  server/
    server.go               # Chi router wiring
    middleware/              # Auth, CORS, logging
    handlers/                # HTTP handlers
migrations/                 # Embedded SQL migrations
docker-compose.yml          # Postgres + Traefik dev stack
```
