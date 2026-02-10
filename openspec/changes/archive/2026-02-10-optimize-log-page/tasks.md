## 1. Channels 页面定位高亮支持

- [x] 1.1 在 `/channels` 页面读取 `highlight` query param，实现滚动定位到目标 channel 卡片（`scrollIntoView({ behavior: 'smooth', block: 'center' })`）
- [x] 1.2 为目标 channel 卡片添加临时高亮动画（ring 边框 + pulse），3 秒后淡出
- [x] 1.3 处理目标 channel 处于折叠状态的情况：自动展开后再滚动
- [x] 1.4 高亮动画完成后使用 `router.replace` 静默移除 URL 中的 `highlight` 参数
- [x] 1.5 处理 `highlight` 指向不存在 channelId 的情况（静默忽略）

## 2. 日志表格 Model/Channel 点击导航

- [x] 2.1 将日志表格 Model 列的 `ModelBadge` 包装为可点击链接，点击导航到 `/channels?highlight=<channelId>`
- [x] 2.2 将日志表格 Channel 列文本包装为可点击链接，点击导航到 `/channels?highlight=<channelId>`
- [x] 2.3 处理 channelId 为空或 channel 已删除的情况：渲染为普通文本不可点击

## 3. 详情面板 Model/Channel 点击导航

- [x] 3.1 将 DetailPanel Overview 模型流中的 requestModelName、actualModelName 包装为可点击链接
- [x] 3.2 将模型流中的 channel 名称包装为可点击链接
- [x] 3.3 链接行为与表格列一致，导航到 `/channels?highlight=<channelId>`

## 4. 实际模型名展示

- [x] 4.1 在 DetailPanel Overview 模型流中，当美化名与原始 modelId 不同时，在美化名下方用 `text-xs text-muted-foreground font-mono` 显示原始 modelId
- [x] 4.2 实际 modelId 文本支持点击复制到剪贴板，显示 "Copied" 确认
- [x] 4.3 日志表格中的 ModelBadge 增加 Tooltip，hover 时显示原始 modelId

## 5. 合并 Request 和 Response Tab

- [x] 5.1 将 DetailPanel 的 "Request" 和 "Response" 两个 Tab 合并为 "Messages" Tab
- [x] 5.2 Messages Tab 内垂直排列 Request (Original)、Request (Upstream)、Response 三个区域，各自带标题栏
- [x] 5.3 每个区域保留独立的搜索和复制功能（复用现有 CodeBlock 组件）
- [x] 5.4 三个区域支持独立折叠/展开，Request (Original) 和 Response 默认展开，Request (Upstream) 默认折叠
- [x] 5.5 各区域之间添加视觉分隔（分隔线 + 标题栏区分样式）

## 6. 后端存储上游请求体

- [x] 6.1 在 `relay_logs` 表新增 `upstream_content` TEXT 字段（Drizzle schema + migration）
- [x] 6.2 在 `handler.ts` 中，`buildUpstreamRequest()` 返回后将转换后的请求 body 用 `truncateForLog()` 截断并存储到 `upstream_content`
- [x] 6.3 当请求无任何转换时（模型名、参数均不变），`upstream_content` 设为 NULL 避免冗余存储
- [x] 6.4 验证 `GET /api/log/:id` 返回数据自动包含 `upstreamContent` 字段

## 7. 前端展示上游请求体

- [x] 7.1 Messages Tab 中当 `upstreamContent` 存在且不为空时，在 Request (Original) 和 Response 之间显示 "Request (Upstream)" 区域
- [x] 7.2 当 `upstreamContent` 为 NULL 或空时（历史数据或无转换），不显示 Upstream 区域
- [x] 7.3 Upstream 区域使用 CodeBlock 组件，拥有独立的搜索和复制功能
