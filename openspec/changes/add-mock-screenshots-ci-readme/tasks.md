## 1. Mock 数据种子命令

- [x] 1.1 在 `apps/worker/cmd/worker/main.go` 中添加 `seed` 子命令入口
- [x] 1.2 创建 `apps/worker/internal/seed/seed.go`，实现种子数据生成逻辑
- [x] 1.3 添加 demo channels 数据（OpenAI、Anthropic、Google Gemini，各含 base URL 和 key）
- [x] 1.4 添加 demo groups 数据（至少 2 个分组，含通道-模型配对、优先级/权重配置）
- [x] 1.5 添加 demo API keys 数据（至少 3 个，含不同配额和过期状态）
- [x] 1.6 添加 demo pricing 数据（常用模型的定价条目）
- [x] 1.7 添加 demo request logs 数据（过去 30 天，含成功/失败/重试等状态，token 用量和成本）
- [x] 1.8 实现幂等逻辑，重复运行 seed 不会产生重复数据

## 2. Playwright 截图脚本

- [x] 2.1 在 `apps/web` 中添加 Playwright 为 devDependency
- [x] 2.2 创建 `apps/web/scripts/screenshots.ts` 截图脚本
- [x] 2.3 实现服务启动逻辑：启动 worker（含 seed 数据）+ web dev server，等待就绪
- [x] 2.4 实现截图逻辑：登录后依次截取 Dashboard、Channels、Groups、Models、Logs、API Keys 页面
- [x] 2.5 实现亮色/暗色主题切换截图（每页两张，共 12 张）
- [x] 2.6 设置统一 viewport 1280x800，等待页面完全渲染后截图
- [x] 2.7 实现进程清理逻辑，截图完成或失败后关闭 worker 和 dev server
- [x] 2.8 在 `apps/web/package.json` 中添加 `screenshots` script

## 3. CI 截图 Job

- [x] 3.1 在 `.github/workflows/release.yml` 中新增 `screenshots` job
- [x] 3.2 配置 job 依赖 `release-please`，仅在 release 创建时触发
- [x] 3.3 安装 Go、Node.js、pnpm、Playwright Chromium 等依赖
- [x] 3.4 运行截图脚本生成截图到 `docs/screenshots/`
- [x] 3.5 使用 GitHub Actions bot 提交截图变更到 main 分支（conventional commit 格式）
- [x] 3.6 设置 `continue-on-error: true`，截图失败不阻塞发版

## 4. README 更新

- [x] 4.1 创建 `docs/screenshots/` 目录并添加 `.gitkeep`
- [x] 4.2 在 README.md 的 Features 部分后添加 Screenshots 章节
- [x] 4.3 嵌入关键页面截图（Dashboard、Channels、Groups、Logs），使用相对路径引用
- [x] 4.4 为截图添加亮色/暗色主题切换展示（使用 GitHub 的 `<picture>` + `prefers-color-scheme`）
