## MODIFIED Requirements

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

### Requirement: Channel/Group CRUD 后缓存主动失效

当管理员通过 API 创建、更新或删除 channel/group 时，relay 层的 channels/groups 缓存 SHALL 立即失效。Stats 缓存的 flush 失效机制保持不变。

#### Scenario: 创建 channel 后 relay 缓存失效

- **WHEN** 管理员创建新 channel
- **THEN** relay 层的 `channels` 缓存 SHALL 被删除，下次 relay 请求从数据库重新加载

#### Scenario: 更新 group 后 relay 缓存失效

- **WHEN** 管理员更新 group
- **THEN** relay 层的 `groups` 缓存 SHALL 被删除
