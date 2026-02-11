## 1. Project Scaffolding & Dependencies

- [x] 1.1 Update `apps/web/package.json`: remove `next` dependency, add `vite`, `@vitejs/plugin-react`, `react-router`, `@tailwindcss/vite`; update scripts (`dev` → `vite`, `build` → `tsc -b && vite build`, `preview` → `vite preview`); remove `start` script; add `"type": "module"`
- [x] 1.2 Create `apps/web/vite.config.ts` with SWC plugin, path alias (`@/` → `src/`), dev server proxy (`/api` and `/v1` → `http://localhost:8787`), and build output config
- [x] 1.3 Create `apps/web/index.html` entry point with `<div id="root">`, `<script type="module" src="/src/main.tsx">`, meta tags (title "Wheel", viewport, favicon), font preload links, and inline theme-init script to prevent FOUC
- [x] 1.4 Create `apps/web/src/main.tsx` that imports fonts from Fontsource, imports `globals.css`, creates React root on `#root`, sets font CSS custom properties on `<body>`, and renders `<App />`
- [x] 1.5 Update `apps/web/tsconfig.json`: remove Next.js-specific compiler options (`jsx: "preserve"`, `plugins` for `next`), set `jsx: "react-jsx"`, ensure path alias `@/*` → `src/*` is configured for IDE resolution
- [x] 1.6 Run `pnpm install` and verify the dev server starts without errors

## 2. Root App Component & Providers

- [x] 2.1 Create `apps/web/src/App.tsx` composing providers: `ThemeProvider` (attribute="class", defaultTheme="system", enableSystem, disableTransitionOnChange) → `QueryProvider` → Router outlet + `<Toaster />`
- [x] 2.2 Update `apps/web/src/components/theme-provider.tsx`: remove `"use client"` directive; keep using `next-themes` (it works without Next.js)
- [x] 2.3 Update `apps/web/src/components/query-provider.tsx`: remove `"use client"` directive

## 3. Routing Setup

- [x] 3.1 Create `apps/web/src/routes.tsx` with React Router v7 route configuration: `/` redirects to `/dashboard`; `/login` renders LoginPage; protected layout wraps `/dashboard`, `/channels`, `/groups`, `/apikeys` (redirects to `/settings`), `/logs`, `/prices`, `/settings`
- [x] 3.2 Create `apps/web/src/components/protected-layout.tsx` extracting the auth guard logic from `apps/web/src/app/(protected)/layout.tsx`: check `useAuthStore`, redirect to `/login` if unauthenticated, show loading spinner, render `<AppLayout><Outlet /></AppLayout>`, call `useStatsWebSocket`
- [x] 3.3 Update `apps/web/src/components/app-layout.tsx`: replace `import Link from "next/link"` with `import { Link } from "react-router"`, replace `usePathname` with `useLocation` from `react-router`, replace `href` props with `to` props on `<Link>` components, remove `"use client"` directive, replace `useTheme` import from `next-themes` (keep as-is, it works)

## 4. Page Component Migration

- [x] 4.1 Migrate `apps/web/src/app/login/page.tsx` → `apps/web/src/pages/login.tsx`: replace `useRouter` from `next/navigation` with `useNavigate` from `react-router`, replace `router.push("/dashboard")` with `navigate("/dashboard")`, remove `"use client"` directive
- [x] 4.2 Migrate `apps/web/src/app/(protected)/dashboard/page.tsx` → `apps/web/src/pages/dashboard.tsx`: replace `next/dynamic` with `React.lazy` + `Suspense`, replace `next/link` with `react-router` Link, replace `useRouter` with `useNavigate`, remove `"use client"` directive
- [x] 4.3 Migrate `apps/web/src/app/(protected)/channels/page.tsx` → `apps/web/src/pages/channels.tsx`: replace `next/dynamic` with `React.lazy` + `Suspense`, replace `useRouter`/`useSearchParams` from `next/navigation` with `useNavigate`/`useSearchParams` from `react-router`, remove `"use client"` directive
- [x] 4.4 Migrate channel sub-components (`_components/channel-dialog.tsx`, `group-dialog.tsx`, `model-picker-dialog.tsx`, `channel-model-picker-dialog.tsx`) → `apps/web/src/pages/channels/` subdirectory: remove `"use client"` directives, update any Next.js imports
- [x] 4.5 Migrate `apps/web/src/app/(protected)/groups/page.tsx` → `apps/web/src/pages/groups.tsx`: replace `useRouter` with `useNavigate`, remove `"use client"` directive
- [x] 4.6 Migrate `apps/web/src/app/(protected)/logs/page.tsx` and `columns.tsx` → `apps/web/src/pages/logs.tsx` and `apps/web/src/pages/logs/columns.tsx`: replace `next/dynamic` with `React.lazy`, replace `next/link` with `react-router` Link, replace `usePathname`/`useRouter`/`useSearchParams` with React Router equivalents, remove `"use client"` directives
- [x] 4.7 Migrate `apps/web/src/app/(protected)/prices/page.tsx` → `apps/web/src/pages/prices.tsx`: remove `"use client"` directive, update any Next.js imports
- [x] 4.8 Migrate `apps/web/src/app/(protected)/settings/page.tsx`, `system-config-section.tsx`, `backup-section.tsx` → `apps/web/src/pages/settings/`: replace `next/dynamic` with `React.lazy`, remove `"use client"` directives
- [x] 4.9 Handle `/apikeys` route: configure as redirect to `/settings` in route config (replacing the `redirect()` page component)

## 5. Dynamic Import Migration

- [x] 5.1 Rewrite `apps/web/src/components/lazy-recharts.tsx`: replace all `next/dynamic` calls with `React.lazy` + `Suspense` wrappers; remove `ssr: false` options (irrelevant in SPA); keep loading fallbacks
- [x] 5.2 Audit all pages for remaining `next/dynamic` usage and convert to `React.lazy` + `Suspense`

## 6. Environment Variables & API Layer

- [x] 6.1 Update `apps/web/src/lib/api.ts`: replace any `process.env.NEXT_PUBLIC_*` references with `import.meta.env.VITE_*`
- [x] 6.2 Update `apps/web/src/hooks/use-stats-ws.ts`: replace any `process.env.NEXT_PUBLIC_*` references with `import.meta.env.VITE_*` for WebSocket URL
- [x] 6.3 Create `apps/web/src/vite-env.d.ts` with `/// <reference types="vite/client" />` and `ImportMetaEnv` interface declaring `VITE_API_BASE_URL`
- [x] 6.4 Create or update `apps/web/.env.example` with `VITE_API_BASE_URL=http://localhost:8787`

## 7. Cleanup

- [x] 7.1 Remove all `"use client"` directives from every file under `apps/web/src/` (no longer needed without Next.js server/client boundary)
- [x] 7.2 Delete `apps/web/src/proxy.ts` (Next.js middleware, no longer needed)
- [x] 7.3 Delete `apps/web/src/app/` directory entirely (replaced by `src/pages/`, `src/routes.tsx`, `src/App.tsx`, `src/main.tsx`)
- [x] 7.4 Delete `apps/web/next.config.ts`
- [x] 7.5 Move `apps/web/src/app/globals.css` to `apps/web/src/globals.css` (if not already moved in step 1.4)
- [x] 7.6 Delete `apps/web/next-env.d.ts` if it exists

## 8. Docker & Deployment

- [x] 8.1 Rewrite `apps/web/Dockerfile`: multi-stage build that installs deps, builds with `vite build`, then serves `dist/` using `nginx:alpine` or `caddy:alpine` with SPA fallback config
- [x] 8.2 Update `Caddyfile`: change the web handler from `reverse_proxy web:3000` to `file_server` with `try_files {path} /index.html` for SPA fallback, or point to the new static server
- [x] 8.3 Update `docker-compose.yml` web service if port or startup command changes
- [x] 8.4 Update `vercel.json` for SPA static deployment with rewrite rules (all paths → `index.html`) or remove if Vercel is no longer used

## 9. Verification

- [x] 9.1 Run `pnpm --filter @wheel/web build` and verify it produces `apps/web/dist/` with `index.html` and hashed assets
- [x] 9.2 Run `pnpm dev` and verify all pages render correctly: login, dashboard, channels, groups, logs, prices, settings
- [x] 9.3 Verify theme toggle works (light/dark/system) and persists across page reloads
- [x] 9.4 Verify protected route redirect: unauthenticated access to `/dashboard` redirects to `/login`
- [x] 9.5 Verify API proxy: login flow works, data loads on dashboard, channels, etc.
- [x] 9.6 Verify WebSocket: real-time stats updates on dashboard work
- [x] 9.7 Verify deep linking: direct browser navigation to `/settings` works after auth
- [x] 9.8 Verify all lazy-loaded components render correctly (charts, dialogs, settings sections)
- [x] 9.9 Run `pnpm --filter @wheel/web test` and verify existing tests pass
- [x] 9.10 Docker build and verify the container serves the app correctly with SPA routing
