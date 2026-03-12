# Sub2api Core Port — OpenAI Codex & Antigravity Channels

**Date:** 2026-03-12
**Status:** Approved
**Scope:** Full port of sub2api patterns for OpenAI Codex CLI and Antigravity (Google internal Gemini API) channels into the wheel Go worker backend.

---

## 1. Overview

Port the sophisticated channel management patterns from [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) into the wheel project's Go worker backend. This adds full-featured support for two third-party channels:

- **OpenAI Codex CLI** — OAuth PKCE flow against auth0.openai.com, token refresh, model normalization (gpt-5.x family), request/response transformation for the ChatGPT internal Responses API.
- **Antigravity** — Google OAuth PKCE flow, token management with project_id backfill, Claude→Gemini request transformation (identity injection, thinking signatures, MCP XML protocol), Gemini→Claude response transformation (streaming + non-streaming), smart retry with URL-level fallback and model capacity exhaustion handling.

Both channels already have stub implementations in wheel (`OutboundCodexCLI = 35`, `OutboundAntigravity = 36`) and basic relay handlers. This spec describes enhancing them to production-grade quality.

---

## 2. Architecture

### 2.1 Layer Structure

```
┌─────────────────────────────────────────────┐
│  HTTP Layer (Gin handlers + relay strategy) │
├─────────────────────────────────────────────┤
│  Service Layer (OAuth, Token Provider,      │
│                 Token Refresher)            │
├─────────────────────────────────────────────┤
│  Core Packages (OAuth helpers, API clients, │
│                 request/response transform) │
├─────────────────────────────────────────────┤
│  Data Layer (codex_auth_files JSON creds,   │
│              bun ORM, Redis cache)          │
└─────────────────────────────────────────────┘
```

### 2.2 Mapping Sub2api to Wheel

| Sub2api Concept                         | Wheel Equivalent                         |
| --------------------------------------- | ---------------------------------------- |
| ent ORM + wire DI                       | bun ORM + direct construction            |
| `internal/pkg/antigravity/`             | `internal/antigravity/`                  |
| `internal/pkg/openai/` (codex parts)    | `internal/codexcli/`                     |
| `internal/service/*_oauth_service.go`   | `internal/service/*_oauth_service.go`    |
| `internal/service/*_token_provider.go`  | `internal/service/*_token_provider.go`   |
| `internal/service/*_token_refresher.go` | `internal/service/*_token_refresher.go`  |
| `internal/service/*_gateway_service.go` | `internal/handler/relay_*.go` (enhanced) |
| `ProxyRepository`                       | Direct HTTP client proxy config          |
| `AccountRepository`                     | `codex_auth_files` table via bun         |
| `GeminiTokenCache` (Redis)              | Wheel's existing Redis cache layer       |

---

## 3. Section 1 — Core Packages

### 3.1 `internal/antigravity/` (8 files)

#### `oauth.go` — OAuth Constants & PKCE Helpers

- Client ID: `1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com`
- Redirect URI: `http://localhost:8085/callback`
- Scopes: `cloud-platform`, `userinfo.email`, `userinfo.profile`, `cclog`, `experimentsandconfigs`
- PKCE helpers: `GenerateState()`, `GenerateCodeVerifier()`, `GenerateCodeChallenge()` (S256)
- `OAuthSession` struct with state, code_verifier, redirect_uri, created_at
- `SessionStore` (in-memory map with mutex, TTL cleanup goroutine)

#### `urls.go` — URL Management & Availability

- Production URL: `https://cloudcode-pa.googleapis.com`
- Daily URL: `https://daily-cloudcode-pa.sandbox.googleapis.com`
- `URLAvailabilityManager`: tracks per-URL rate limit state with cooldown windows
- `shouldFallbackToNextURL(statusCode, err)`: returns true for connection errors, 429, 408, 404, 5xx
- `GetAvailableURL()`: returns first non-rate-limited URL, with fallback

#### `client.go` — API Client

- `ExchangeCode(ctx, code, codeVerifier, redirectURI, proxyURL)` → token response
- `RefreshToken(ctx, refreshToken, proxyURL)` → token response
- `GetUserInfo(ctx, accessToken, proxyURL)` → user info (email, name)
- `LoadCodeAssist(ctx, accessToken, projectID, proxyURL)` → project config (with URL fallback)
- `OnboardUser(ctx, accessToken, projectID, proxyURL)` → onboarding result (with polling)
- `FetchAvailableModels(ctx, accessToken, projectID, proxyURL)` → model list

#### `request_transformer.go` — Claude → Gemini Request Transformation

- `TransformClaudeToGeminiWithOptions(reqBody, opts)` → transformed request body
- Identity injection: prepends `<identity>You are Antigravity...</identity>` to system instruction
- Model-specific identity: maps claude-opus-4-5, claude-opus-4-6, claude-sonnet-4-5, claude-sonnet-4-6, claude-haiku-4-5 to display names and canonical IDs
- `SYSTEM_PROMPT_END` boundary marker injection
- Thinking signature handling: Claude models use real signatures, Gemini models use `skip_thought_signature_validator`
- MCP XML protocol injection for tools prefixed with `mcp__`
- Schema cleaning: removes `additionalProperties` from nested schemas
- Field mapping: `messages` → `contents`, `system` → `systemInstruction`, `max_tokens` → `generationConfig.maxOutputTokens`, `tools` → Gemini tool format, `tool_choice` → `toolConfig`
- Wraps result in `V1InternalRequest` envelope with `model`, `project_id`, `request` fields

#### `response_transformer.go` — Gemini → Claude Non-Streaming Response

- `TransformGeminiToClaude(geminiResp, requestModel)` → Claude-format response
- `NonStreamingProcessor`: handles thinking blocks (with signature propagation), text blocks, functionCall → tool_use mapping
- `generateRandomID()` for content block IDs
- Grounding metadata extraction (search queries, web results)
- Token usage mapping: `promptTokenCount` → `input_tokens`, `candidatesTokenCount` → `output_tokens`, `cachedContentTokenCount` → `cache_read_input_tokens`

#### `stream_transformer.go` — Gemini → Claude Streaming Response

- `StreamingProcessor` with `ProcessLine(line)` → list of SSE events
- Handles `data: ` prefix parsing, accumulates partial JSON
- Emits Claude SSE events: `message_start`, `content_block_start`, `content_block_delta`, `content_block_stop`, `message_delta`, `message_stop`
- Thinking block streaming with signature handling
- Tool use streaming (function_call name + arguments)

#### `identity.go` — Identity Text & Model Info

- `antigravityIdentity` constant with full identity XML
- `modelInfoMap` mapping model IDs to display names and canonical IDs
- `GetIdentityText(model)` → identity text with model-specific info
- `GetModelDisplayName(model)` → human-readable model name

#### `errors.go` — Error Types

- `RateLimitError` with retry-after duration
- `ModelCapacityExhaustedError`
- `shouldRetry(statusCode)` helper
- Error classification from HTTP status codes and response bodies

### 3.2 `internal/codexcli/` (4 files)

#### `oauth.go` — OpenAI OAuth Constants & PKCE

- Auth URL: `https://auth0.openai.com/authorize`
- Token URL: `https://auth0.openai.com/oauth/token`
- Client IDs: `pdlLIX2Y72MIl2rhLhTE9VV9bN905kBh` (ChatGPT), sora variant
- PKCE helpers: `GenerateState()`, `GenerateCodeVerifier()`, `GenerateCodeChallenge()`
- `OAuthSession` struct with state, code_verifier, client_id, redirect_uri
- `SessionStore` with TTL cleanup
- `ParseIDToken(idToken)` → claims with user info (email, account_id, user_id, org_id, plan_type)
- `BuildAuthorizationURL(state, codeChallenge, redirectURI)` → auth URL

#### `client.go` — OAuth Token Client

- `ExchangeCode(ctx, code, codeVerifier, redirectURI, proxyURL, clientID)` → token response
- `RefreshToken(ctx, refreshToken, proxyURL)` → token response
- `RefreshTokenWithClientID(ctx, refreshToken, proxyURL, clientID)` → token response

#### `models.go` — Model Normalization

- `codexModelMap`: maps ~60 model name variants to canonical IDs
  - gpt-5.4, gpt-5.3-codex, gpt-5.2, gpt-5.2-codex, gpt-5.1, gpt-5.1-codex, gpt-5.1-codex-max, gpt-5.1-codex-mini, codex-mini-latest
  - Reasoning level suffixes: `-none`, `-low`, `-medium`, `-high`, `-xhigh`
- `NormalizeCodexModel(model)` → canonical model ID (fuzzy matching with substring fallback)
- `SupportsVerbosity(model)` → bool (gpt-5.3+ supports verbosity)

#### `transform.go` — Codex Request Transformation

- `ApplyCodexOAuthTransform(reqBody, isCodexCLI, isCompact)` → transform result
- Forces `store=false`, `stream=true` for ChatGPT internal API
- Strips unsupported parameters: `max_output_tokens`, `temperature`, `top_p`, `frequency_penalty`, `presence_penalty`
- Tool normalization: ChatCompletions-style `{type:"function", function:{...}}` → Responses-style `{type:"function", name:..., parameters:...}`
- Input filtering: removes `item_reference` and `id` (preserves for tool continuation chains)
- String input → message array conversion
- Default instructions injection when empty
- Tool continuation detection (`NeedsToolContinuation`)

---

## 4. Section 2 — Service Layer

### 4.1 `internal/service/antigravity_oauth_service.go`

- `AntigravityOAuthService` struct with session store, API client, proxy resolver
- `GenerateAuthURL(ctx, proxyID, redirectURI)` → auth URL + session ID
- `ExchangeCode(ctx, input)` → token info (with project_id retry via `loadProjectIDWithRetry` + `tryOnboardProjectID`)
- `RefreshToken(ctx, refreshToken, proxyURL)` → token info (exponential backoff: 1s, 2s, 4s)
- `ValidateRefreshToken(ctx, refreshToken, proxyURL)` → bool
- `RefreshAccountToken(ctx, account)` → token info (reads credentials from account, resolves proxy)
- `BuildAccountCredentials(tokenInfo)` → credentials map
- `FillProjectID(ctx, account)` → fills project_id if missing

### 4.2 `internal/service/openai_oauth_service.go`

- `OpenAIOAuthService` struct with session store, API client, proxy resolver
- `GenerateAuthURL(ctx, proxyID, redirectURI, platform)` → auth URL + session ID
- `ExchangeCode(ctx, input)` → token info (PKCE code exchange, ID token parsing for user info)
- `RefreshToken(ctx, refreshToken, proxyURL)` → token info
- `RefreshTokenWithClientID(ctx, refreshToken, proxyURL, clientID)` → token info
- `ExchangeSoraSessionToken(ctx, sessionToken, proxyID)` → token info (Sora session_token → access_token)
- `RefreshAccountToken(ctx, account)` → token info
- `BuildAccountCredentials(tokenInfo)` → credentials map

### 4.3 `internal/service/antigravity_token_provider.go`

- `AntigravityTokenProvider` struct with account repo, token cache, OAuth service, metrics
- `GetAccessToken(ctx, account)` → access_token string
- Flow: cache check → distributed lock → double-check cache → DB refresh → OAuth refresh → project_id backfill → version check → cache write
- Key constants: `antigravityTokenRefreshSkew = 3min`, `antigravityTokenCacheSkew = 5min`, `antigravityBackfillCooldown = 5min`
- Runtime metrics: refresh requests/success/failure, lock contention, wait samples/total/hit/miss

### 4.4 `internal/service/openai_token_provider.go`

- `OpenAITokenProvider` struct with account repo, token cache, OAuth service, metrics
- `GetAccessToken(ctx, account)` → access_token string
- Flow: cache check → distributed lock with jitter-based contention handling → DB refresh → OAuth refresh → version check → cache write
- Key constants: `openAITokenRefreshSkew = 3min`, `openAITokenCacheSkew = 5min`
- Lock contention: exponential backoff polling (20ms initial, 120ms max, 5 attempts, ±20% jitter)
- Short TTL (1 min) on refresh failure to avoid stale token caching

### 4.5 `internal/service/antigravity_token_refresher.go`

- Implements `TokenRefresher` interface: `CanRefresh(account)`, `NeedsRefresh(account)`, `Refresh(ctx, account)`
- Background refresh window: 15 minutes before expiry
- Runs via existing wheel background job scheduler

### 4.6 `internal/service/openai_token_refresher.go`

- Implements `TokenRefresher` interface for OpenAI OAuth accounts
- Skips Sora accounts (separate refresh chain)
- Background refresh window: 15 minutes before expiry

### 4.7 `internal/service/token_version_check.go`

- `CheckTokenVersion(ctx, account, repo)` → (latestAccount, isStale)
- Compares in-memory account version with DB version
- Prevents race conditions between async refresh tasks and request threads

---

## 5. Section 3 — Enhanced Relay Handlers

### 5.1 `internal/handler/relay_antigravity.go` (Major Enhancement)

Current state: basic proxy with simple `convertAnthropicToGemini()` / `convertGeminiToAnthropic()`.

Enhanced to:

- Use `antigravity.TransformClaudeToGeminiWithOptions()` for full request transformation
- Use `antigravity.NonStreamingProcessor` / `antigravity.StreamingProcessor` for response transformation
- Integrate `AntigravityTokenProvider.GetAccessToken()` for token management
- **Smart retry logic** (`handleSmartRetry`):
  - URL-level rate limit fallback (prod → daily, with 7s cooldown threshold)
  - Model-level rate limiting with account switching
  - `MODEL_CAPACITY_EXHAUSTED` retry: 60 attempts, 1s fixed interval, global dedup via `sync.Map`
  - Single-account in-place retry for transient errors

### 5.2 `internal/handler/relay_codexcli.go` (Major Enhancement)

Current state: basic proxy forwarding.

Enhanced to:

- Use `codexcli.ApplyCodexOAuthTransform()` for request transformation
- Use `codexcli.NormalizeCodexModel()` for model normalization
- Integrate `OpenAITokenProvider.GetAccessToken()` for token management
- Route to `chatgpt.com/backend-api/codex/responses` (OAuth accounts) or `api.openai.com/v1/responses` (API key accounts)
- Header whitelist filtering (allowed request/response headers only)
- Compact mode support for non-OAuth requests

### 5.3 `internal/handler/relay_strategy.go` (Minor Enhancement)

- Wire new token providers and OAuth services into relay dispatch
- Add smart retry wrapping for Antigravity relay attempts

### 5.4 `internal/handler/routes.go` (Minor Enhancement)

- Add management API endpoints for OAuth flows:
  - `POST /api/antigravity/oauth/authorize` — generate auth URL
  - `POST /api/antigravity/oauth/callback` — exchange code for tokens
  - `POST /api/codexcli/oauth/authorize` — generate auth URL
  - `POST /api/codexcli/oauth/callback` — exchange code for tokens
  - `POST /api/codexcli/oauth/refresh` — refresh token

---

## 6. Section 4 — Data Model & Configuration

### 6.1 Credential JSON Structure

No DB schema changes needed. The `codex_auth_files` table's JSON `credentials` column gains new fields.

**Antigravity credentials:**

```json
{
  "access_token": "ya29.a0...",
  "refresh_token": "1//0e...",
  "expires_at": "2026-03-12T15:00:00Z",
  "email": "user@gmail.com",
  "project_id": "extensions-XXXX",
  "project_id_backfilled_at": "2026-03-12T12:00:00Z"
}
```

**OpenAI Codex credentials:**

```json
{
  "access_token": "eyJhbGciOi...",
  "refresh_token": "v1.MjE...",
  "id_token": "eyJhbGciOi...",
  "expires_at": "2026-03-12T15:00:00Z",
  "client_id": "pdlLIX2Y72MIl2rhLhTE9VV9bN905kBh",
  "email": "user@example.com",
  "chatgpt_account_id": "...",
  "chatgpt_user_id": "...",
  "organization_id": "...",
  "plan_type": "plus"
}
```

### 6.2 Environment Variables

| Variable                          | Description                   | Default                                             |
| --------------------------------- | ----------------------------- | --------------------------------------------------- |
| `ANTIGRAVITY_PROD_URL`            | Production API URL            | `https://cloudcode-pa.googleapis.com`               |
| `ANTIGRAVITY_DAILY_URL`           | Daily sandbox URL             | `https://daily-cloudcode-pa.sandbox.googleapis.com` |
| `ANTIGRAVITY_RATE_LIMIT_COOLDOWN` | URL rate limit cooldown       | `7s`                                                |
| `ANTIGRAVITY_CAPACITY_RETRY_MAX`  | Model capacity retry attempts | `60`                                                |
| `ANTIGRAVITY_CAPACITY_RETRY_WAIT` | Capacity retry interval       | `1s`                                                |
| `CODEXCLI_CHATGPT_URL`            | ChatGPT Codex endpoint        | `https://chatgpt.com/backend-api/codex/responses`   |
| `CODEXCLI_PLATFORM_URL`           | OpenAI platform endpoint      | `https://api.openai.com/v1/responses`               |

---

## 7. File Inventory

### New Files (~22)

| #   | Path                                              | Purpose                              |
| --- | ------------------------------------------------- | ------------------------------------ |
| 1   | `internal/antigravity/oauth.go`                   | OAuth constants, PKCE, session store |
| 2   | `internal/antigravity/urls.go`                    | URL availability manager             |
| 3   | `internal/antigravity/client.go`                  | Antigravity API client               |
| 4   | `internal/antigravity/request_transformer.go`     | Claude → Gemini transform            |
| 5   | `internal/antigravity/response_transformer.go`    | Gemini → Claude non-streaming        |
| 6   | `internal/antigravity/stream_transformer.go`      | Gemini → Claude streaming            |
| 7   | `internal/antigravity/identity.go`                | Identity text & model info           |
| 8   | `internal/antigravity/errors.go`                  | Error types                          |
| 9   | `internal/codexcli/oauth.go`                      | OpenAI OAuth constants, PKCE         |
| 10  | `internal/codexcli/client.go`                     | OpenAI OAuth token client            |
| 11  | `internal/codexcli/models.go`                     | Model normalization map              |
| 12  | `internal/codexcli/transform.go`                  | Codex request transformation         |
| 13  | `internal/service/antigravity_oauth_service.go`   | Antigravity OAuth service            |
| 14  | `internal/service/openai_oauth_service.go`        | OpenAI OAuth service                 |
| 15  | `internal/service/antigravity_token_provider.go`  | Antigravity token provider           |
| 16  | `internal/service/openai_token_provider.go`       | OpenAI token provider                |
| 17  | `internal/service/antigravity_token_refresher.go` | Antigravity background refresher     |
| 18  | `internal/service/openai_token_refresher.go`      | OpenAI background refresher          |
| 19  | `internal/service/token_version_check.go`         | Token version check helper           |
| 20  | `internal/handler/antigravity_mgmt.go`            | Antigravity OAuth management API     |
| 21  | `internal/handler/codexcli_mgmt.go`               | CodexCLI OAuth management API        |
| 22  | `internal/handler/relay_retry.go`                 | Smart retry logic                    |

### Modified Files (~5)

| #   | Path                                    | Change                                                      |
| --- | --------------------------------------- | ----------------------------------------------------------- |
| 1   | `internal/handler/relay_antigravity.go` | Replace simple proxy with full transform + token provider   |
| 2   | `internal/handler/relay_codexcli.go`    | Add model normalization + codex transforms + token provider |
| 3   | `internal/handler/relay_strategy.go`    | Wire new services, add smart retry dispatch                 |
| 4   | `internal/handler/routes.go`            | Add OAuth management endpoints                              |
| 5   | `internal/types/enums.go`               | No changes needed (already has types 35, 36)                |

---

## 8. Key Design Decisions

1. **No new DB tables.** All credentials stored as JSON in existing `codex_auth_files` table. This avoids migration complexity and matches wheel's existing pattern.

2. **Separate packages for Antigravity and CodexCLI.** Despite both being "third-party channels," their OAuth flows, API protocols, and transformation logic are completely different. Shared abstractions would be forced.

3. **Token provider pattern replicated per channel.** Each channel gets its own provider with channel-specific refresh logic, cache keys, and TTL strategies. The pattern is the same (cache → lock → refresh → cache), but the details differ enough to warrant separate implementations.

4. **Smart retry lives in handler layer.** Retry logic depends on HTTP response details (status codes, headers, body) and channel-specific error classification. It wraps relay calls rather than being embedded in them.

5. **Identity injection is Antigravity-specific.** The `<identity>You are Antigravity...</identity>` text is only prepended for Antigravity channel requests. OpenAI Codex uses its own instructions handling.

6. **Streaming processor is stateful.** The `StreamingProcessor` maintains state across SSE lines (current content block index, accumulated thinking text, etc.) because Gemini streaming events don't map 1:1 to Claude events.

---

## 9. Risk Mitigation

| Risk                      | Mitigation                                                                |
| ------------------------- | ------------------------------------------------------------------------- |
| Upstream API changes      | URL/endpoint constants are configurable via env vars                      |
| Token refresh storms      | Distributed lock + double-check pattern prevents thundering herd          |
| Rate limit cascades       | URL availability manager with cooldown windows isolates failures          |
| Model capacity exhaustion | Bounded retry (60 attempts max) with global dedup prevents infinite loops |
| Stale token caching       | Version check before cache write + short TTL on refresh failure           |
| Large request bodies      | Stream transformation processes line-by-line, not buffering full response |
