## ADDED Requirements

### Requirement: OpenAPI annotations on all admin API handlers

The worker SHALL have swaggo/swag annotations on every handler function under `/api/v1/*` (channels, groups, API keys, logs, stats, settings, models, user).

Each annotation SHALL include:

- `@Summary` — one-line description
- `@Tags` — grouping tag (e.g., Channels, Groups, Stats)
- `@Accept json` / `@Produce json`
- `@Param` — path params, query params, and request body schemas
- `@Success` / `@Failure` — response status codes with schema references
- `@Security BearerAuth` — for JWT-protected endpoints
- `@Router` — method and path

#### Scenario: All admin endpoints are annotated

- **WHEN** `swag init` is run in the worker directory
- **THEN** the generated `docs/swagger.json` SHALL contain definitions for all admin API endpoints listed in `routes.go`
- **AND** each endpoint SHALL have request/response schemas defined

#### Scenario: Login endpoint is publicly documented

- **WHEN** viewing the generated OpenAPI spec
- **THEN** the `POST /api/v1/user/login` endpoint SHALL NOT require `BearerAuth` security
- **AND** it SHALL document the `username` and `password` request body fields

### Requirement: OpenAPI general info block

The worker's main function or a dedicated doc.go file SHALL contain a swag general API info annotation block with:

- `@title Wheel API`
- `@version` matching the current release
- `@description` with a brief project description
- `@securityDefinitions.apikey BearerAuth` with `in: header`, `name: Authorization`

#### Scenario: Spec contains general info

- **WHEN** the OpenAPI spec is generated
- **THEN** the `info` section SHALL contain title "Wheel API" and a non-empty description
- **AND** the `securityDefinitions` section SHALL define `BearerAuth`

### Requirement: Makefile integration for spec generation

The worker `Makefile` SHALL include a `docs` target that runs `swag init` with the correct entry point and output directory.

#### Scenario: Running make docs generates spec

- **WHEN** running `make docs` in the worker directory
- **THEN** `docs/swagger.json` and `docs/docs.go` SHALL be generated or updated
- **AND** the process SHALL exit with code 0

### Requirement: Generated spec files committed to repository

The generated `docs/` directory (containing `swagger.json`, `swagger.yaml`, `docs.go`) SHALL be committed to the repository so that downstream consumers (frontend codegen, CI) can reference them without running `swag init`.

#### Scenario: Spec files exist in repository

- **WHEN** cloning the repository
- **THEN** `apps/worker/docs/swagger.json` SHALL exist and be valid JSON conforming to OpenAPI 3.0 or Swagger 2.0 schema
