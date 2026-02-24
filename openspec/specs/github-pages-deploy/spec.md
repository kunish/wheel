## MODIFIED Requirements

### Requirement: GitHub Actions workflow for Pages deployment

A GitHub Actions workflow SHALL build the web frontend and deploy it to GitHub Pages on every release. The release workflow SHALL also include a screenshot generation job that runs in parallel with existing Docker and Pages jobs.

#### Scenario: Automatic deployment on release

- **WHEN** release-please creates a new release
- **THEN** the GitHub Actions workflow SHALL build the web app with `VITE_HASH_ROUTER=true`
- **AND** deploy the contents of `apps/web/dist` to GitHub Pages

#### Scenario: Pages deployment uses correct permissions

- **WHEN** the workflow runs
- **THEN** it SHALL use the `actions/deploy-pages` action
- **AND** request `pages: write` and `id-token: write` permissions

#### Scenario: Screenshot job runs alongside existing jobs

- **WHEN** release-please creates a new release
- **THEN** the `screenshots` job SHALL run in parallel with `docker` and `pages-build` jobs
- **AND** all jobs SHALL depend only on `release-please` output
