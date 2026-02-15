## ADDED Requirements

### Requirement: 统一使用 OpenAPI 生成的 API 客户端

所有页面和组件 SHALL 使用 `api-client.ts`（OpenAPI 生成）进行 API 调用。手写的 `api.ts` SHALL 在迁移完成后删除。

#### Scenario: 新代码使用 api-client

- **WHEN** 开发者需要调用后端 API
- **THEN** SHALL 使用 `api-client.ts` 中的类型安全函数，不使用 `api.ts`

#### Scenario: 迁移完成后 api.ts 删除

- **WHEN** 所有页面迁移到 api-client.ts 完成
- **THEN** `lib/api.ts` SHALL 被删除，不保留任何函数

### Requirement: 统计类型定义独立文件

`api.ts` 中内联定义的类型（`StatsMetrics`、`StatsDaily`、`StatsHourly`、`ModelStatsItem` 等）SHALL 提取到 `lib/types/stats.ts`。

#### Scenario: 类型从独立文件导入

- **WHEN** 组件需要使用统计相关类型
- **THEN** SHALL 从 `@/lib/types/stats` 导入，不从 api.ts 导入

### Requirement: 所有 API 调用使用统一的错误处理

迁移后的 API 调用 SHALL 统一通过 `api-client.ts` 的错误处理机制。当前 `api.ts` 中 `exportData`、`importData`、`replayLog` 绕过 `apiFetch` 直接使用 `fetch` 的模式 SHALL 消除。

#### Scenario: 导出数据使用统一客户端

- **WHEN** 用户触发数据导出
- **THEN** 导出请求 SHALL 通过 api-client 发送，使用统一的认证和错误处理
