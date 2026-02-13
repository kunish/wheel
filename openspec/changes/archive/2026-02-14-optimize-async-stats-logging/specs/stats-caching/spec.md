## ADDED Requirements

### Requirement: Stats API 响应缓存

Stats API 端点（global, channel, model, apikey, total, today, daily, hourly）SHALL 优先从内存缓存读取结果。缓存命中时 SHALL 直接返回缓存数据，不执行数据库查询。缓存未命中时 SHALL 执行数据库查询并将结果写入缓存。

#### Scenario: 缓存命中时直接返回

- **WHEN** 客户端请求任意 Stats API 端点且缓存中存在对应 key 的有效数据
- **THEN** 系统 SHALL 直接返回缓存数据，不执行数据库查询，响应格式与无缓存时完全一致

#### Scenario: 缓存未命中时查询并缓存

- **WHEN** 客户端请求任意 Stats API 端点且缓存中不存在对应 key 或已过期
- **THEN** 系统 SHALL 执行数据库查询，将结果写入缓存（TTL 30 秒），并返回查询结果

#### Scenario: 相同参数的请求命中同一缓存

- **WHEN** 两次请求同一 Stats 端点且参数相同（如相同的 tz、start、end）
- **THEN** 第二次请求 SHALL 命中第一次请求写入的缓存（在 TTL 内）

#### Scenario: 不同参数的请求独立缓存

- **WHEN** 两次请求同一 Stats 端点但参数不同（如不同的 tz 值）
- **THEN** 两次请求 SHALL 使用不同的缓存 key，各自独立缓存

### Requirement: 日志 flush 后缓存失效

LogWriter 完成一次 flush（日志批量写入成功）后 SHALL 主动失效所有 stats 相关的缓存条目。

#### Scenario: flush 成功后缓存被清除

- **WHEN** LogWriter 成功将一批日志写入数据库
- **THEN** 所有以 `stats:` 为前缀的缓存条目 SHALL 被删除

#### Scenario: flush 失败不影响缓存

- **WHEN** LogWriter 批量写入数据库失败
- **THEN** stats 缓存 SHALL 保持不变（依赖 TTL 自然过期）

### Requirement: 缓存 TTL 兜底

所有 stats 缓存条目 SHALL 设置 30 秒 TTL 作为兜底机制，防止缓存失效逻辑异常时数据永不更新。

#### Scenario: TTL 过期后自动失效

- **WHEN** stats 缓存条目写入超过 30 秒且未被主动失效
- **THEN** 该条目 SHALL 被视为过期，下次请求 SHALL 重新查询数据库

### Requirement: MemoryKV 支持前缀删除

`cache.MemoryKV` SHALL 提供 `DeletePrefix(prefix string)` 方法，删除所有 key 以指定前缀开头的条目。

#### Scenario: 删除匹配前缀的所有条目

- **WHEN** 调用 `DeletePrefix("stats:")`
- **THEN** 所有 key 以 `stats:` 开头的缓存条目 SHALL 被删除，其他条目不受影响

#### Scenario: 无匹配条目时无副作用

- **WHEN** 调用 `DeletePrefix` 但无任何 key 匹配该前缀
- **THEN** 缓存 SHALL 保持不变，方法正常返回
