## 1. 后端：扩展 log-stream-start 事件 payload

- [x] 1.1 在 `apps/worker/internal/handler/relay.go` 的流式路径中，调用 `relay.CalculateCost` 相关的定价查询逻辑，获取当前目标模型的 `inputPrice` 和 `outputPrice`（每百万 token 单价），添加到 `streamStartPayload` 中
- [x] 1.2 在 `streamStartPayload` 中增加 `estimatedInputTokens` 字段，基于请求体（`body` map 序列化后的字符数 / 3）粗略估算输入 token 数
- [x] 1.3 抽取定价查询为独立函数（如 `LookupModelPrice`），使其可被 `log-stream-start` 和 `CalculateCost` 共用，避免重复代码

## 2. 后端：扩展 log-streaming 事件 payload

- [x] 2.1 在 `apps/worker/internal/relay/proxy.go` 的 `StreamContentCallback` 类型中，将签名扩展为 `func(thinking, response string, thinkingLen, responseLen int)`，传递累计字符长度
- [x] 2.2 在 `apps/worker/internal/handler/relay.go` 的 `onContent` 回调中，将 `thinkingLength` 和 `responseLength` 添加到 `log-streaming` 事件的 payload 中

## 3. 前端：扩展 pending 流式条目数据模型

- [x] 3.1 在 `apps/web/src/pages/logs.tsx` 的 `log-stream-start` 事件处理器中，从 payload 读取 `estimatedInputTokens`、`inputPrice`、`outputPrice`，存入 pending 条目（分别作为 `inputTokens`、`_inputPrice`、`_outputPrice` 字段）
- [x] 3.2 在 `log-streaming` 事件处理器中，根据 `responseLength + thinkingLength` 估算 `outputTokens`（除以 3 取整），并用 `(estimatedInputTokens × inputPrice + outputTokens × outputPrice) / 1_000_000` 计算 `cost`，更新 pending 条目

## 4. 前端：pending 条目预估值视觉区分

- [x] 4.1 在日志表格的 token 数和费用列渲染逻辑中，对 `_streaming: true` 的行应用 `opacity-50` 样式，暗示数值为预估值
- [x] 4.2 确认 `log-created` 事件替换 pending 条目后，新行以正常透明度渲染

## 5. 前端：Dashboard 增量状态管理

- [x] 5.1 在 `apps/web/src/pages/dashboard.tsx` 中创建一个 `useRef` 或 `useState` 管理的增量状态 Map（`Map<streamId, { estimatedInputTokens, outputTokens, cost }>`）
- [x] 5.2 监听 `log-stream-start` 事件：在增量 Map 中创建条目，记录 `estimatedInputTokens`、`inputPrice`、`outputPrice`，初始 `outputTokens` 和 `cost` 为 0
- [x] 5.3 监听 `log-streaming` 事件：更新对应条目的 `outputTokens`（内容长度估算）和 `cost`（重新计算）
- [x] 5.4 监听 `log-created` 和 `log-stream-end` 事件：从增量 Map 中移除对应 `streamId` 的条目

## 6. 前端：Dashboard 统计卡片叠加增量

- [x] 6.1 计算增量汇总值：遍历增量 Map，汇总所有条目的 `estimatedInputTokens`、`outputTokens`、`cost`
- [x] 6.2 修改统计卡片的数据源：将 React Query 缓存的 stats 值（`input_token`、`output_token`、`input_cost`、`output_cost`）与增量汇总值相加后传给 `AnimatedNumber` 组件
- [x] 6.3 验证流式完成后的过渡：增量条目移除 + `stats-updated` 触发全量刷新，`AnimatedNumber` 应自动实现平滑动画过渡
