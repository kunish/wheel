## 1. 简化 Proxy 层

- [x] 1.1 重写 `proxyNonStreaming`：移除 429 重试循环，单次 fetch，非 2xx 抛 ProxyError
- [x] 1.2 重写 `proxyStreaming`：移除 429 重试循环，单次 fetch，非 2xx reject firstChunkPromise
- [x] 1.3 移除 `passthrough` 参数（proxyNonStreaming 和 proxyStreaming）
- [x] 1.4 保留 `parseRetryDelay` 工具函数（ProxyError 仍需 retryAfterMs）

## 2. 对齐 Handler 重试逻辑

- [x] 2.1 移除 `had429`、`retryDelay` 变量和指数退避 sleep 逻辑
- [x] 2.2 确保 3 轮重试之间无任何 delay，直接 continue
- [x] 2.3 保留 429 key marking 和成功后 status 重置逻辑

## 3. 对齐 Key 选择和 Base URL

- [x] 3.1 修改 `selectBaseUrl`：从随机选择改为按 delay 升序排序，选最低
- [x] 3.2 恢复 `RATE_LIMIT_COOLDOWN` 为 300 秒（5 分钟）
- [x] 3.3 保留 fallback 机制（所有 key 冷却时选最旧的）

## 4. 补全 OutboundType

- [x] 4.1 在 `packages/core/src/types/enums.ts` 添加 `OpenAIChat = 0` 和 `OpenAIEmbedding = 5`
- [x] 4.2 检查现有代码中 OutboundType 的使用，确保新增值不引起回归

## 5. 对齐定价逻辑

- [x] 5.1 在 `llmPrices` schema 添加 `cacheReadPrice` 和 `cacheWritePrice` 字段
- [x] 5.2 生成 Drizzle 迁移文件
- [x] 5.3 修改 `pricing.ts` 的 `calculateCost`：支持 Anthropic cache tokens
- [x] 5.4 修改价格同步逻辑：从 models.dev 提取 cache_read/cache_write 价格

## 6. 构建和测试

- [x] 6.1 TypeScript 类型检查通过
- [x] 6.2 构建 amd64 Docker 镜像并推送
- [ ] 6.3 端到端测试：Anthropic 流式/非流式 + OpenAI 流式/非流式
