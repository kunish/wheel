## Context

当前 Channel 编辑对话框中，模型列表需要手动输入或通过 Fetch Models 从上游 API 拉取。对于常见 provider（Anthropic、OpenAI、Google），用户经常需要重复添加相同的模型集合。系统已有从 models.dev 获取模型元数据的能力（`modelsync.go`），可以复用这些数据生成内置预设。

后端使用 Go + Gin + bun ORM + TiDB/MySQL，前端使用 React + TanStack Query + shadcn/ui。

## Goals / Non-Goals

**Goals:**
- 提供 Profile 机制，让用户一键加载预定义的模型列表到 Channel
- 从 models.dev 数据生成 Anthropic、OpenAI、Google 三家内置 Profile
- 支持用户自定义 Profile 的 CRUD
- Profile 应用到 Channel 时为"复制"语义（应用后修改 Profile 不影响已配置的 Channel）

**Non-Goals:**
- 不做 Profile 与 Channel 的实时绑定/同步
- 不做 Profile 的导入/导出功能
- 不做 Profile 的权限控制（所有管理员共享）
- 不做 Profile 的版本管理

## Decisions

### 1. Profile 存储方案

**选择**：新建 `model_profiles` 数据库表，字段包括 id、name、provider、models（JSON 数组）、is_builtin、created_at、updated_at。

**替代方案**：存储在 KV 中。但 Profile 需要 CRUD 操作和列表查询，关系型表更合适。

**理由**：与现有的 channels、groups 等实体保持一致的存储模式，便于查询和管理。

### 2. 内置 Profile 生成策略

**选择**：后端在启动时（或手动触发时）从 models.dev 数据中提取 anthropic、openai、google 三家 provider 的模型列表，生成/更新 `is_builtin=true` 的 Profile 记录。

**替代方案**：前端硬编码内置 Profile。但这样无法随 models.dev 数据更新而自动更新。

**理由**：复用现有的 `FetchAndFlattenMetadata()` 逻辑，内置 Profile 可以随 models.dev 数据刷新而更新。

### 3. Profile 应用语义

**选择**：应用 Profile 时，将 Profile 中的模型列表**合并**到 Channel 当前的 model 列表中（去重），而非替换。

**替代方案**：替换当前列表。但用户可能已经手动添加了一些模型，替换会丢失。

**理由**：合并语义更安全，用户可以先清空再应用来实现替换效果。

### 4. 前端 Profile 选择器位置

**选择**：在 Channel 对话框的模型输入区域上方添加一个 Profile 下拉选择器，选择后将模型列表合并到当前输入。

**理由**：紧邻模型输入区域，操作直觉。不需要单独的页面或 Tab。

### 5. Profile 管理入口

**选择**：在模型页面的工具栏中添加 "Profiles" 按钮，打开 Profile 管理对话框，支持查看、创建、编辑、删除自定义 Profile。内置 Profile 只读。

## Risks / Trade-offs

- [models.dev 数据变更] → 内置 Profile 在刷新元数据时自动更新，旧的模型 ID 不会从已配置的 Channel 中移除
- [模型列表过长] → Profile 选择器显示模型数量，应用前可预览列表
- [内置 Profile 覆盖用户修改] → 内置 Profile 标记为 `is_builtin`，用户不可编辑；如需定制可复制为自定义 Profile
