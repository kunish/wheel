## Why

The frontend currently uses Next.js 16 (App Router) but functions as a pure client-side SPA dashboard — it uses no SSR, no server components, no API routes, and no static generation. Next.js adds unnecessary complexity, slower dev startup, and a heavier build output (standalone Node.js server) for what is fundamentally a static single-page application. Migrating to React + Vite eliminates this mismatch, resulting in faster builds, simpler deployment (static files served by the existing Caddy/backend), and a lighter dependency footprint.

## What Changes

- **BREAKING**: Remove Next.js framework entirely (`next`, `next-themes` dependencies)
- Replace Next.js App Router with React Router v7 for client-side routing
- Replace Next.js build system with Vite + SWC
- Replace `next.config.ts` API rewrites with Vite dev proxy + production reverse proxy (Caddy already handles this)
- Replace `next/link`, `next/navigation` imports with React Router equivalents
- Replace `next-themes` with a lightweight custom theme provider or `next-themes` alternative
- Convert file-based routing to explicit React Router route configuration
- Update Docker build to serve static assets instead of running a Node.js server
- Update environment variable prefix from `NEXT_PUBLIC_*` to `VITE_*`
- All existing UI components (shadcn/ui, Radix, Recharts, TanStack Table, etc.) remain unchanged
- All state management (Zustand, React Query) remains unchanged
- All pages, features, and visual appearance remain pixel-perfect identical

## Capabilities

### New Capabilities

- `vite-spa-setup`: Vite project configuration, entry point, HTML template, dev proxy, and build output for the React SPA
- `client-side-routing`: React Router v7 route configuration replacing Next.js App Router file-based routing, including protected route handling and navigation
- `theme-system`: Dark/light theme toggle replacing next-themes, compatible with shadcn/ui and Tailwind CSS v4

### Modified Capabilities

_(No existing spec-level requirements change — this is a pure infrastructure migration with identical user-facing behavior)_

## Impact

- **Code**: All files under `apps/web/` are affected. Components in `src/components/`, `src/hooks/`, `src/lib/` require minimal changes (mostly import path updates). Pages in `src/app/` need restructuring from file-based to explicit routes.
- **Dependencies**: Remove `next`, `next-themes`. Add `react-router`, `vite`, `@vitejs/plugin-react-swc`. All other deps (shadcn/ui, Radix, Recharts, Zustand, React Query, etc.) are unchanged.
- **Build & Deploy**: Docker web stage changes from running a Node.js server to building static files. Caddy/nginx serves the static build and proxies API requests. Vercel deployment config needs updating.
- **Dev Experience**: `pnpm dev` for web switches from `next dev` to `vite`. Faster HMR, faster cold start.
- **Backend**: Zero changes to `apps/worker/`. API contract unchanged.
- **Shared Package**: `packages/core/` unchanged.
