## ADDED Requirements

### Requirement: HTTP Client 配置超时

`relay/proxy.go` 中的 HTTP 请求 SHALL 使用带超时配置的 `*http.Client`，不使用 `http.DefaultClient`。非流式请求超时 120 秒，流式请求连接超时 30 秒。

#### Scenario: 非流式请求超时

- **WHEN** 非流式代理请求发送后上游 120 秒内未响应
- **THEN** 请求 SHALL 超时并返回错误，释放连接资源

#### Scenario: 流式请求连接超时

- **WHEN** 流式代理请求建立连接超过 30 秒
- **THEN** 连接 SHALL 超时并返回错误

#### Scenario: HTTP Client 通过依赖注入

- **WHEN** 构造 proxy 相关组件
- **THEN** `*http.Client` SHALL 通过构造函数注入，不使用包级全局变量

### Requirement: Channel/Group 缓存主动失效

当管理员通过 API 修改 channel 或 group 时，对应的缓存 SHALL 立即失效。

#### Scenario: 创建 channel 后缓存失效

- **WHEN** 管理员创建新 channel
- **THEN** `channels` 缓存 SHALL 被删除，下次 relay 请求 SHALL 从数据库重新加载

#### Scenario: 更新 group 后缓存失效

- **WHEN** 管理员更新 group 配置
- **THEN** `groups` 缓存 SHALL 被删除

#### Scenario: 删除操作后缓存失效

- **WHEN** 管理员删除 channel 或 group
- **THEN** 对应缓存 SHALL 被删除

### Requirement: 熔断器配置缓存化

`getThreshold()` 和 `GetCooldownConfig()` 的数据库查询结果 SHALL 缓存到 MemoryKV 中，TTL 60 秒。热路径上 SHALL 不直接查询数据库。

#### Scenario: 熔断器阈值从缓存读取

- **WHEN** 记录失败需要检查熔断阈值
- **THEN** SHALL 优先从缓存读取，缓存未命中时查询数据库并缓存结果

#### Scenario: 配置更新后 60 秒内生效

- **WHEN** 管理员修改熔断器配置
- **THEN** 新配置 SHALL 在最多 60 秒后生效（缓存 TTL 过期）

### Requirement: Session 和 CircuitBreaker 过期清理

`SessionManager` 和 `CircuitBreakerManager` SHALL 启动后台 goroutine 定期清理过期条目。清理间隔 5 分钟。

#### Scenario: 过期 session 被清理

- **WHEN** session 超过 TTL 且后台清理 goroutine 运行
- **THEN** 该 session SHALL 从 map 中删除

#### Scenario: 已关闭的熔断器被清理

- **WHEN** 熔断器处于 closed 状态超过 30 分钟
- **THEN** 该熔断器条目 SHALL 从 map 中删除

#### Scenario: 优雅关闭时停止清理

- **WHEN** 应用收到关闭信号
- **THEN** 清理 goroutine SHALL 停止运行

### Requirement: copyBody 深拷贝

`relay/adapter.go` 中的 `copyBody` SHALL 执行深拷贝，确保嵌套对象（messages、tools 等）不与原始请求共享引用。

#### Scenario: 修改拷贝不影响原始

- **WHEN** 对 copyBody 返回的副本修改嵌套字段
- **THEN** 原始请求体 SHALL 不受影响
