## Why

当前模型页面中，每个 Channel 的模型列表需要手动逐个添加，对于常见的 provider（如 Anthropic、OpenAI、Google）缺乏快速配置的方式。用户经常需要为同一个 provider 重复添加相同的模型集合。通过引入 Profile（预设配置）机制，用户可以一键加载某个 provider 的推荐模型列表，大幅减少重复操作。同时，利用已有的 models.dev 数据源，可以提供官方收集的内置预设。

## What Changes

- 新增 Profile 概念：一组预定义的模型 ID 列表，可快速应用到 Channel 的模型配置中
- 提供内置 Profile：从 models.dev 数据中提取 Anthropic、OpenAI、Google（Gemini）三家 provider 的模型列表作为内置预设
- 在 Channel 编辑对话框中添加 Profile 选择器，支持一键加载预设模型列表
- 支持用户自定义 Profile 的创建、编辑、删除
- Profile 数据持久化到后端数据库

## Capabilities

### New Capabilities
- `model-profiles`: 模型预设（Profile）的 CRUD、内置预设生成、以及在 Channel 编辑中的快速应用

### Modified Capabilities
- `unified-model-page`: Channel 编辑对话框中新增 Profile 选择器入口

## Impact

- 后端：新增 `model_profiles` 数据库表、CRUD API 端点、从 models.dev 生成内置预设的逻辑
- 前端：Channel 对话框新增 Profile 选择器组件、新增 Profile 管理 UI
- API：新增 `/api/v1/model/profiles` 系列端点
- 依赖：复用现有 models.dev 数据获取逻辑（`modelsync.go`）
