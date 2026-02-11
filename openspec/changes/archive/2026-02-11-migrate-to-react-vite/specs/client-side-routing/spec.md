## ADDED Requirements

### Requirement: Route configuration

The system SHALL define all application routes in a single explicit route configuration using React Router v7, replacing Next.js file-based routing.

#### Scenario: All existing routes are mapped

- **WHEN** the route configuration is loaded
- **THEN** it SHALL include routes for `/login`, `/dashboard`, `/channels`, `/groups`, `/apikeys`, `/logs`, `/prices`, and `/settings`

#### Scenario: Root path redirects to dashboard

- **WHEN** a user navigates to `/`
- **THEN** the system SHALL redirect to `/dashboard`

### Requirement: Protected route wrapper

The system SHALL wrap all authenticated routes (`/dashboard`, `/channels`, `/groups`, `/apikeys`, `/logs`, `/prices`, `/settings`) in a layout route that checks authentication before rendering content.

#### Scenario: Authenticated user sees page content

- **WHEN** an authenticated user navigates to `/dashboard`
- **THEN** the system SHALL render the AppLayout with sidebar and the Dashboard page content

#### Scenario: Unauthenticated user is redirected to login

- **WHEN** an unauthenticated user navigates to any protected route
- **THEN** the system SHALL redirect the user to `/login`

#### Scenario: Loading state during auth check

- **WHEN** the auth check is in progress
- **THEN** the system SHALL display a centered loading spinner

### Requirement: Navigation with React Router

All internal navigation SHALL use React Router's `Link` component and `useNavigate` hook instead of `next/link` and `next/navigation`.

#### Scenario: Sidebar navigation uses React Router Link

- **WHEN** the sidebar renders navigation items
- **THEN** each item SHALL use React Router's `Link` component with `to` prop instead of `href`

#### Scenario: Programmatic navigation uses useNavigate

- **WHEN** code needs to navigate programmatically (e.g., after login, after logout)
- **THEN** it SHALL use React Router's `useNavigate` hook instead of Next.js `useRouter`

#### Scenario: Active route detection uses useLocation

- **WHEN** the sidebar determines which nav item is active
- **THEN** it SHALL use React Router's `useLocation` hook instead of Next.js `usePathname`

### Requirement: SPA fallback routing

In production, the static file server SHALL return `index.html` for all paths that don't match a static file, enabling client-side routing.

#### Scenario: Deep link to protected route works

- **WHEN** a user directly navigates to `https://app.example.com/settings` via browser URL bar
- **THEN** the server SHALL serve `index.html` and React Router SHALL render the Settings page (after auth check)

#### Scenario: Static assets are served directly

- **WHEN** a browser requests `/assets/index-abc123.js`
- **THEN** the server SHALL serve the actual file, not the `index.html` fallback

### Requirement: Provider composition in App component

The root `App` component SHALL compose all providers (ThemeProvider, QueryProvider, Toaster) and render the router outlet, directly replacing the Next.js root `layout.tsx` hierarchy.

#### Scenario: Provider order matches current behavior

- **WHEN** the App component renders
- **THEN** providers SHALL be nested in order: ThemeProvider → QueryProvider → RouterProvider + Toaster

### Requirement: WebSocket connection in protected layout

The protected route layout SHALL initialize the WebSocket connection for real-time stats updates, matching the current behavior of the `(protected)/layout.tsx`.

#### Scenario: WebSocket connects when authenticated

- **WHEN** an authenticated user enters the protected layout
- **THEN** the `useStatsWebSocket` hook SHALL be called with the QueryClient to enable real-time dashboard updates
