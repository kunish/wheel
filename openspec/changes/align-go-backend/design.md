## Context

当前 TypeScript 后端是从 Go 项目 bestruirui/wheel 移植而来，但在移植过程中引入了与上游不一致的逻辑：proxy 层添加了 Go 版本没有的 429 重试循环（MAX_429_RETRIES=5），handler 层添加了指数退避，Base URL 随机选择而非按延迟排序，429 冷却期从 5 分钟降到 60 秒，且缺少 Anthropic 缓存 token 定价。

## Goals / Non-Goals

**Goals:**

- 对齐 proxy 层逻辑：单次 fetch，无重试
- 对齐 handler 重试逻辑：无指数退避，直接尝试下一个 channel
- 对齐 Base URL 选择：按 delay 升序，选最低
- 恢复 429 冷却期为 5 分钟，保留 fallback（所有 key 都冷却时选最旧的）
- 添加 Anthropic 缓存 token 定价
- 补全 OutboundType 枚举

**Non-Goals:**

- 不重写 adapter 层的格式转换逻辑（已验证可用）
- 不改变 streaming SSE 转换逻辑
- 不改变日志结构或 attempt tracking（这是相对 Go 的增强）
- 不迁移到 Go 版的 in-memory cache 架构（保持 KV cache）

## Decisions

1. **移除 proxy 层 429 重试**：Go 版 proxy 只做单次 HTTP 请求，错误直接抛出由 handler 处理。双层重试增加了不必要的延迟（proxy 429 重试最多等 5×1s=5s，handler 再等指数退避）。改为 proxy 单次 fetch，非 2xx 即抛 ProxyError。

2. **handler 重试无 sleep**：Go 版 3 轮重试之间无延迟。当前实现在检测到 429 后会 sleep `2^(round-2)` 秒，这在只有一个 channel 时会导致 1-8 秒的无效等待。移除所有 sleep。

3. **Base URL 选 delay 最低**：Go 版 `GetBaseUrl()` 选 delay 最低的 URL。delay 代表 ping 时间，选最低延迟的 URL 能获得最佳性能。

4. **429 冷却期 5 分钟 + fallback**：恢复 Go 的 5 分钟冷却期，但保留 TS 独有的 fallback 机制（所有 key 冷却时选最旧的）。Go 版在所有 key 冷却时会返回空导致静默失败。

5. **LLM Price 表添加 cache 字段**：需要新增 `cacheReadPrice` 和 `cacheWritePrice` 列。定价公式对齐 Go：对 Anthropic 响应中的 cache tokens 使用对应价格。

## Risks / Trade-offs

- [移除 proxy 429 重试] → 如果上游频繁 429 且只有一个 key，请求会更快失败。但 handler 的 3 轮重试 + key cooldown 已提供足够保护。
- [移除 sleep] → 如果上游短暂过载，可能在毫秒内耗尽所有重试。但 Go 版本也是这样工作的，实践中 3 轮 × N channels 足够。
- [5 分钟冷却期] → fallback 机制确保不会完全拒绝请求，只是优先用未冷却的 key。
