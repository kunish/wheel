# AGENTS.md

## Cursor Cloud specific instructions

### Architecture

Wheel is a pnpm monorepo with two main apps:

| App                  | Path          | Tech                                       | Dev Port |
| -------------------- | ------------- | ------------------------------------------ | -------- |
| **Web** (frontend)   | `apps/web`    | React 19, Vite, TypeScript, Tailwind CSS 4 | 5173     |
| **Worker** (backend) | `apps/worker` | Go 1.26, Gin, TiDB/MySQL                   | 8787     |

The Vite dev server proxies `/api`, `/v1`, `/docs`, `/mcp/*` to the worker at `http://localhost:8787`.

### Prerequisites

- **Go 1.26** — required by `apps/worker/go.mod`
- **Node.js 22 + pnpm 10.29.3** — required by root `package.json`
- **Docker** — needed to run TiDB (MySQL-compatible database on port 4000)

### Running services

1. **TiDB** must be running before the worker starts:
   ```
   docker start tidb || docker run -d --name tidb -p 4000:4000 -v tidb-data:/tmp/tidb pingcap/tidb:v8.5.0
   ```
2. **Worker** (with hot-reload via Air): from repo root:
   ```
   cd apps/worker && JWT_SECRET=dev-secret-key-for-local-testing-only-12345 DB_DSN="root:@tcp(127.0.0.1:4000)/wheel?parseTime=true&charset=utf8mb4" go run github.com/air-verse/air@latest
   ```
   Or simply `pnpm dev:worker` (requires env vars exported or `.env` configured).
3. **Web** (Vite dev server): `pnpm dev:web` or `cd apps/web && npx vite --host 0.0.0.0`

`pnpm dev` starts both concurrently.

### Key gotchas

- The worker auto-creates the `wheel` database and runs migrations on first start — no manual migration step needed.
- Default admin credentials: `admin` / `admin`. The worker logs a security warning if the default password is used.
- The worker creates a default API key on first boot (logged to stdout as `sk-wheel-...`).
- `.env` at repo root: copy from `.env.example`, set `JWT_SECRET` to any non-empty string for local dev.
- The Docker daemon in the Cloud VM requires `sudo dockerd` with `fuse-overlayfs` storage driver and `iptables-legacy`. Socket permissions may need `sudo chmod 666 /var/run/docker.sock`.

### Lint / Test / Build

See `package.json` scripts for standard commands:

- **Lint**: `pnpm lint` (ESLint), `pnpm format:check` (Prettier)
- **Frontend tests**: `pnpm --filter @wheel/web run test` (Vitest)
- **Backend tests**: `cd apps/worker && go test ./...`
- **Build frontend**: `pnpm build`
- **Build worker**: `cd apps/worker && make build`

### Pre-commit hooks

Husky runs `lint-staged` (ESLint + Prettier) on pre-commit and `commitlint` on commit-msg. Commit messages must follow Conventional Commits format.
