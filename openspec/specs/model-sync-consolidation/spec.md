## ADDED Requirements

### Requirement: 模型获取函数统一到 relay/sync 包

`handler/channel.go` 中的 `fetchOpenAIModels`、`fetchAnthropicModels`、`fetchGeminiModels`、`setBrowserHeaders`、`fallbackModelsFromMetadata` SHALL 删除。所有模型获取逻辑 SHALL 统一使用 `relay/sync.go` 中的对应函数。

#### Scenario: handler 调用 sync 包获取模型

- **WHEN** handler 需要获取某个 channel 的模型列表
- **THEN** handler SHALL 调用 `sync.FetchModels(channel)` 而非本地的 fetch 函数

#### Scenario: sync 包函数公开导出

- **WHEN** relay/sync.go 中的模型获取函数被 handler 包调用
- **THEN** 这些函数 SHALL 以大写字母开头导出（如 `FetchOpenAIModels`）

### Requirement: 模型同步逻辑统一

`handler.syncAllModels` 和 `relay.SyncAllModels` SHALL 合并为单一的 `sync.SyncAllModels` 函数。handler 版本的 `isFallback` 处理和 relay 版本的 `autoGroup` 处理 SHALL 通过参数或选项统一。

#### Scenario: 手动触发同步

- **WHEN** 管理员通过 API 手动触发模型同步
- **THEN** 系统 SHALL 调用统一的 `sync.SyncAllModels`，支持 isFallback 选项

#### Scenario: 自动定时同步

- **WHEN** 定时任务触发模型同步
- **THEN** 系统 SHALL 调用同一个 `sync.SyncAllModels`，支持 autoGroup 选项

### Requirement: API 响应检查函数复用

三个 fetch 函数中重复的 HTTP 响应状态检查逻辑 SHALL 提取为 `checkAPIResponse(resp, body, apiName) error` 辅助函数。

#### Scenario: 上游返回非 200

- **WHEN** 上游 API 返回非 200 状态码
- **THEN** checkAPIResponse SHALL 返回包含 API 名称、状态码和截断响应体（最多 200 字符）的错误

### Requirement: pricing 查找逻辑去重

`LookupModelPrice` 和 `CalculateCost` 中重复的模型名称解析逻辑（后缀剥离、前缀剥离）SHALL 提取为 `resolveModelName` 内部函数。`CalculateCost` SHALL 内部调用 `LookupModelPrice`。

#### Scenario: CalculateCost 复用 LookupModelPrice

- **WHEN** 调用 CalculateCost 计算某模型的费用
- **THEN** 模型价格查找 SHALL 通过 LookupModelPrice 完成，不重复实现名称解析
