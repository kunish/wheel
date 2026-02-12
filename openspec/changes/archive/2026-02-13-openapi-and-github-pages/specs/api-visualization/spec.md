## ADDED Requirements

### Requirement: Scalar API Reference page served by worker

The worker SHALL serve an interactive API documentation page at `GET /docs` using Scalar API Reference, loaded via CDN.

#### Scenario: Accessing /docs returns Scalar UI

- **WHEN** a user navigates to `http://<worker-host>/docs` in a browser
- **THEN** the response SHALL be an HTML page that loads Scalar API Reference
- **AND** Scalar SHALL reference the OpenAPI spec at `/docs/openapi.json`

### Requirement: OpenAPI spec served as JSON endpoint

The worker SHALL serve the generated OpenAPI spec at `GET /docs/openapi.json`.

#### Scenario: Fetching the spec JSON

- **WHEN** a client sends `GET /docs/openapi.json`
- **THEN** the response SHALL have `Content-Type: application/json`
- **AND** the body SHALL be valid OpenAPI/Swagger JSON

### Requirement: Docs routes are public

The `/docs` and `/docs/openapi.json` routes SHALL NOT require authentication (no JWT, no API key).

#### Scenario: Unauthenticated access to docs

- **WHEN** a client sends `GET /docs` without any Authorization header
- **THEN** the response status SHALL be 200
