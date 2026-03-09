# Changelog

## [1.27.4](https://github.com/kunish/wheel/compare/v1.27.3...v1.27.4) (2026-03-09)


### Bug Fixes

* use correct management API path for auth file models endpoint ([9047222](https://github.com/kunish/wheel/commit/90472229601b01d52c2786052803e80a02f1f44c))


### Performance Improvements

* optimize bulk auth file upload with batch upsert and async model sync ([05c9136](https://github.com/kunish/wheel/commit/05c91366ed17215bfe5d9c87e25ebfd4c3823093))

## [1.27.3](https://github.com/kunish/wheel/compare/v1.27.2...v1.27.3) (2026-03-09)


### Bug Fixes

* remove stale CLIProxyAPIPlus COPY from worker Dockerfile ([424b291](https://github.com/kunish/wheel/commit/424b2911733cd46d553eb29431750419de3a6e2f))

## [1.27.2](https://github.com/kunish/wheel/compare/v1.27.1...v1.27.2) (2026-03-09)


### Bug Fixes

* add OAuth session TTL and propagate request context across handlers ([22b22ad](https://github.com/kunish/wheel/commit/22b22ada791d6c155207a977ae77e81dbb595dd9))
* auth file list text overflow by completing min-w-0 truncation chain ([872e3e1](https://github.com/kunish/wheel/commit/872e3e13c0f7f042eb1269271cf81e704944f250))
* wrap static file serving in catch-all handle to avoid API rewrite ([05dab40](https://github.com/kunish/wheel/commit/05dab40c8a6da927de01c1392b6f0f03e49c091c))

## [1.27.1](https://github.com/kunish/wheel/compare/v1.27.0...v1.27.1) (2026-03-08)


### Bug Fixes

* fix pre-existing test failures, update docs, and eliminate sdk/config indirection ([1c42da8](https://github.com/kunish/wheel/commit/1c42da882042c7e3f6d56e1dd14fa7d2f51d5670))

## [1.27.0](https://github.com/kunish/wheel/compare/v1.26.1...v1.27.0) (2026-03-08)


### Features

* absorb vendored module into worker and migrate all auth handlers to first-party ([bd6e3ad](https://github.com/kunish/wheel/commit/bd6e3ad679c8ef5a4b1aa785bcc44d27200e87d8))
* migrate device-flow auth providers to first-party ownership ([1b2e801](https://github.com/kunish/wheel/commit/1b2e8010ccba19a498a264030998ecb25b9556dd))
* migrate runtime integration ownership seams ([926ef98](https://github.com/kunish/wheel/commit/926ef9827f067c0d8957320f1f7f58f59e4c3877))


### Bug Fixes

* resolve Copilot 502 by direct executor integration in relay handler ([41934e1](https://github.com/kunish/wheel/commit/41934e132b5bb995ebe05adddc62089f66b133de))
* restore runtime seam factory files ([e9f7056](https://github.com/kunish/wheel/commit/e9f7056ea41b09fe0b685977cfb60610c6579dad))

## [1.26.1](https://github.com/kunish/wheel/compare/v1.26.0...v1.26.1) (2026-03-08)


### Bug Fixes

* tighten runtime OpenAI compatibility ([20d4d72](https://github.com/kunish/wheel/commit/20d4d7288b38bc6d6fb9427ba86c9c5d97179821))

## [1.26.0](https://github.com/kunish/wheel/compare/v1.25.0...v1.26.0) (2026-03-07)


### Features

* improve runtime auth bulk management ([ab6c57a](https://github.com/kunish/wheel/commit/ab6c57a964f072f6a493f8b4e26693abed700b3a))

## [1.25.0](https://github.com/kunish/wheel/compare/v1.24.0...v1.25.0) (2026-03-07)


### Features

* clarify runtime model sync updates ([2048f87](https://github.com/kunish/wheel/commit/2048f87b5caaf564568e050cadbfc4e80361647d))

## [1.24.0](https://github.com/kunish/wheel/compare/v1.23.0...v1.24.0) (2026-03-07)


### Features

* improve runtime channel auth and quota UX ([ef4f6e4](https://github.com/kunish/wheel/commit/ef4f6e4a8cb0aeed47e3d43cdc8198c3483ccb2f))

## [1.23.0](https://github.com/kunish/wheel/compare/v1.22.5...v1.23.0) (2026-03-07)


### Features

* expand OpenAI-compatible relay support ([5e28cda](https://github.com/kunish/wheel/commit/5e28cdaa7934f5308b2c9eb886e79b5158922cf4))

## [1.22.5](https://github.com/kunish/wheel/compare/v1.22.4...v1.22.5) (2026-03-07)


### Bug Fixes

* include vendored cliproxy sdk in worker runtime ([706de3a](https://github.com/kunish/wheel/commit/706de3a94ce0647cf53adaf1de65417a2d3f4bbf))

## [1.22.4](https://github.com/kunish/wheel/compare/v1.22.3...v1.22.4) (2026-03-07)


### Bug Fixes

* stage vendored worker module metadata in docker build ([2d9edff](https://github.com/kunish/wheel/commit/2d9edff16dc53b653f535bacf5d19245170e9875))

## [1.22.3](https://github.com/kunish/wheel/compare/v1.22.2...v1.22.3) (2026-03-06)


### Bug Fixes

* vendor CLIProxyAPIPlus for worker runtime ([13e936d](https://github.com/kunish/wheel/commit/13e936df0de2263531a03c6ea696fef68fe63dd3))

## [1.22.2](https://github.com/kunish/wheel/compare/v1.22.1...v1.22.2) (2026-03-06)


### Bug Fixes

* preserve codex runtime management key on auth refresh ([a9faf5b](https://github.com/kunish/wheel/commit/a9faf5bb9f2dcfea6148b425275674d56eb645b0))

## [1.22.1](https://github.com/kunish/wheel/compare/v1.22.0...v1.22.1) (2026-03-06)


### Bug Fixes

* remove local CLIProxyAPI replace from worker module ([4ffb192](https://github.com/kunish/wheel/commit/4ffb1926bfee29173fe04c2441bc2dd629ab2e30))

## [1.22.0](https://github.com/kunish/wheel/compare/v1.21.0...v1.22.0) (2026-03-06)


### Features

* expand runtime channels and log observability ([bd1d809](https://github.com/kunish/wheel/commit/bd1d8094b69ba6726b45449a8f90bc05f603a4af))


### Bug Fixes

* stabilize Codex auth sync and quota loading ([b1cccc7](https://github.com/kunish/wheel/commit/b1cccc76f6dbde5620adb3d47f3ced3d2afc5ac8))

## [1.21.0](https://github.com/kunish/wheel/compare/v1.20.0...v1.21.0) (2026-03-06)


### Features

* add Codex (CLIProxyAPI) provider support ([e5b237f](https://github.com/kunish/wheel/commit/e5b237f3020e612e70f69d4499107c1389ff41d6))
* add codex runtime and auth upload flow ([a9eeb63](https://github.com/kunish/wheel/commit/a9eeb631680086399875dd74b574d5c0171609ec))

## [1.20.0](https://github.com/kunish/wheel/compare/v1.19.2...v1.20.0) (2026-03-05)


### Features

* **logs:** stabilize list layout and enrich detail diagnostics ([256804d](https://github.com/kunish/wheel/commit/256804d9fbfc84798f1aa6549a5a4513998c81bb))

## [1.19.2](https://github.com/kunish/wheel/compare/v1.19.1...v1.19.2) (2026-03-05)


### Bug Fixes

* **mcp:** split gateway endpoints and stabilize tool discovery ([43e6249](https://github.com/kunish/wheel/commit/43e62497f8ee81ccc388039f1ab7c10dd5216a8d))
* **relay:** stop retrying non-retryable proxy errors ([fd7fefb](https://github.com/kunish/wheel/commit/fd7fefbe78d65c308b97f1efb31567c3274feba5))

## [1.19.1](https://github.com/kunish/wheel/compare/v1.19.0...v1.19.1) (2026-03-04)


### Bug Fixes

* **playground:** streamline settings entry and confirm clear action ([468dbf1](https://github.com/kunish/wheel/commit/468dbf1e9eed243f2fe0f01466fe19e9e1787d32))

## [1.19.0](https://github.com/kunish/wheel/compare/v1.18.0...v1.19.0) (2026-03-04)


### Features

* implement 20 enterprise features inspired by Bifrost AI gateway ([e9b5826](https://github.com/kunish/wheel/commit/e9b5826126d3491efad293ae5a9b160a00bb5c82))
* **playground:** add MCP auto/manual chat workflow ([b07be5c](https://github.com/kunish/wheel/commit/b07be5c97730d8b8cce1b9a61efa0baa8a563b33))
* **playground:** add MCP-aware request builders and API helpers ([7aab15c](https://github.com/kunish/wheel/commit/7aab15cae68d2ab62af846cc512b7acfc09b94f0))


### Bug Fixes

* **playground:** keep cleared tool selection and continue after tool errors ([17683d5](https://github.com/kunish/wheel/commit/17683d5d126ec1fa96472d851c910a3b1b17b098))
* **playground:** resolve MCP review blockers ([6c178fd](https://github.com/kunish/wheel/commit/6c178fdc72e630f4b04d8f88e6c179e5c53b41a7))
* **playground:** scope model selector to active profile groups ([dac0d6b](https://github.com/kunish/wheel/commit/dac0d6bb8d1db2f0d28d93ec6741c4d796505304))
* **playground:** stabilize relay background flows and dashboard UX ([67a7c85](https://github.com/kunish/wheel/commit/67a7c85f4e8decdbff1699d18d83626cf530c113))

## [1.18.0](https://github.com/kunish/wheel/compare/v1.17.0...v1.18.0) (2026-03-02)

### Features

- add backend CRUD for guardrails and tags pages ([85cffbb](https://github.com/kunish/wheel/commit/85cffbb7f397bacf223a75651ac237a7b2547bf0))

### Bug Fixes

- oauth hardening, API docs routing and frontend improvements ([79d793e](https://github.com/kunish/wheel/commit/79d793ef2796d1a99a84afb9dc1f9c6d7020708f))

## [1.17.0](https://github.com/kunish/wheel/compare/v1.16.0...v1.17.0) (2026-02-27)

### Features

- add OAuth support for MCP gateway ([3467770](https://github.com/kunish/wheel/commit/3467770962ff75b00d76d3c9afe705290a31e64b))

## [1.16.0](https://github.com/kunish/wheel/compare/v1.15.0...v1.16.0) (2026-02-26)

### Features

- add audit logs, MCP logs, model limits backend + UI enhancements ([cf88b7f](https://github.com/kunish/wheel/commit/cf88b7fa7f12d43ec445059f84781027653509f6))

## [1.15.0](https://github.com/kunish/wheel/compare/v1.14.1...v1.15.0) (2026-02-26)

### Features

- add CLI reset-password command and DB-backed auth ([ec86345](https://github.com/kunish/wheel/commit/ec863458d466ca4a5b276f6124bf2862b766401f))
- add MCP gateway, routing rules and relay pipeline ([32348dd](https://github.com/kunish/wheel/commit/32348dd330a223619911f0f99b3ab92f5ebe8a6f))
- expand providers, multimodal API, plugins, OTel and provider icons ([03ae5ed](https://github.com/kunish/wheel/commit/03ae5edd0dba30cbf525f6e73a70fb5ba1718f45))

### Bug Fixes

- correct .gitignore paths for worker binary and temp files ([2914d8d](https://github.com/kunish/wheel/commit/2914d8d8d732d1ded92518eb943fd5def601e0d8))

## [1.14.1](https://github.com/kunish/wheel/compare/v1.14.0...v1.14.1) (2026-02-25)

### Bug Fixes

- **relay:** add fallback ID for tool_calls missing id field ([#55](https://github.com/kunish/wheel/issues/55)) ([1115c9b](https://github.com/kunish/wheel/commit/1115c9b0686a06b0110db9e07ab57e50a508bbf1))

## [1.14.0](https://github.com/kunish/wheel/compare/v1.13.0...v1.14.0) (2026-02-25)

### Features

- **web:** redesign model page card styling and remove animation wrappers ([5e71ed2](https://github.com/kunish/wheel/commit/5e71ed26d9250c35e1459f4035e27f1d9b12c236))

## [1.13.0](https://github.com/kunish/wheel/compare/v1.12.4...v1.13.0) (2026-02-25)

### Features

- **worker:** create default API key on first startup ([53e7b67](https://github.com/kunish/wheel/commit/53e7b67b99851ca84b81507f3eb07fbbdee4eb06))

## [1.12.4](https://github.com/kunish/wheel/compare/v1.12.3...v1.12.4) (2026-02-25)

### Bug Fixes

- **docker:** rename container user from 'wheel' to 'app' to avoid Alpine built-in group conflict ([afd1bee](https://github.com/kunish/wheel/commit/afd1beebf4ec77bfdac45c39535697b8204e3636))

## [1.12.3](https://github.com/kunish/wheel/compare/v1.12.2...v1.12.3) (2026-02-25)

### Bug Fixes

- **ui:** remove jarring zoom animation from tabs content ([1632bb4](https://github.com/kunish/wheel/commit/1632bb496521a783949aa59d14a8263b16dd14ed))

## [1.12.2](https://github.com/kunish/wheel/compare/v1.12.1...v1.12.2) (2026-02-24)

### Bug Fixes

- optimize materialize/unmaterialize selection interaction ([#49](https://github.com/kunish/wheel/issues/49)) ([879ced6](https://github.com/kunish/wheel/commit/879ced67f658b4fa4f9604595d8d4a60bccd4826))

## [1.12.1](https://github.com/kunish/wheel/compare/v1.12.0...v1.12.1) (2026-02-24)

### Bug Fixes

- optimize drag-and-drop interaction for channel and group sorting ([bcaa58d](https://github.com/kunish/wheel/commit/bcaa58da0db4d5be830f0e23e8ffa158ea339baf))

## [1.12.0](https://github.com/kunish/wheel/compare/v1.11.3...v1.12.0) (2026-02-24)

### Features

- improve model profiles UX and sync workflow ([7eb24b1](https://github.com/kunish/wheel/commit/7eb24b1b9c99a4f6f294d4cbe6e012480641dbba))

### Bug Fixes

- enable scrolling in log detail sheet panel ([29b99a5](https://github.com/kunish/wheel/commit/29b99a5aa9b651e6c572a86fb151b152f1ece728))

## [1.11.3](https://github.com/kunish/wheel/compare/v1.11.2...v1.11.3) (2026-02-24)

### Bug Fixes

- run ALTER before CREATE INDEX in migration to avoid missing column error ([63e9df6](https://github.com/kunish/wheel/commit/63e9df6c84144dd1599eb18887a442346ddbf241))

## [1.11.2](https://github.com/kunish/wheel/compare/v1.11.1...v1.11.2) (2026-02-24)

### Bug Fixes

- redistribute card padding to sub-components and remove overflow clip ([69b0572](https://github.com/kunish/wheel/commit/69b05722e41dc97cb3999516f147cb8863cffdbd))

## [1.11.1](https://github.com/kunish/wheel/compare/v1.11.0...v1.11.1) (2026-02-24)

### Bug Fixes

- code quality, UI consistency, and tooltip arrow issues ([45e6b67](https://github.com/kunish/wheel/commit/45e6b672b2de607c6833444f6ec0f6019e0b890d))

## [1.11.0](https://github.com/kunish/wheel/compare/v1.10.0...v1.11.0) (2026-02-24)

### Features

- add manual circuit breaker reset via settings page ([a87dac8](https://github.com/kunish/wheel/commit/a87dac8587d1f2e18a050f0ae06294b8ac307765))

## [1.10.0](https://github.com/kunish/wheel/compare/v1.9.2...v1.10.0) (2026-02-23)

### Features

- add version display and update check to settings page ([#41](https://github.com/kunish/wheel/issues/41)) ([df310cb](https://github.com/kunish/wheel/commit/df310cbff051866c440d9298fb31cedb4571a6fb))

## [1.9.2](https://github.com/kunish/wheel/compare/v1.9.1...v1.9.2) (2026-02-23)

### Bug Fixes

- persist fetchedModel when creating channels ([f9074da](https://github.com/kunish/wheel/commit/f9074da2fa67adea10642212342a0cf0ec703b88))

## [1.9.1](https://github.com/kunish/wheel/compare/v1.9.0...v1.9.1) (2026-02-23)

### Bug Fixes

- use backtick quoting for `order` column in channels DAL and models ([8543847](https://github.com/kunish/wheel/commit/8543847335cd051052cfc08ce8901f4c9ed701d7))

## [1.9.0](https://github.com/kunish/wheel/compare/v1.8.0...v1.9.0) (2026-02-23)

### Features

- reuse normal log components for streaming logs and collapse tool definitions ([1451b01](https://github.com/kunish/wheel/commit/1451b0146fe7705d923d4441b507eb38314756eb))

### Bug Fixes

- add TiDB service for screenshots CI job ([882d95c](https://github.com/kunish/wheel/commit/882d95cf45d3cdcb80dcea2c18912edcfc8f717f))
- use backtick quoting for `order` column in MySQL/TiDB queries ([4a533fa](https://github.com/kunish/wheel/commit/4a533fa257b6d4c01402d519b9e516a04c0c6548))

## [1.8.0](https://github.com/kunish/wheel/compare/v1.7.3...v1.8.0) (2026-02-23)

### Features

- auto-create database on startup ([13d6818](https://github.com/kunish/wheel/commit/13d68180f6897a4e8314bd768aaac337f3240afe))
- migrate database from SQLite to TiDB/MySQL ([34a0d3c](https://github.com/kunish/wheel/commit/34a0d3c494abb964d7c7246533428da1627ef1d2))
- **relay:** improve Anthropic conversion and add native Gemini support ([35f3a47](https://github.com/kunish/wheel/commit/35f3a475029b5daac24b0e043f00aca7129b911f))

### Bug Fixes

- add persistent volume for TiDB data ([95750d5](https://github.com/kunish/wheel/commit/95750d5c76e94edbd3afdbe8d619d4391f7ba2d3))
- quote `key` reserved word in MySQL queries ([0023550](https://github.com/kunish/wheel/commit/0023550a08808c5099e845cdecedaac06bfbe12b))
- update deployment config for MySQL migration and fix MEDIUMTEXT columns ([4c82779](https://github.com/kunish/wheel/commit/4c8277998ac80572ae6cd43c32b5949817615367))

## [1.7.3](https://github.com/kunish/wheel/compare/v1.7.2...v1.7.3) (2026-02-15)

### Bug Fixes

- center month view content in dashboard ([#32](https://github.com/kunish/wheel/issues/32)) ([5779db0](https://github.com/kunish/wheel/commit/5779db0e40be7b01ff1251d9d097970b5c071409))

## [1.7.2](https://github.com/kunish/wheel/compare/v1.7.1...v1.7.2) (2026-02-15)

### Bug Fixes

- measure TTFT from request start instead of response headers received ([#29](https://github.com/kunish/wheel/issues/29)) ([2d02b92](https://github.com/kunish/wheel/commit/2d02b92ad84822a01d2e0305c54df167c6897f2c))

## [1.7.1](https://github.com/kunish/wheel/compare/v1.7.0...v1.7.1) (2026-02-15)

### Bug Fixes

- allow all origins for WebSocket when ALLOWED_ORIGINS is not set ([#27](https://github.com/kunish/wheel/issues/27)) ([d264e16](https://github.com/kunish/wheel/commit/d264e1681386cb546b92f6c2cc713a9139597c13))

## [1.7.0](https://github.com/kunish/wheel/compare/v1.6.1...v1.7.0) (2026-02-15)

### Features

- add Prometheus and Grafana monitoring setup ([#23](https://github.com/kunish/wheel/issues/23)) ([b8a030e](https://github.com/kunish/wheel/commit/b8a030e496e6572cee4aad85978a594e479c4a5d))

### Bug Fixes

- **group:** enabled=false not persisted when updating group items ([#24](https://github.com/kunish/wheel/issues/24)) ([6010595](https://github.com/kunish/wheel/commit/60105959f4abd3957d9278a42c926af9b7a42464))

## [1.6.1](https://github.com/kunish/wheel/compare/v1.6.0...v1.6.1) (2026-02-14)

### Bug Fixes

- add missing canonical provider prefixes for model matching ([#21](https://github.com/kunish/wheel/issues/21)) ([46e7c56](https://github.com/kunish/wheel/commit/46e7c56231cd7a20184ab825b118cd1d8f87348c))

## [1.6.0](https://github.com/kunish/wheel/compare/v1.5.3...v1.6.0) (2026-02-14)

### Features

- **web:** add syntax highlighting, mermaid support, and streaming UX improvements ([05deee2](https://github.com/kunish/wheel/commit/05deee2670b37b3a03126b0e11525d45098dc2b6))
- **worker:** add Prometheus metrics and OpenTelemetry tracing ([0beac76](https://github.com/kunish/wheel/commit/0beac7622f70b71163415f288f89c76330ce9f93))
- **worker:** optimize async stats caching and non-blocking log submission ([79b3823](https://github.com/kunish/wheel/commit/79b3823dc8c63ea5d60cb66ce42018febe12c9ab))

### Bug Fixes

- **worker:** fix token extraction and streaming reliability ([d49b6b7](https://github.com/kunish/wheel/commit/d49b6b78d35b3961172cbb1616b60e26c1693163))

## [1.5.3](https://github.com/kunish/wheel/compare/v1.5.2...v1.5.3) (2026-02-13)

### Bug Fixes

- **ci:** add tsx as devDependency for screenshot script ([962b99f](https://github.com/kunish/wheel/commit/962b99f873c47b7d7e4be13887517d808c7bae34))

## [1.5.2](https://github.com/kunish/wheel/compare/v1.5.1...v1.5.2) (2026-02-13)

### Bug Fixes

- **ci:** use pnpm exec instead of npx for tsx ([4aa8a58](https://github.com/kunish/wheel/commit/4aa8a58c52d168d8e21f9a36f3993b236389b99f))

## [1.5.1](https://github.com/kunish/wheel/compare/v1.5.0...v1.5.1) (2026-02-13)

### Bug Fixes

- fix screenshot script failures ([663e047](https://github.com/kunish/wheel/commit/663e047ba46327595a9a5d4d796b8d41f2f0eb19))

## [1.5.0](https://github.com/kunish/wheel/compare/v1.4.1...v1.5.0) (2026-02-13)

### Features

- add mock data seed, automated screenshots, and CI integration ([36dfb1b](https://github.com/kunish/wheel/commit/36dfb1b457d2f51dd4ea477deae07928eb17ff00))

## [1.4.1](https://github.com/kunish/wheel/compare/v1.4.0...v1.4.1) (2026-02-13)

### Bug Fixes

- improve dashboard clock alignment and layout ([a539ad1](https://github.com/kunish/wheel/commit/a539ad12b2509919ac5a372d36b421ffe9f91890))

## [1.4.0](https://github.com/kunish/wheel/compare/v1.3.0...v1.4.0) (2026-02-13)

### Features

- add model source tracking with three-category display ([2dd46b2](https://github.com/kunish/wheel/commit/2dd46b287de79e0308e799bf93856a63f44a7af2))

### Bug Fixes

- resolve content overflow in edit group dialog ([469b8f8](https://github.com/kunish/wheel/commit/469b8f8fa21c03e9475cd08f41f4c0cf9e5823f2))

## [1.3.0](https://github.com/kunish/wheel/compare/v1.2.1...v1.3.0) (2026-02-13)

### Features

- add channel drag-and-drop reorder + Anthropic model fetch fallback ([5aa6f91](https://github.com/kunish/wheel/commit/5aa6f91b9641a2dda22e46459e69f2b7c05ae418))
- add enabled toggle for group items ([8b08ba5](https://github.com/kunish/wheel/commit/8b08ba5f0c21db49874214d9850a8674c5564268))

## [1.2.1](https://github.com/kunish/wheel/compare/v1.2.0...v1.2.1) (2026-02-13)

### Bug Fixes

- **ci:** set VITE_BASE_PATH for GitHub Pages deployment ([52755bb](https://github.com/kunish/wheel/commit/52755bb4859d6424a6681bb5e4ebb59c3df27505))

## [1.2.0](https://github.com/kunish/wheel/compare/v1.1.0...v1.2.0) (2026-02-12)

### Features

- **web:** support light/dark/system three-mode theme switching ([6907733](https://github.com/kunish/wheel/commit/69077337fc364ec9ca31cec4f856c9287087cec3))

## [1.1.0](https://github.com/kunish/wheel/compare/v1.0.0...v1.1.0) (2026-02-12)

### Features

- add OpenAPI docs, configurable API URL, and GitHub Pages deployment ([1529a68](https://github.com/kunish/wheel/commit/1529a68fa0f8b7f81fdfc632290fa6783be9bb46))
- **dashboard:** add data stats to gear clock hub center and fix popover flicker ([2461cc2](https://github.com/kunish/wheel/commit/2461cc25d62fcc2a854bd0702433a1c726bb3aed))

### Bug Fixes

- **worker:** align change-password API with frontend request format ([#10](https://github.com/kunish/wheel/issues/10)) ([642288a](https://github.com/kunish/wheel/commit/642288ad0f7f84c3e7f71cf9237a153b62c37538))

## 1.0.0 (2026-02-12)

### Features

- initial release ([bad5adb](https://github.com/kunish/wheel/commit/bad5adba2dc710a4833fe9ef9cf36ec7d6668a89))

### Bug Fixes

- pin Docker base images and add fail-fast: false ([e93bce2](https://github.com/kunish/wheel/commit/e93bce2d44c5bf0c1c3e0d8ba183ba4e8e0ac872))
