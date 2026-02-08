## ADDED Requirements

### Requirement: 部署目标选择

系统 SHALL 在部署向导的第一步提供部署目标选择界面，用户可选择 Cloudflare Worker 或 Vercel 作为部署目标。

#### Scenario: 选择 Cloudflare Worker 部署

- **WHEN** 用户在部署向导中选择 "Cloudflare Worker"
- **THEN** 系统展示 Cloudflare Worker 部署流程的后续步骤，包括 API Token 配置、D1 数据库设置等

#### Scenario: 选择 Vercel 部署

- **WHEN** 用户在部署向导中选择 "Vercel"
- **THEN** 系统展示 Vercel 部署流程的后续步骤，包括 Vercel Token 配置、环境变量设置等

### Requirement: Cloudflare 凭证配置

系统 SHALL 提供表单让用户输入 Cloudflare 部署所需的凭证信息：Account ID 和 API Token。系统 SHALL 验证凭证有效性。

#### Scenario: 有效凭证验证

- **WHEN** 用户输入 Cloudflare Account ID 和 API Token 并点击"验证"
- **THEN** 系统调用 Cloudflare API 验证凭证，显示验证成功状态和账户信息

#### Scenario: 无效凭证

- **WHEN** 用户输入无效的 API Token
- **THEN** 系统显示错误提示，说明凭证无效，阻止进入下一步

### Requirement: 部署配置预览

系统 SHALL 在执行部署前展示将要生成的配置文件内容（如 `wrangler.toml`），允许用户确认或修改。

#### Scenario: 预览并确认配置

- **WHEN** 用户完成所有配置步骤
- **THEN** 系统展示完整的部署配置预览（wrangler.toml 内容、环境变量列表），用户可点击"确认部署"

#### Scenario: 修改预览配置

- **WHEN** 用户在预览页面修改某项配置值
- **THEN** 配置预览实时更新，反映修改后的值

### Requirement: 部署命令生成

系统 SHALL 生成用户可复制执行的部署命令序列，或提供一键执行功能。

#### Scenario: 生成 Cloudflare 部署命令

- **WHEN** 用户确认 Cloudflare Worker 部署配置
- **THEN** 系统生成包含以下步骤的命令序列：创建 D1 数据库、执行 schema 迁移、部署 Worker，并提供一键复制按钮

#### Scenario: 生成 Vercel 部署链接

- **WHEN** 用户确认 Vercel 部署配置
- **THEN** 系统生成 Vercel Deploy Button URL，包含预设的环境变量和项目配置

### Requirement: 部署状态跟踪

系统 SHALL 在部署执行后跟踪部署状态，展示成功或失败信息。

#### Scenario: 部署成功

- **WHEN** 部署命令执行成功
- **THEN** 系统显示部署成功状态，包括 Worker URL 或 Vercel 域名，以及后续步骤指引

#### Scenario: 部署失败

- **WHEN** 部署过程中发生错误
- **THEN** 系统显示具体错误信息和排查建议

### Requirement: 步骤式进度指示

系统 SHALL 使用步骤条（Stepper）组件展示部署向导的进度，用户可以在已完成的步骤间来回导航。

#### Scenario: 步骤导航

- **WHEN** 用户在第 3 步点击第 1 步的步骤指示器
- **THEN** 系统返回第 1 步，保留之前填写的配置数据
