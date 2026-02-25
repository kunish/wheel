# Changelog

## [1.13.0](https://github.com/kunish/wheel/compare/v1.12.4...v1.13.0) (2026-02-25)


### Features

* **worker:** create default API key on first startup ([53e7b67](https://github.com/kunish/wheel/commit/53e7b67b99851ca84b81507f3eb07fbbdee4eb06))

## [1.12.4](https://github.com/kunish/wheel/compare/v1.12.3...v1.12.4) (2026-02-25)


### Bug Fixes

* **docker:** rename container user from 'wheel' to 'app' to avoid Alpine built-in group conflict ([afd1bee](https://github.com/kunish/wheel/commit/afd1beebf4ec77bfdac45c39535697b8204e3636))

## [1.12.3](https://github.com/kunish/wheel/compare/v1.12.2...v1.12.3) (2026-02-25)


### Bug Fixes

* **ui:** remove jarring zoom animation from tabs content ([1632bb4](https://github.com/kunish/wheel/commit/1632bb496521a783949aa59d14a8263b16dd14ed))

## [1.12.2](https://github.com/kunish/wheel/compare/v1.12.1...v1.12.2) (2026-02-24)


### Bug Fixes

* optimize materialize/unmaterialize selection interaction ([#49](https://github.com/kunish/wheel/issues/49)) ([879ced6](https://github.com/kunish/wheel/commit/879ced67f658b4fa4f9604595d8d4a60bccd4826))

## [1.12.1](https://github.com/kunish/wheel/compare/v1.12.0...v1.12.1) (2026-02-24)


### Bug Fixes

* optimize drag-and-drop interaction for channel and group sorting ([bcaa58d](https://github.com/kunish/wheel/commit/bcaa58da0db4d5be830f0e23e8ffa158ea339baf))

## [1.12.0](https://github.com/kunish/wheel/compare/v1.11.3...v1.12.0) (2026-02-24)


### Features

* improve model profiles UX and sync workflow ([7eb24b1](https://github.com/kunish/wheel/commit/7eb24b1b9c99a4f6f294d4cbe6e012480641dbba))


### Bug Fixes

* enable scrolling in log detail sheet panel ([29b99a5](https://github.com/kunish/wheel/commit/29b99a5aa9b651e6c572a86fb151b152f1ece728))

## [1.11.3](https://github.com/kunish/wheel/compare/v1.11.2...v1.11.3) (2026-02-24)


### Bug Fixes

* run ALTER before CREATE INDEX in migration to avoid missing column error ([63e9df6](https://github.com/kunish/wheel/commit/63e9df6c84144dd1599eb18887a442346ddbf241))

## [1.11.2](https://github.com/kunish/wheel/compare/v1.11.1...v1.11.2) (2026-02-24)


### Bug Fixes

* redistribute card padding to sub-components and remove overflow clip ([69b0572](https://github.com/kunish/wheel/commit/69b05722e41dc97cb3999516f147cb8863cffdbd))

## [1.11.1](https://github.com/kunish/wheel/compare/v1.11.0...v1.11.1) (2026-02-24)


### Bug Fixes

* code quality, UI consistency, and tooltip arrow issues ([45e6b67](https://github.com/kunish/wheel/commit/45e6b672b2de607c6833444f6ec0f6019e0b890d))

## [1.11.0](https://github.com/kunish/wheel/compare/v1.10.0...v1.11.0) (2026-02-24)


### Features

* add manual circuit breaker reset via settings page ([a87dac8](https://github.com/kunish/wheel/commit/a87dac8587d1f2e18a050f0ae06294b8ac307765))

## [1.10.0](https://github.com/kunish/wheel/compare/v1.9.2...v1.10.0) (2026-02-23)


### Features

* add version display and update check to settings page ([#41](https://github.com/kunish/wheel/issues/41)) ([df310cb](https://github.com/kunish/wheel/commit/df310cbff051866c440d9298fb31cedb4571a6fb))

## [1.9.2](https://github.com/kunish/wheel/compare/v1.9.1...v1.9.2) (2026-02-23)


### Bug Fixes

* persist fetchedModel when creating channels ([f9074da](https://github.com/kunish/wheel/commit/f9074da2fa67adea10642212342a0cf0ec703b88))

## [1.9.1](https://github.com/kunish/wheel/compare/v1.9.0...v1.9.1) (2026-02-23)


### Bug Fixes

* use backtick quoting for `order` column in channels DAL and models ([8543847](https://github.com/kunish/wheel/commit/8543847335cd051052cfc08ce8901f4c9ed701d7))

## [1.9.0](https://github.com/kunish/wheel/compare/v1.8.0...v1.9.0) (2026-02-23)


### Features

* reuse normal log components for streaming logs and collapse tool definitions ([1451b01](https://github.com/kunish/wheel/commit/1451b0146fe7705d923d4441b507eb38314756eb))


### Bug Fixes

* add TiDB service for screenshots CI job ([882d95c](https://github.com/kunish/wheel/commit/882d95cf45d3cdcb80dcea2c18912edcfc8f717f))
* use backtick quoting for `order` column in MySQL/TiDB queries ([4a533fa](https://github.com/kunish/wheel/commit/4a533fa257b6d4c01402d519b9e516a04c0c6548))

## [1.8.0](https://github.com/kunish/wheel/compare/v1.7.3...v1.8.0) (2026-02-23)


### Features

* auto-create database on startup ([13d6818](https://github.com/kunish/wheel/commit/13d68180f6897a4e8314bd768aaac337f3240afe))
* migrate database from SQLite to TiDB/MySQL ([34a0d3c](https://github.com/kunish/wheel/commit/34a0d3c494abb964d7c7246533428da1627ef1d2))
* **relay:** improve Anthropic conversion and add native Gemini support ([35f3a47](https://github.com/kunish/wheel/commit/35f3a475029b5daac24b0e043f00aca7129b911f))


### Bug Fixes

* add persistent volume for TiDB data ([95750d5](https://github.com/kunish/wheel/commit/95750d5c76e94edbd3afdbe8d619d4391f7ba2d3))
* quote `key` reserved word in MySQL queries ([0023550](https://github.com/kunish/wheel/commit/0023550a08808c5099e845cdecedaac06bfbe12b))
* update deployment config for MySQL migration and fix MEDIUMTEXT columns ([4c82779](https://github.com/kunish/wheel/commit/4c8277998ac80572ae6cd43c32b5949817615367))

## [1.7.3](https://github.com/kunish/wheel/compare/v1.7.2...v1.7.3) (2026-02-15)


### Bug Fixes

* center month view content in dashboard ([#32](https://github.com/kunish/wheel/issues/32)) ([5779db0](https://github.com/kunish/wheel/commit/5779db0e40be7b01ff1251d9d097970b5c071409))

## [1.7.2](https://github.com/kunish/wheel/compare/v1.7.1...v1.7.2) (2026-02-15)


### Bug Fixes

* measure TTFT from request start instead of response headers received ([#29](https://github.com/kunish/wheel/issues/29)) ([2d02b92](https://github.com/kunish/wheel/commit/2d02b92ad84822a01d2e0305c54df167c6897f2c))

## [1.7.1](https://github.com/kunish/wheel/compare/v1.7.0...v1.7.1) (2026-02-15)


### Bug Fixes

* allow all origins for WebSocket when ALLOWED_ORIGINS is not set ([#27](https://github.com/kunish/wheel/issues/27)) ([d264e16](https://github.com/kunish/wheel/commit/d264e1681386cb546b92f6c2cc713a9139597c13))

## [1.7.0](https://github.com/kunish/wheel/compare/v1.6.1...v1.7.0) (2026-02-15)


### Features

* add Prometheus and Grafana monitoring setup ([#23](https://github.com/kunish/wheel/issues/23)) ([b8a030e](https://github.com/kunish/wheel/commit/b8a030e496e6572cee4aad85978a594e479c4a5d))


### Bug Fixes

* **group:** enabled=false not persisted when updating group items ([#24](https://github.com/kunish/wheel/issues/24)) ([6010595](https://github.com/kunish/wheel/commit/60105959f4abd3957d9278a42c926af9b7a42464))

## [1.6.1](https://github.com/kunish/wheel/compare/v1.6.0...v1.6.1) (2026-02-14)


### Bug Fixes

* add missing canonical provider prefixes for model matching ([#21](https://github.com/kunish/wheel/issues/21)) ([46e7c56](https://github.com/kunish/wheel/commit/46e7c56231cd7a20184ab825b118cd1d8f87348c))

## [1.6.0](https://github.com/kunish/wheel/compare/v1.5.3...v1.6.0) (2026-02-14)


### Features

* **web:** add syntax highlighting, mermaid support, and streaming UX improvements ([05deee2](https://github.com/kunish/wheel/commit/05deee2670b37b3a03126b0e11525d45098dc2b6))
* **worker:** add Prometheus metrics and OpenTelemetry tracing ([0beac76](https://github.com/kunish/wheel/commit/0beac7622f70b71163415f288f89c76330ce9f93))
* **worker:** optimize async stats caching and non-blocking log submission ([79b3823](https://github.com/kunish/wheel/commit/79b3823dc8c63ea5d60cb66ce42018febe12c9ab))


### Bug Fixes

* **worker:** fix token extraction and streaming reliability ([d49b6b7](https://github.com/kunish/wheel/commit/d49b6b78d35b3961172cbb1616b60e26c1693163))

## [1.5.3](https://github.com/kunish/wheel/compare/v1.5.2...v1.5.3) (2026-02-13)


### Bug Fixes

* **ci:** add tsx as devDependency for screenshot script ([962b99f](https://github.com/kunish/wheel/commit/962b99f873c47b7d7e4be13887517d808c7bae34))

## [1.5.2](https://github.com/kunish/wheel/compare/v1.5.1...v1.5.2) (2026-02-13)


### Bug Fixes

* **ci:** use pnpm exec instead of npx for tsx ([4aa8a58](https://github.com/kunish/wheel/commit/4aa8a58c52d168d8e21f9a36f3993b236389b99f))

## [1.5.1](https://github.com/kunish/wheel/compare/v1.5.0...v1.5.1) (2026-02-13)


### Bug Fixes

* fix screenshot script failures ([663e047](https://github.com/kunish/wheel/commit/663e047ba46327595a9a5d4d796b8d41f2f0eb19))

## [1.5.0](https://github.com/kunish/wheel/compare/v1.4.1...v1.5.0) (2026-02-13)


### Features

* add mock data seed, automated screenshots, and CI integration ([36dfb1b](https://github.com/kunish/wheel/commit/36dfb1b457d2f51dd4ea477deae07928eb17ff00))

## [1.4.1](https://github.com/kunish/wheel/compare/v1.4.0...v1.4.1) (2026-02-13)


### Bug Fixes

* improve dashboard clock alignment and layout ([a539ad1](https://github.com/kunish/wheel/commit/a539ad12b2509919ac5a372d36b421ffe9f91890))

## [1.4.0](https://github.com/kunish/wheel/compare/v1.3.0...v1.4.0) (2026-02-13)


### Features

* add model source tracking with three-category display ([2dd46b2](https://github.com/kunish/wheel/commit/2dd46b287de79e0308e799bf93856a63f44a7af2))


### Bug Fixes

* resolve content overflow in edit group dialog ([469b8f8](https://github.com/kunish/wheel/commit/469b8f8fa21c03e9475cd08f41f4c0cf9e5823f2))

## [1.3.0](https://github.com/kunish/wheel/compare/v1.2.1...v1.3.0) (2026-02-13)


### Features

* add channel drag-and-drop reorder + Anthropic model fetch fallback ([5aa6f91](https://github.com/kunish/wheel/commit/5aa6f91b9641a2dda22e46459e69f2b7c05ae418))
* add enabled toggle for group items ([8b08ba5](https://github.com/kunish/wheel/commit/8b08ba5f0c21db49874214d9850a8674c5564268))

## [1.2.1](https://github.com/kunish/wheel/compare/v1.2.0...v1.2.1) (2026-02-13)


### Bug Fixes

* **ci:** set VITE_BASE_PATH for GitHub Pages deployment ([52755bb](https://github.com/kunish/wheel/commit/52755bb4859d6424a6681bb5e4ebb59c3df27505))

## [1.2.0](https://github.com/kunish/wheel/compare/v1.1.0...v1.2.0) (2026-02-12)


### Features

* **web:** support light/dark/system three-mode theme switching ([6907733](https://github.com/kunish/wheel/commit/69077337fc364ec9ca31cec4f856c9287087cec3))

## [1.1.0](https://github.com/kunish/wheel/compare/v1.0.0...v1.1.0) (2026-02-12)


### Features

* add OpenAPI docs, configurable API URL, and GitHub Pages deployment ([1529a68](https://github.com/kunish/wheel/commit/1529a68fa0f8b7f81fdfc632290fa6783be9bb46))
* **dashboard:** add data stats to gear clock hub center and fix popover flicker ([2461cc2](https://github.com/kunish/wheel/commit/2461cc25d62fcc2a854bd0702433a1c726bb3aed))


### Bug Fixes

* **worker:** align change-password API with frontend request format ([#10](https://github.com/kunish/wheel/issues/10)) ([642288a](https://github.com/kunish/wheel/commit/642288ad0f7f84c3e7f71cf9237a153b62c37538))

## 1.0.0 (2026-02-12)


### Features

* initial release ([bad5adb](https://github.com/kunish/wheel/commit/bad5adba2dc710a4833fe9ef9cf36ec7d6668a89))


### Bug Fixes

* pin Docker base images and add fail-fast: false ([e93bce2](https://github.com/kunish/wheel/commit/e93bce2d44c5bf0c1c3e0d8ba183ba4e8e0ac872))
