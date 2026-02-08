## ADDED Requirements

### Requirement: Wrangler 配置生成

系统 SHALL 根据用户输入生成完整的 `wrangler.toml` 配置文件，包含 Worker 名称、兼容日期、D1 绑定、KV 绑定和环境变量。

#### Scenario: 生成基础 wrangler.toml

- **WHEN** 用户提供 Worker 名称和 D1 数据库 ID
- **THEN** 系统生成包含 `name`、`compatibility_date`、`[[d1_databases]]` 绑定的 wrangler.toml 内容

#### Scenario: 包含 KV 缓存绑定

- **WHEN** 用户启用 KV 缓存选项
- **THEN** 生成的 wrangler.toml 额外包含 `[[kv_namespaces]]` 绑定配置

### Requirement: Vercel 配置生成

系统 SHALL 根据用户输入生成 `vercel.json` 配置文件，包含构建设置、路由重写和环境变量引用。

#### Scenario: 生成 vercel.json

- **WHEN** 用户选择 Vercel 部署并配置了 API 后端地址
- **THEN** 系统生成包含 `buildCommand`、`outputDirectory`、`rewrites`（将 `/v1/*` 代理到 Worker）的 vercel.json

### Requirement: 环境变量模板生成

系统 SHALL 生成 `.env.example` 模板文件，列出所有必需和可选的环境变量及其说明。

#### Scenario: 生成环境变量模板

- **WHEN** 用户请求生成环境变量模板
- **THEN** 系统生成包含 `ADMIN_USERNAME`、`ADMIN_PASSWORD`、`JWT_SECRET` 等必需变量和 `LOG_LEVEL` 等可选变量的 `.env.example` 文件

### Requirement: D1 Schema 迁移文件生成

系统 SHALL 生成 Cloudflare D1 的 SQL 迁移文件，包含创建 Channel、Group、GroupItem、APIKey、RelayLog、User 等表的 DDL 语句。

#### Scenario: 生成初始迁移

- **WHEN** 用户首次部署
- **THEN** 系统生成 `migrations/0001_initial.sql`，包含所有核心表的 CREATE TABLE 语句，字段定义与 Wheel 原版 GORM 模型兼容

### Requirement: 配置文件下载

系统 SHALL 提供将生成的配置文件打包下载为 ZIP 或单独下载各文件的功能。

#### Scenario: 打包下载所有配置

- **WHEN** 用户点击"下载全部配置"
- **THEN** 系统将 wrangler.toml、.env.example、migrations/ 等文件打包为 ZIP 下载

#### Scenario: 单独复制配置内容

- **WHEN** 用户点击某个配置文件旁的"复制"按钮
- **THEN** 系统将该文件内容复制到剪贴板，显示复制成功提示
