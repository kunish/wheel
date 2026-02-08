## 1. 增量日志推送 — 后端

- [x] 1.1 修改 `createLog` (DAL) 返回插入的行（`.returning()`）
- [x] 1.2 修改 handler.ts 异步回调：广播 `log-created` 事件并携带日志摘要数据
- [x] 1.3 保留 `broadcast("stats-updated")` 但不再携带日志数据

## 2. 增量日志推送 — 前端

- [x] 2.1 修改 `use-stats-ws.ts`：`stats-updated` 不再 invalidate `["logs"]`
- [x] 2.2 修改日志页面：监听 `log-created` 事件，第 1 页无筛选时用 `setQueryData` 追加
- [x] 2.3 添加 "有新日志" 提示条：非第 1 页或有筛选条件时显示

## 3. 日志保留清理

- [x] 3.1 添加 `cleanupOldLogs` DAL 函数
- [x] 3.2 添加 `getSetting` 通用函数
- [x] 3.3 handler.ts 中 1/100 概率触发清理
- [x] 3.4 设置页面添加 `log_retention_days` 配置项（通过通用 System Config UI 自动支持）

## 4. 构建和验证

- [x] 4.1 TypeScript 类型检查通过
- [x] 4.2 构建 Docker 镜像并推送
