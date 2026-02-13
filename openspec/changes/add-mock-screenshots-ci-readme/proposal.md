## Why

项目 README 缺少 UI 截图，新用户无法直观了解 Wheel 的界面和功能。同时项目没有 mock 数据，开发和演示时需要手动配置真实 LLM 提供商才能看到完整界面效果。CI 流程中也缺少截图自动化，每次发版后截图需要手动更新。

## What Changes

- 添加 mock 数据种子脚本，可一键填充 dashboard、channels、groups、API keys、logs 等演示数据
- 基于 mock 数据，使用 Playwright 自动截取关键页面截图
- 在 CI release 流程中新增截图自动化 job，发版时自动生成最新截图并提交到仓库
- 更新 README.md，嵌入截图展示 dashboard、通道管理、分组管理、日志等核心页面

## Capabilities

### New Capabilities

- `mock-data`: Mock 数据种子脚本，支持一键填充演示数据到 SQLite 数据库
- `screenshot-automation`: 基于 Playwright 的截图自动化，捕获关键页面截图
- `ci-screenshot-job`: CI release 流程中新增截图生成与提交 job

### Modified Capabilities

- `github-pages-deploy`: release workflow 新增截图自动化 job，在发版时自动生成截图并提交

## Impact

- **CI/CD**: `.github/workflows/release.yml` 新增 screenshot job
- **依赖**: 新增 Playwright 作为 devDependency（仅 CI 和开发使用）
- **仓库**: 新增 `docs/screenshots/` 目录存放截图，新增 mock 数据脚本
- **README.md**: 重构部分内容，嵌入截图展示
- **后端**: 新增 mock 数据种子命令或脚本（Go 或独立脚本）
