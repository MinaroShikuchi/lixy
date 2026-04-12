# CLAUDE.md

## Project Overview

Lixy is a GitOps-inspired controller-agent system for managing LXC container deployments in Proxmox environments. It uses Docker Compose as the deployment specification with Git as the source of truth.

**Two binaries:**
- `lixy` — the controller server (centralized coordinator)
- `lixies` — the agent server (runs on each LXC container)

Both include self-installing daemon support (`install`/`uninstall` subcommands) and CLIs.

## Tech Stack

- **Language:** Go 1.25.1
- **Database:** SQLite3 (via `github.com/mattn/go-sqlite3`)
- **CLI:** Cobra (`github.com/spf13/cobra`)
- **Auth:** JWT (`github.com/golang-jwt/jwt/v5`)
- **Logging:** `log/slog` (structured, injected — no `fmt.Printf` for logging)
- **Transport:** HTTP APIs + Unix sockets

## Build

No Makefile. Use `go build` directly:

```bash
# Controller server
go build -o lixy ./cmd/lixy

# Agent server
go build -o lixies ./cmd/lixies

# CLIs
go build -o lixy-cli ./cmd/lixy/cli
go build -o lixies-cli ./cmd/lixies/cli

# With version injected (used in CI releases)
go build -ldflags "-X main.Version=v1.2.3" -o lixy ./cmd/lixy
```

## Tests

No test suite exists yet. There are no `*_test.go` files.

## Architecture

```
cmd/lixy/          → controller entry point (server + install/uninstall)
cmd/lixy/cli/      → controller CLI
cmd/lixies/        → agent entry point
cmd/lixies/cli/    → agent CLI

internal/
  controller/      → controller app: bootstrap, config, router, handlers
  agent/           → agent app: bootstrap, config, router, handlers, reconciler
  domain/          → interfaces and entity types (no implementations)
  services/        → business logic (depends on domain interfaces)
  store/           → SQLite implementations of domain repository interfaces
  server/          → shared HTTP + Unix socket server lifecycle
  middlewares/     → auth and logging middleware
  client/socket/   → Unix socket clients for CLI → daemon communication
  daemon/          → systemd unit file generation and installer
```

### Key Patterns

**Composition root:** `bootstrap.go` in `internal/controller/` and `internal/agent/` wire all dependencies via constructors. Never use global state.

**Domain interfaces in `internal/domain/`:** Repositories (`AgentRepository`, `DeploymentRepository`, `UserRepository`) are defined as interfaces. Services depend on these interfaces; stores implement them. Always program to the interface, not the concrete store.

**Handler structure:** Handlers are grouped by entity in `handlers/` subdirectories. Each handler struct receives injected services and a logger. HTTP methods map to named handler methods.

**Middleware:** Auth middleware validates both user JWT tokens and agent tokens. Context keys for authenticated entities live in `internal/domain/` (e.g., `domain.AgentNameKey`, `domain.UsernameKey`).

**Commands:** CLI commands communicate with the running daemon via Unix socket clients in `internal/client/socket/`.

## Configuration

**Controller** — env vars or `/etc/lixy/lixy.yaml`:
- `LIXY_JWT_SECRET` (required)
- `LIXY_DB_PATH`, `LIXY_LOG_LEVEL`, `LIXY_PORT`

**Agent** — env vars or `/etc/lixy/lixies.yaml`:
- `LIXIES_CONTROLLER` (controller URL, required)
- `LIXIES_LOG_LEVEL`, `LIXIES_WORK_DIR`

## CI/CD

GitHub Actions (`.github/workflows/main.yml`) triggers on `v*` tags and manual dispatch. Builds for `linux/amd64` and `darwin/arm64`, packages as `.tar.gz`, and creates a GitHub Release. No tests run in CI.

## Conventions

- File names match their primary type: `deployment_service.go` → `DeploymentService`
- Errors wrapped with context: `fmt.Errorf("context: %w", err)`
- HTTP responses are JSON; use `Authorization: Bearer <token>` for auth
- REST-style routes: `/api/deployments`, `/api/agents`, `/api/auth`
- Add new features following the pattern: domain interface → store implementation → service → handler → router registration → bootstrap wiring
