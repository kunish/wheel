## ADDED Requirements

### Requirement: 统一使用 any 替代 interface{}

所有 Go 源文件 SHALL 使用 `any` 替代 `interface{}`。

#### Scenario: 现有 interface{} 替换

- **WHEN** 代码中存在 `interface{}` 类型声明
- **THEN** SHALL 替换为 `any`（包括 `user.go:24` 的 `successJSON` 和 `types/api.go:287` 的 `Data` 字段）

### Requirement: 请求结构体定义位置统一

所有 HTTP 请求/响应结构体 SHALL 统一定义在 `types/api.go` 中并导出。handler 中 SHALL 不使用匿名结构体或未导出结构体定义请求体。

#### Scenario: model handler 使用 types 包结构体

- **WHEN** model handler 需要解析请求体
- **THEN** SHALL 使用 `types.LLMCreateRequest`、`types.LLMUpdateRequest` 等已定义的结构体，不使用 `var body struct{...}`

#### Scenario: user handler 请求结构体迁移

- **WHEN** user handler 需要解析登录请求
- **THEN** SHALL 使用 `types.LoginRequest`（从 `types/api.go` 导出），不使用 handler 内的 `loginRequest`

### Requirement: Handler 结构体组合

`Handler` 和 `RelayHandler` 的公共字段（DB、LogDB、Cache）SHALL 通过嵌入组合复用。

#### Scenario: RelayHandler 嵌入 Handler

- **WHEN** 定义 RelayHandler
- **THEN** RelayHandler SHALL 嵌入 Handler，继承 DB、LogDB、Cache 字段，不重复声明

### Requirement: Update handler 统一为 struct + 指针模式

`UpdateChannel` 和 `UpdateGroup` 中的 `map[string]interface{}` 手动字段映射 SHALL 重构为 struct + 指针字段模式（与 `UpdateApiKey` 一致）。

#### Scenario: UpdateChannel 使用类型安全的更新

- **WHEN** 更新 channel 时
- **THEN** handler SHALL 使用 `ChannelUpdateRequest` struct 解析请求体，通过 nil 检查判断哪些字段需要更新

### Requirement: 清理未使用的类型定义

`types/api.go` 中未被任何代码引用的 Response 类型（如 `ChannelListResponse`、`GroupListResponse` 等）SHALL 删除或在 handler 中实际使用。

#### Scenario: Response 类型被 handler 使用

- **WHEN** handler 返回列表数据
- **THEN** SHALL 使用 `types.XxxListResponse` 结构体包装返回值，或删除未使用的类型定义

### Requirement: BroadcastFunc 和 StreamTracker 定义去重

`handler/relay.go` 和 `db/logwriter.go` 中重复定义的 `BroadcastFunc` 类型和 `StreamTracker` 接口 SHALL 统一到 `types` 包中。

#### Scenario: 两个包引用同一定义

- **WHEN** handler 和 logwriter 需要使用 BroadcastFunc
- **THEN** 两者 SHALL 从 `types` 包导入同一个定义
