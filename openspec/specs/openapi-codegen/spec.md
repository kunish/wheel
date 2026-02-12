## ADDED Requirements

### Requirement: TypeScript types generated from OpenAPI spec

The frontend project SHALL use `openapi-typescript` to generate TypeScript type definitions from the worker's OpenAPI spec file.

#### Scenario: Running codegen produces type file

- **WHEN** running `pnpm generate:api` in the web project root
- **THEN** a TypeScript type definition file SHALL be generated at `src/lib/api.gen.d.ts`
- **AND** the file SHALL export type interfaces matching all API request/response schemas from the OpenAPI spec

### Requirement: Type-safe API client using openapi-fetch

The frontend SHALL use `openapi-fetch` with the generated types to provide a type-safe API client, replacing the manually-typed `apiFetch` functions in `lib/api.ts`.

#### Scenario: API call is type-checked

- **WHEN** a developer calls an API endpoint using the generated client (e.g., `client.GET("/api/v1/channel/list")`)
- **THEN** TypeScript SHALL infer the response type from the OpenAPI spec
- **AND** passing an invalid path or wrong parameters SHALL produce a compile-time error

### Requirement: Codegen script in package.json

The web project's `package.json` SHALL include a `generate:api` script that invokes `openapi-typescript` pointing to the worker's spec file.

#### Scenario: Script is runnable

- **WHEN** a developer runs `pnpm --filter @wheel/web generate:api`
- **THEN** the TypeScript types SHALL be regenerated from the latest OpenAPI spec
