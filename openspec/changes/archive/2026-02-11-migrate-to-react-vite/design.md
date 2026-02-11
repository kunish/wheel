## Context

The Wheel project is a monorepo (`apps/web`, `apps/worker`, `packages/core`) where the frontend (`@wheel/web`) is a Next.js 16 App Router application serving as a dashboard for an LLM API gateway. Despite using Next.js, the frontend is entirely client-rendered — every page component is marked `"use client"`, no server components or API routes are used, and the only server-side feature is `next.config.ts` rewrites for API proxying. The app uses a neobrutalism design system with shadcn/ui, Radix UI, Tailwind CSS v4, Zustand for auth state, and React Query for server state. WebSocket connections go directly to the worker (port 8787), bypassing Next.js entirely.

The current Docker deployment runs a full Node.js server (`node apps/web/server.js`) via Next.js standalone output, which is unnecessary overhead for what is a static SPA. Caddy already handles reverse proxying of `/api/*` and `/v1/*` to the worker.

## Goals / Non-Goals

**Goals:**

- Replace Next.js with Vite + React Router v7 while preserving 100% of existing functionality and UI
- Produce static build output (`dist/`) instead of a Node.js standalone server
- Maintain the existing monorepo structure and `packages/core` shared types
- Keep all existing dependencies (shadcn/ui, Radix, Recharts, TanStack Table/Query, Zustand, etc.)
- Simplify the Docker web stage to serve static files via a lightweight HTTP server or Caddy
- Preserve the neobrutalism design system, dark/light theme toggle, and all CSS

**Non-Goals:**

- Changing any backend code in `apps/worker/`
- Modifying the shared `packages/core` package
- Adding SSR capabilities to Vite (the app is a pure SPA)
- Changing the UI design or component library
- Upgrading or changing UI component dependencies beyond what's necessary for the migration

## Decisions

### 1. Routing: React Router v7 (framework mode disabled)

**Choice**: React Router v7 in library/SPA mode with explicit route configuration.

**Rationale**: React Router v7 is the standard for React SPAs. The current app has only ~8 routes in a flat structure with a single protected layout wrapper — no nested layouts complexity that would warrant a file-based router. An explicit route config in a single file is simpler and more maintainable.

**Alternatives considered**:

- TanStack Router: More type-safe but smaller ecosystem, adds learning curve for contributors
- Wouter: Too minimal, lacks built-in outlet/layout patterns needed for the protected route wrapper

### 2. Theme System: next-themes replacement

**Choice**: Keep `next-themes` — it works with any React app, not just Next.js, despite its name. It handles `class`-based dark mode toggling, system preference detection, and localStorage persistence. The existing `ThemeProvider` wrapper and `useTheme` hook calls remain unchanged.

**Rationale**: `next-themes` has zero Next.js-specific runtime dependencies. It works by toggling a `class` attribute on the `<html>` element and persisting to localStorage — purely client-side DOM manipulation. Replacing it would mean reimplementing identical functionality.

**Alternatives considered**:

- Custom hook: More code to maintain for no benefit
- `@theme-toggles/react`: Less mature, different API would require component changes

### 3. Font Loading: CSS @font-face instead of next/font

**Choice**: Replace `next/font/google` (Space Grotesk, JetBrains Mono) with Fontsource packages or direct CSS `@font-face` declarations.

**Rationale**: `next/font` is Next.js-specific. Fontsource provides the same self-hosted, optimized font loading as npm packages. The CSS custom properties (`--font-sans-custom`, `--font-mono-custom`) remain unchanged — only the loading mechanism changes.

### 4. API Proxying

**Choice**: Vite dev server proxy for development; Caddy reverse proxy for production (already configured).

**Rationale**: In development, Vite's `server.proxy` config replaces `next.config.ts` rewrites with identical behavior. In production, the existing Caddyfile already proxies `/api/*` and `/v1/*` to the worker — the web container just needs to serve static files. The Docker web service switches from running a Node.js server to serving `dist/` via a lightweight static server.

### 5. Build Output & Docker

**Choice**: Replace Next.js standalone output with Vite static build. Docker web stage uses `nginx:alpine` or `caddy:alpine` to serve the `dist/` directory.

**Rationale**: Static files are simpler to serve, cache, and CDN-distribute than a Node.js server. The Dockerfile shrinks significantly — no Node.js runtime needed in the final image.

### 6. Entry Point Architecture

**Choice**: Standard Vite SPA entry with `index.html` → `main.tsx` → `App.tsx` (providers + router).

**Rationale**: This is the standard Vite React project structure. `App.tsx` composes providers (ThemeProvider, QueryProvider, Toaster) and renders the router outlet — directly mapping from the current `layout.tsx` hierarchy.

### 7. Environment Variables

**Choice**: Prefix migration from `NEXT_PUBLIC_*` to `VITE_*`. Currently only `NEXT_PUBLIC_API_BASE_URL` is used.

**Rationale**: Vite exposes env vars prefixed with `VITE_` to client code via `import.meta.env`. Single variable to rename.

## Risks / Trade-offs

- **[SPA Routing]** Client-side routing requires a catch-all fallback (`index.html` for all paths) in production. → Caddy config needs `try_files` or equivalent. Mitigation: Add `try_files {path} /index.html` to Caddyfile.
- **[SEO]** Loss of any potential SSR/SSG capability. → This is a dashboard behind authentication; SEO is irrelevant. Non-issue.
- **[Font Flash]** `next/font` provides automatic font optimization (preloading, size-adjust). Without it, there may be a brief FOUT. → Mitigation: Use Fontsource packages which provide similar self-hosted optimization, and add `font-display: swap` with preload links.
- **[WebSocket Proxy]** Currently WebSocket connections bypass Next.js (direct to worker:8787). → No change needed — WebSocket connections already go directly to the worker via the `VITE_WS_URL` (currently `NEXT_PUBLIC_API_BASE_URL`).
- **[Vercel Deployment]** Current Vercel config is Next.js-specific. → Mitigation: Update `vercel.json` for SPA static deployment with rewrite rules, or remove if not needed.
