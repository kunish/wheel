## Context

Wheel 是一个 LLM API 网关，代理多个 LLM 提供商的请求。当前架构中，流式请求的数据流分为两个阶段：

1. **流式阶段**：后端通过 `log-streaming` WebSocket 事件推送内容片段（每 100 字符触发一次），前端在日志页面显示 pending 条目，但 token 数和费用始终为 0
2. **完成阶段**：后端从上游 SSE 流的最终 chunk 中提取 usage 数据（token 数），计算费用，写入数据库，然后通过 `log-created` 和 `stats-updated` 事件一次性推送最终数据

问题在于：token 计数来自上游 API 的 SSE 响应中的 usage 数据（Anthropic 的 `message_delta`、OpenAI 的最终 chunk），这些数据只在流结束时才可用。因此在流式过程中，前端无法显示任何 token/费用信息。

约束条件：

- 上游 API 的 token 计数只在流结束时提供，无法提前获取精确值
- `onContent` 回调已经以 100 字符为粒度推送内容，可复用这个通道
- Dashboard 通过 React Query 的 `invalidateQueries` 刷新，每次刷新是 5 个 API 请求
- WebSocket Hub 已有 `activeStreams` 追踪机制

## Goals / Non-Goals

**Goals:**

- 流式请求过程中，日志列表的 pending 条目实时显示**预估** token 数和费用
- 流式请求过程中，Dashboard 统计卡片实时反映正在进行中请求的 token/费用增量
- 流式请求完成时，预估值平滑过渡到精确值（无突变感）
- 最小化后端变更，尽量在前端完成预估逻辑

**Non-Goals:**

- 不追求流式过程中的精确 token 计数（上游 API 不支持）
- 不修改 REST API 端点
- 不改变非流式请求的处理逻辑
- 不新增数据库表或字段
- 不在后端做 token 估算（避免增加代理延迟）

## Decisions

### Decision 1: 前端侧 token 估算 vs 后端侧 token 估算

**选择：前端估算**

- **前端估算**：根据 `log-streaming` 事件推送的 `responseContent` 和 `thinkingContent` 长度，在前端用字符数/token 比率估算 token 数
- **后端估算**：在 Go 代理代码中引入 tokenizer 库，在流式处理循环中精确计数
- **理由**：后端引入 tokenizer 会增加代理延迟和内存开销，而且不同模型的 tokenizer 不同（tiktoken vs Claude tokenizer）。前端估算虽不精确，但对用户体验已经足够——关键是让数字"动起来"，而非精确到个位数。流结束后会被精确值替换。

### Decision 2: 费用估算的实现位置

**选择：扩展 `log-streaming` 事件 payload，后端推送模型定价信息**

- **方案 A**：前端硬编码定价表 → 维护成本高，前后端不一致
- **方案 B**：后端在 `log-stream-start` 事件中附带当前模型的 input/output 单价 → 前端根据估算 token 数自行计算费用
- **选择方案 B**：在 `log-stream-start` payload 中增加 `inputPrice` 和 `outputPrice` 字段（每百万 token 价格），前端用 `estimatedTokens × price / 1_000_000` 计算预估费用。这样定价数据只传一次，且与后端 `CalculateCost` 使用相同的价格源。

### Decision 3: Dashboard 实时更新策略

**选择：WebSocket 增量叠加 + 完成时修正**

- **方案 A**：流式过程中高频触发 `stats-updated` → 每次 5 个 API 请求，性能差
- **方案 B**：前端维护一个"进行中请求的增量"状态，叠加到已缓存的 stats 数据上显示；当 `log-created` 到达时移除增量，让 `stats-updated` 的全量刷新接管
- **选择方案 B**：零额外 API 请求，前端通过 `useWsEvent` 监听 `log-streaming` 事件更新增量状态，Dashboard 组件将缓存的 stats + 增量 = 显示值。`AnimatedNumber` 组件天然支持平滑过渡。

### Decision 4: token 估算比率

**选择：统一使用 1 token ≈ 4 字符（英文）/ 1.5 字符（中文）的近似值**

实际上，更简单的方案是使用一个固定比率 `1 token ≈ 3 字符`作为跨语言平均估算。这个数字不需要精确，因为：

- 流结束后会被精确值替换
- 目的是给用户一个"数字在增长"的感知，而非精确预算

### Decision 5: 输入 token 的处理

**选择：在 `log-stream-start` 时发送预估输入 token 数**

输入 token 数在请求开始时就可以估算（基于请求 body 的大小），后端可以在 `log-stream-start` 事件中附带一个基于请求体大小的粗略估算值。这样前端从一开始就能显示输入 token 的近似值，而不必等到流结束。

## Risks / Trade-offs

**[预估不准确]** → 使用 `≈` 前缀或半透明样式暗示数字是预估值，流结束后自动替换为精确值。`AnimatedNumber` 的动画过渡可以消除突变感。

**[Dashboard 增量状态一致性]** → 当浏览器标签不可见时 WebSocket 消息仍会到达，增量状态可能与实际不一致。通过 `log-created` 事件清除对应流的增量来自我修正，且 `stats-updated` 的全量刷新最终保证一致性。

**[并发流式请求]** → 多个同时进行的流式请求都会叠加增量。前端增量状态需要按 `streamId` 独立追踪，互不干扰。

**[定价缺失]** → 如果模型没有配置定价，`log-stream-start` 中 `inputPrice`/`outputPrice` 为 0，前端显示费用为 $0。与当前行为一致（最终 cost 也为 0）。
