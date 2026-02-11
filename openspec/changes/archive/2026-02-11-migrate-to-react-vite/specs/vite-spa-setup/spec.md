## ADDED Requirements

### Requirement: Vite project configuration

The system SHALL use Vite with `@vitejs/plugin-react-swc` as the build tool and dev server for the frontend application, replacing Next.js entirely.

#### Scenario: Vite config exists with correct plugins

- **WHEN** the frontend project is initialized
- **THEN** a `vite.config.ts` file SHALL exist at `apps/web/vite.config.ts` with `@vitejs/plugin-react-swc` plugin configured

### Requirement: HTML entry point

The system SHALL provide an `index.html` at `apps/web/index.html` as the SPA entry point, referencing the main TypeScript entry file and including meta tags, font preloads, and the root div.

#### Scenario: index.html loads the application

- **WHEN** a browser requests the root URL
- **THEN** the server SHALL serve `index.html` which loads `src/main.tsx` as a module script

#### Scenario: index.html includes document metadata

- **WHEN** `index.html` is served
- **THEN** it SHALL include the page title "Wheel", viewport meta tag, and favicon reference matching the current Next.js metadata

### Requirement: TypeScript entry point

The system SHALL have a `src/main.tsx` file that renders the root React component into the DOM, imports global CSS, and sets up font CSS custom properties.

#### Scenario: Application mounts to root element

- **WHEN** `main.tsx` executes
- **THEN** it SHALL call `ReactDOM.createRoot` on the `#root` element and render the `App` component

### Requirement: Dev server proxy

The Vite dev server SHALL proxy `/api/*` and `/v1/*` requests to the backend worker server, replicating the behavior of Next.js rewrites in `next.config.ts`.

#### Scenario: API requests are proxied in development

- **WHEN** the dev server receives a request to `/api/some-endpoint`
- **THEN** it SHALL forward the request to `http://localhost:8787/api/some-endpoint`

#### Scenario: v1 requests are proxied in development

- **WHEN** the dev server receives a request to `/v1/chat/completions`
- **THEN** it SHALL forward the request to `http://localhost:8787/v1/chat/completions`

#### Scenario: Proxy target is configurable

- **WHEN** the `VITE_API_BASE_URL` environment variable is set
- **THEN** the proxy SHALL forward requests to that URL instead of the default `http://localhost:8787`

### Requirement: Environment variable migration

The system SHALL use `VITE_` prefixed environment variables instead of `NEXT_PUBLIC_` prefixed variables, exposed via `import.meta.env`.

#### Scenario: API base URL variable renamed

- **WHEN** code references the API base URL
- **THEN** it SHALL use `import.meta.env.VITE_API_BASE_URL` instead of `process.env.NEXT_PUBLIC_API_BASE_URL`

### Requirement: Font loading via CSS

The system SHALL load Space Grotesk and JetBrains Mono fonts via Fontsource npm packages or CSS `@font-face` declarations, setting `--font-sans-custom` and `--font-mono-custom` CSS custom properties on the body element.

#### Scenario: Fonts are applied correctly

- **WHEN** the application renders
- **THEN** the body element SHALL have `--font-sans-custom` set to Space Grotesk and `--font-mono-custom` set to JetBrains Mono

### Requirement: Static build output

The `vite build` command SHALL produce a static `dist/` directory containing all HTML, JS, CSS, and assets needed to serve the application without a Node.js runtime.

#### Scenario: Build produces static files

- **WHEN** `pnpm --filter @wheel/web build` is executed
- **THEN** a `dist/` directory SHALL be created at `apps/web/dist/` containing `index.html` and hashed asset files

### Requirement: Docker static serving

The web Dockerfile SHALL build the frontend and serve the `dist/` directory using a lightweight static file server (nginx or caddy), replacing the Node.js standalone server.

#### Scenario: Docker image serves static files

- **WHEN** the Docker web container starts
- **THEN** it SHALL serve the built static files on port 3000 and handle SPA routing fallback (all non-file paths return `index.html`)

### Requirement: Package.json scripts

The `package.json` scripts SHALL be updated to use Vite commands instead of Next.js commands.

#### Scenario: Dev script uses Vite

- **WHEN** `pnpm dev` is run in `apps/web/`
- **THEN** it SHALL start the Vite dev server

#### Scenario: Build script uses Vite

- **WHEN** `pnpm build` is run in `apps/web/`
- **THEN** it SHALL run `vite build` to produce static output

#### Scenario: Preview script available

- **WHEN** `pnpm preview` is run in `apps/web/`
- **THEN** it SHALL run `vite preview` to preview the production build locally

### Requirement: Path alias resolution

The Vite configuration SHALL resolve the `@/` import alias to `src/`, matching the current Next.js/TypeScript path alias configuration.

#### Scenario: Path aliases resolve correctly

- **WHEN** a source file imports from `@/components/ui/button`
- **THEN** Vite SHALL resolve it to `src/components/ui/button`
