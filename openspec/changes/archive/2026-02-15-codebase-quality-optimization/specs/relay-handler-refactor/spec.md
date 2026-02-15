## ADDED Requirements

### Requirement: handleRelay 拆分为方法链

`RelayHandler.handleRelay` SHALL 拆分为以下私有方法，每个方法职责单一：

- `parseRelayRequest(c *gin.Context)` — 解析请求体、提取模型名、判断流式/非流式
- `selectChannels(modelName string)` — 加载 channels/groups、匹配模型、应用 session 粘性
- `executeWithRetry(ctx, channels, request)` — 重试循环，包含熔断器检查和 attempt 记录
- `recordResult(ctx, result, attempts)` — 统一的成功/失败日志和指标记录

原始 `handleRelay` SHALL 变为编排方法，仅调用上述方法并传递中间结果。

#### Scenario: 方法链正常执行

- **WHEN** 收到一个合法的 relay 请求
- **THEN** 系统 SHALL 依次调用 parseRelayRequest → selectChannels → executeWithRetry → recordResult，每个方法不超过 80 行

#### Scenario: parseRelayRequest 失败

- **WHEN** 请求体解析失败或模型名为空
- **THEN** parseRelayRequest SHALL 返回错误，handleRelay 直接返回 400 响应，不进入后续步骤

### Requirement: 流式/非流式统一执行路径

`executeWithRetry` 内部 SHALL 通过 `RelayStrategy` 接口统一流式和非流式的执行差异。接口定义：

- `Execute(ctx, channel, request) (result, error)` — 执行单次代理请求
- `HandleSuccess(ctx, result)` — 处理成功响应

流式和非流式各自实现该接口，消除当前 handleRelay 中两条路径的重复代码。

#### Scenario: 非流式请求使用 NonStreamStrategy

- **WHEN** 请求不包含 `stream: true`
- **THEN** executeWithRetry SHALL 使用 NonStreamStrategy 执行代理

#### Scenario: 流式请求使用 StreamStrategy

- **WHEN** 请求包含 `stream: true`
- **THEN** executeWithRetry SHALL 使用 StreamStrategy 执行代理

### Requirement: asyncLog 合并为统一方法

`asyncStreamLog` 和 `asyncNonStreamLog` SHALL 合并为单个 `asyncRecordLog` 方法。公共逻辑（cost 计算、attempts 序列化、LogWriter.Submit、Observer 指标记录）SHALL 只存在一份。流式/非流式的差异通过参数区分。

#### Scenario: 流式请求日志记录

- **WHEN** 流式代理请求完成
- **THEN** asyncRecordLog SHALL 记录包含 stream token 用量的日志条目

#### Scenario: 非流式请求日志记录

- **WHEN** 非流式代理请求完成
- **THEN** asyncRecordLog SHALL 记录包含响应体 token 用量的日志条目

### Requirement: 全局状态封装为结构体

以下全局变量 SHALL 封装为结构体：

- `relay.breakers` + `circuitObserver` → `CircuitBreakerManager` 结构体
- `relay.sessions` → `SessionManager` 结构体
- `relay.rrCounters` → `BalancerState` 结构体

每个结构体 SHALL 提供 `New*()` 构造函数。`RelayHandler` SHALL 通过构造函数接收这些依赖。

#### Scenario: CircuitBreakerManager 隔离测试

- **WHEN** 创建两个独立的 `CircuitBreakerManager` 实例
- **THEN** 一个实例中记录的失败 SHALL 不影响另一个实例的熔断状态

#### Scenario: main.go 初始化注入

- **WHEN** 应用启动
- **THEN** main.go SHALL 构造 CircuitBreakerManager、SessionManager、BalancerState 并注入到 RelayHandler

### Requirement: API Key 认证逻辑去重

`RelayHandler.apiKeyAuthMiddleware()` 中的 key 提取逻辑 SHALL 复用 `middleware.ApiKeyAuth` 中的实现。RelayHandler SHALL 不再重复实现 key 提取（从 `x-api-key` header 或 `Authorization: Bearer sk-wheel-*`）。

#### Scenario: relay 请求使用统一的 key 提取

- **WHEN** relay 请求携带 API Key
- **THEN** key 提取逻辑 SHALL 与管理 API 的 key 提取逻辑完全一致，来自同一份代码
