## ADDED Requirements

### Requirement: 模型同步逻辑下沉到 service 层

`handler/model.go` 中的 `fetchAndFlattenMetadata`（:244-343）和 `SyncPricesFromModelsDev`（:413-492）SHALL 移动到 `internal/service/modelsync.go`。handler 层 SHALL 仅调用 service 函数。

#### Scenario: handler 调用 service 同步价格

- **WHEN** 管理员触发价格同步 API
- **THEN** handler SHALL 调用 `service.SyncPricesFromModelsDev()`，不包含 HTTP 请求或数据转换逻辑

#### Scenario: handler 调用 service 获取元数据

- **WHEN** 需要获取模型元数据
- **THEN** handler SHALL 调用 `service.FetchAndFlattenMetadata()`，不包含 HTTP 请求逻辑

### Requirement: 数据导入逻辑下沉到 service 层

`handler/setting.go:154-288` 中的 `ImportData` 业务逻辑（去重检查、多实体导入）SHALL 移动到 `internal/service/import.go`。

#### Scenario: handler 调用 service 导入数据

- **WHEN** 管理员触发数据导入 API
- **THEN** handler SHALL 解析请求体后调用 `service.ImportData(dump)`，返回导入结果

#### Scenario: service 返回导入统计

- **WHEN** service.ImportData 执行完成
- **THEN** SHALL 返回每种资源的 Added/Skipped 计数

### Requirement: handler 层保持薄包装

重构后的 handler 函数 SHALL 仅包含：参数解析（`ShouldBindJSON`）、调用 service、格式化响应（`successJSON`/`c.JSON`）。每个 handler 函数 SHALL 不超过 30 行。

#### Scenario: model handler 函数精简

- **WHEN** 查看重构后的 model handler
- **THEN** 每个 handler 函数 SHALL 不超过 30 行，不包含 HTTP 请求、数据转换或复杂业务逻辑
