## Why

当前 TypeScript 后端在核心 relay 逻辑上偏离了上游 Go 实现（bestruirui/wheel），引入了不必要的复杂度（proxy 层 429 重试、指数退避）和遗漏（Base URL 选择忽略 delay、缺少 Anthropic 缓存 token 定价、缺少 channel 类型）。需要对齐以确保行为一致性和降低维护成本。

## What Changes

- **移除 proxy 层 429 重试循环** — Go 版只在 handler 层做 channel 级别重试，proxy 只做单次 fetch
- **移除 handler 重试间指数退避** — Go 版不在重试之间 sleep，直接尝试下一个 channel
- **修复 Base URL 选择** — 从随机选择改为选择 delay 最低的 URL（与 Go 一致）
- **恢复 429 冷却期为 5 分钟** — 与 Go 的 `RATE_LIMIT_COOLDOWN = 5 * 60` 一致，保留 fallback 机制
- **添加 Anthropic 缓存 token 定价** — 支持 `cache_read` 和 `cache_write` 价格字段
- **补全 channel 类型** — 添加 `OpenAIChat (0)` 和 `OpenAIEmbedding (5)`
- **简化 proxy.ts** — 移除 429 重试循环和 passthrough 参数，单次 fetch + 错误抛出

## Capabilities

### New Capabilities

- `simplify-proxy`: 简化 proxy 层为单次 fetch，移除 429 重试循环和 passthrough 参数
- `align-retry-logic`: 对齐 handler 重试逻辑，移除指数退避
- `align-key-selector`: 恢复 429 冷却期并修复 Base URL 选择策略
- `align-pricing`: 添加 Anthropic 缓存 token 定价支持和完整价格字段

### Modified Capabilities

## Impact

- `apps/worker/src/relay/proxy.ts` — 大幅简化，移除 ~100 行 429 重试代码
- `apps/worker/src/relay/handler.ts` — 移除指数退避逻辑
- `apps/worker/src/relay/adapter.ts` — 修改 Base URL 选择函数
- `apps/worker/src/relay/key-selector.ts` — 恢复冷却期
- `apps/worker/src/relay/pricing.ts` — 添加缓存 token 支持
- `apps/worker/src/db/schema.ts` — 可能需要为 LLM price 添加 cache_read/cache_write 字段
- `packages/core/src/types/enums.ts` — 添加缺失的 OutboundType 值
