## ADDED Requirements

### Requirement: HashRouter support for static hosting

The web frontend SHALL support `HashRouter` as an alternative to `BrowserRouter`, controlled by the build-time environment variable `VITE_HASH_ROUTER`. When set to `"true"`, the app SHALL use `HashRouter`.

#### Scenario: Build with hash router enabled

- **WHEN** the app is built with `VITE_HASH_ROUTER=true`
- **THEN** all routes SHALL use hash-based URLs (e.g., `/#/dashboard`, `/#/login`)
- **AND** the built SPA SHALL work correctly when served from any static file server without server-side routing configuration

#### Scenario: Default build uses BrowserRouter

- **WHEN** the app is built without `VITE_HASH_ROUTER` or with it set to `"false"`
- **THEN** all routes SHALL use standard path-based URLs (e.g., `/dashboard`, `/login`)

### Requirement: GitHub Actions workflow for Pages deployment

A GitHub Actions workflow SHALL build the web frontend and deploy it to GitHub Pages on every push to the `main` branch.

#### Scenario: Automatic deployment on push

- **WHEN** a commit is pushed to the `main` branch
- **THEN** the GitHub Actions workflow SHALL build the web app with `VITE_HASH_ROUTER=true`
- **AND** deploy the contents of `apps/web/dist` to GitHub Pages

#### Scenario: Pages deployment uses correct permissions

- **WHEN** the workflow runs
- **THEN** it SHALL use the `actions/deploy-pages` action
- **AND** request `pages: write` and `id-token: write` permissions

### Requirement: Static build produces standalone SPA

The web build output SHALL be a fully self-contained SPA that works without any backend proxy or server-side routing. All API communication SHALL go through the user-configured base URL.

#### Scenario: Opening index.html directly

- **WHEN** the built `dist/index.html` is opened in a browser (served by any static file server)
- **AND** the user has configured an API base URL
- **THEN** the app SHALL function correctly, loading all routes and making API requests to the configured URL

### Requirement: 404.html fallback for GitHub Pages

The build process SHALL copy `index.html` to `404.html` in the output directory as a fallback for direct URL access on GitHub Pages.

#### Scenario: Direct URL access on GitHub Pages

- **WHEN** a user navigates directly to a deep link (e.g., `https://user.github.io/wheel/#/logs`)
- **THEN** GitHub Pages SHALL serve the SPA via the `404.html` fallback
- **AND** the hash router SHALL handle the route correctly
