# Spec: 日志保留清理

## Summary

添加日志自动保留清理机制，防止日志无限增长。

## Changes

### Backend

#### `apps/worker/src/db/dal/logs.ts`

- 添加 `cleanupOldLogs(db: Database, retentionDays: number)` 函数
  - 删除 `time < now - retentionDays * 86400` 的日志
  - 返回删除的行数

#### `apps/worker/src/relay/handler.ts`

- 在异步日志写入回调末尾，以 1/100 的概率调用 `cleanupOldLogs`
- 从 settings 读取 `log_retention_days`（默认 30 天）

#### `apps/worker/src/db/dal/settings.ts`

- 如果尚不存在，添加 `getSetting(db, key)` 通用函数

#### `apps/worker/src/routes/setting.ts`

- 确保 `log_retention_days` 可通过设置 API 读写
