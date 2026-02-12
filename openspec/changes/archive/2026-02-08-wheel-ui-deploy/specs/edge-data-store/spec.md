## ADDED Requirements

### Requirement: D1 数据库 Schema

系统 SHALL 使用 Cloudflare D1 作为数据存储，通过 Drizzle ORM 定义 Schema。Schema MUST 包含以下核心表：users、channels、channel_keys、groups、group_items、api_keys、relay_logs、settings。

#### Scenario: Schema 与 Wheel 兼容

- **WHEN** 系统初始化 D1 数据库
- **THEN** 创建的表结构在语义上与 Wheel 的 GORM 模型兼容，支持相同的数据关系（Channel 1:N ChannelKey, Group 1:N GroupItem 等）

### Requirement: Channel CRUD

系统 SHALL 提供 Channel 的完整 CRUD 操作，通过管理 API 暴露。每个 Channel 包含：名称、类型（OpenAI/Anthropic/Gemini 等）、BaseURL 列表、API Key 列表、支持的模型列表。

#### Scenario: 创建 Channel

- **WHEN** 管理员通过 `POST /api/v1/channel/create` 提交 Channel 数据
- **THEN** 系统在 D1 中创建 Channel 记录及关联的 ChannelKey 记录，返回创建结果

#### Scenario: 更新 Channel 的 API Key

- **WHEN** 管理员通过 `POST /api/v1/channel/update` 修改 Channel 的 Keys 列表
- **THEN** 系统同步更新 channel_keys 表，新增/删除/修改对应记录

### Requirement: Group CRUD

系统 SHALL 提供 Group 的完整 CRUD 操作。每个 Group 包含：名称、负载均衡模式、模型匹配正则、首 token 超时、Channel 分配列表（GroupItem）。

#### Scenario: 创建 Group 并分配 Channel

- **WHEN** 管理员创建 Group 并添加 2 个 Channel（各自配置 priority 和 weight）
- **THEN** 系统在 D1 中创建 Group 记录和 2 条 GroupItem 记录

#### Scenario: 按模型名匹配 Group

- **WHEN** relay 请求模型为 "gpt-4o"
- **THEN** 系统遍历所有 Group，使用 MatchRegex 匹配请求模型名，返回匹配的 Group

### Requirement: API Key 管理

系统 SHALL 提供 API Key 的 CRUD 操作。API Key 格式为 `sk-wheel-` 前缀加 48 位随机字符。支持过期时间、成本限额、模型白名单配置。

#### Scenario: 创建 API Key

- **WHEN** 管理员通过 `POST /api/v1/apikey/create` 提交 API Key 配置
- **THEN** 系统生成 `sk-wheel-[48 chars]` 格式的 Key，存入 D1，返回完整 Key 值（仅此一次可见）

#### Scenario: API Key 成本累计

- **WHEN** 使用某 API Key 的 relay 请求完成
- **THEN** 系统异步更新该 API Key 的累计成本字段

### Requirement: 热数据缓存

系统 SHALL 使用 Cloudflare KV 缓存频繁读取但低频更新的数据（Channel 配置、Group 配置），减少 D1 查询次数。

#### Scenario: 缓存命中

- **WHEN** relay 请求需要查询 Channel 配置，KV 中存在有效缓存
- **THEN** 系统直接从 KV 读取配置，不查询 D1

#### Scenario: 缓存失效

- **WHEN** 管理员更新了 Channel 配置
- **THEN** 系统在 D1 写入成功后，删除对应的 KV 缓存条目，下次请求时重建缓存

#### Scenario: 缓存 TTL

- **WHEN** KV 缓存条目超过 5 分钟未被刷新
- **THEN** 系统视为过期，重新从 D1 加载数据并更新 KV

### Requirement: 管理员认证

系统 SHALL 支持 JWT 认证用于管理 API。使用 `ADMIN_USERNAME` 和 `ADMIN_PASSWORD` 环境变量配置管理员凭证，JWT secret 从环境变量 `JWT_SECRET` 获取。

#### Scenario: 管理员登录

- **WHEN** 用户通过 `POST /api/v1/user/login` 提交正确的用户名和密码
- **THEN** 系统返回 JWT token 和过期时间

#### Scenario: JWT 验证

- **WHEN** 请求管理 API 时携带有效的未过期 JWT token
- **THEN** 系统允许请求通过

#### Scenario: JWT 过期

- **WHEN** 请求携带已过期的 JWT token
- **THEN** 系统返回 HTTP 401，前端跳转到登录页
