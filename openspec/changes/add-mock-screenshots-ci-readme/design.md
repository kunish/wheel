## Context

Wheel 是一个 LLM API Gateway，提供 React Dashboard + Go Worker 的 monorepo 架构。当前项目 README 没有任何 UI 截图，新用户无法直观了解产品界面。项目也没有 mock 数据，开发和演示需要配置真实 LLM 提供商。CI 流程（release.yml）已有 Docker 构建和 GitHub Pages 部署，但缺少截图自动化。

技术栈：React 19 + Vite + shadcn/ui（前端），Go + Gin + SQLite（后端），GitHub Actions（CI），Playwright（新增）。

## Goals / Non-Goals

**Goals:**

- 提供一键 mock 数据填充，让开发者和演示场景无需真实 LLM 提供商即可看到完整 UI
- 自动化截图流程，发版时自动生成最新 UI 截图
- README 嵌入截图，提升项目第一印象

**Non-Goals:**

- 不做端到端测试（截图仅用于文档展示，不做视觉回归测试）
- 不修改现有业务逻辑或 API
- 不支持动态 mock server（仅静态种子数据）

## Decisions

### 1. Mock 数据方案：Go seed 命令

**选择**: 在 worker 中添加 `seed` 子命令，直接写入 SQLite。

**理由**: 项目后端已有完整的 DB 层（Bun ORM + SQLite），复用现有 model 和 migration 最自然。相比独立脚本（Python/Node），不引入额外运行时依赖。

**替代方案**:

- SQL 文件直接导入 → 与 ORM model 脱节，migration 变更时容易过期
- Node 脚本通过 API 写入 → 需要先启动 worker，增加复杂度

### 2. 截图工具：Playwright

**选择**: 使用 Playwright 截图，在 CI 中运行。

**理由**: Playwright 是 headless browser 截图的事实标准，支持多浏览器、多分辨率，pnpm 生态原生支持。项目前端已有 Vitest，Playwright 作为 devDependency 不影响生产构建。

**替代方案**:

- Puppeteer → 功能类似但 Playwright API 更现代，跨浏览器支持更好
- 手动截图 → 不可维护，每次 UI 变更都需要人工更新

### 3. 截图脚本位置：apps/web/scripts/

**选择**: 截图脚本放在 `apps/web/scripts/screenshots.ts`，使用 Playwright 启动本地 dev server 后截图。

**理由**: 截图是前端关注点，放在 web app 目录下最合理。脚本先启动 worker（填充 mock 数据）+ web dev server，然后逐页截图。

### 4. CI Job 设计：独立 screenshot job

**选择**: 在 release.yml 中新增 `screenshots` job，依赖 release-please，在发版时运行。

**理由**: 截图生成需要构建前后端并运行，耗时较长，独立 job 不阻塞 Docker 构建和 Pages 部署。截图完成后通过 GitHub Actions bot 提交到仓库。

### 5. 截图存储：docs/screenshots/

**选择**: 截图存放在 `docs/screenshots/` 目录，PNG 格式。

**理由**: 与代码分离，README 通过相对路径引用。PNG 在 GitHub 上直接渲染，无需额外托管。

### 6. 截图页面选择

捕获以下关键页面（亮色 + 暗色主题各一套）：

- Dashboard（仪表盘总览）
- Channels（通道管理）
- Groups（分组管理）
- Models（模型管理）
- Logs（请求日志）
- API Keys（密钥管理）

## Risks / Trade-offs

- **[截图体积]** → PNG 截图会增加仓库体积。Mitigation: 使用合理分辨率（1280x800），压缩 PNG，控制截图数量在 12 张以内（6 页面 × 2 主题）。
- **[Mock 数据过期]** → DB schema 变更可能导致 seed 命令失败。Mitigation: seed 命令复用 ORM model，migration 变更时编译期即可发现问题。
- **[CI 截图失败]** → 截图 job 失败不应阻塞发版。Mitigation: screenshot job 设置 `continue-on-error: true`，失败时仅跳过截图更新。
- **[Git 提交冲突]** → CI bot 提交截图可能与其他 PR 冲突。Mitigation: 截图提交到独立 branch 并自动创建 PR，或直接提交到 main（截图文件路径固定，冲突概率低）。
