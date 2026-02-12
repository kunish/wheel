## Context

日志页面（`/logs`）是 Wheel 的核心调试工具，用户通过它查看 API 请求的路由结果、性能指标和错误信息。当前实现中：

- **DetailPanel** 使用 Tab 结构分为 Overview / Request / Response / Retry Timeline 四个 Tab
- **Model 和 Channel** 在表格和详情中仅作为静态文本展示，无导航能力
- **模型名显示** 通过 `fuzzyLookup()` 只展示美化名（如 "Claude Sonnet 4.5"），实际 API 标识符（如 `claude-sonnet-4-5-20250929`）在 Overview 的模型流中虽有展示但不够明显
- **Channels 页面** 无外部定位支持，不接受 URL 参数来聚焦特定 channel

技术栈：Next.js App Router + shadcn/ui + Tailwind CSS + Framer Motion。

## Goals / Non-Goals

**Goals:**

- 让 Model/Channel 成为可交互的导航入口，点击后跳转到 Channels & Groups 并定位高亮
- 日志详情中明确展示实际模型 ID，方便调试排查
- 合并 Request/Response 到单个 Tab，减少频繁切换

**Non-Goals:**

- 不修改 Channels & Groups 页面的编辑逻辑，仅增加定位/高亮能力
- 不改变模型的 fuzzyLookup 逻辑本身
- 不存储请求/响应的 HTTP headers（仅存储 body）

## Decisions

### 1. Channel 页面定位机制：URL query param

**方案**: 使用 `/channels?highlight=<channelId>` query param 定位到指定 channel。

**替代方案**:

- URL hash (`#channel-123`): 需要给每个 channel 元素设置 id，且 hash 变更不触发 Next.js 路由更新
- 全局状态: 页面刷新后丢失，不支持链接分享

**理由**: Query param 与项目现有的 URL 状态管理模式一致（日志页已使用 query param 管理过滤器），且支持链接分享和刷新保持。Channel 页面读取 `highlight` 参数后：

1. 找到对应 channel 的 DOM 元素
2. 使用 `scrollIntoView({ behavior: 'smooth', block: 'center' })` 滚动到可视区域
3. 添加临时高亮动画（ring + pulse），3 秒后淡出

### 2. 实际模型名展示位置：Overview 模型流区域

**方案**: 在 DetailPanel Overview 中的模型流可视化区域，每个模型节点下方小字显示原始 model ID（当与美化名不同时）。

**替代方案**:

- Tooltip 悬浮显示: 不够直观，移动端不友好
- 单独字段: 占用额外空间

**理由**: 模型流（`requestModelName → channel → actualModelName`）已经展示了模型信息，在每个节点下方用 `text-xs text-muted-foreground font-mono` 显示原始 ID 是最自然的位置。

### 3. Request/Response 合并方式：垂直堆叠 + 可折叠

**方案**: 合并为 "Messages" Tab，Request 和 Response 上下垂直排列，各自带标题栏和独立的搜索/复制功能。两个区域默认展开，可独立折叠。

**替代方案**:

- 左右并排: 屏幕宽度不够（Sheet 已是 max-w-2xl）
- 保持分 Tab 但增加快捷键切换: 没有根本解决来回切换的问题

**理由**: 垂直堆叠利用了侧面板的纵向空间，用户可以同时看到请求和响应的上下文关系。可折叠设计允许用户在只关注某一部分时收起另一部分。

### 4. 上游请求体存储方案：新增 `upstream_content` 字段

**方案**: 在 `relay_logs` 表新增 `upstream_content` TEXT 字段，在 `handler.ts` 中于 `buildUpstreamRequest()` 返回后、实际发送前，将转换后的请求 body 序列化并截断存储。

**替代方案**:

- 前端实时重建：根据 channel 配置和原始请求在前端模拟转换 → 逻辑复杂且无法保证与实际一致
- 在 adapter 层存储：修改 `buildUpstreamRequest()` 返回值 → 侵入性更大

**理由**: 在 handler 中存储是最小改动方案。`buildUpstreamRequest()` 已经返回完整的 upstream request 对象，只需在发送前将 body 部分用与 `requestContent` 相同的 `truncateForLog()` 处理后存储。复用现有截断逻辑保持一致性。

### 5. Messages Tab 中三个内容区域的展示

**方案**: Messages Tab 垂直排列三个可折叠区域：

1. **Request (Original)** — 用户发来的原始请求体（`requestContent`）
2. **Request (Upstream)** — 实际发送给上游 provider 的请求体（`upstreamContent`），仅当与原始请求不同时显示
3. **Response** — 上游返回的响应（`responseContent`）

**理由**: 将 Original 和 Upstream 放在一起便于对比差异（模型名替换、参数注入等），默认折叠 Upstream 区域避免信息过载。

## Risks / Trade-offs

- **[Channel 高亮时机]** Channel 页面数据异步加载，高亮逻辑需等 DOM 渲染完成 → 使用 `useEffect` + `requestAnimationFrame` 确保 DOM 就绪后再滚动
- **[合并 Tab 后内容过长]** Request + Response 同时展示时面板内容变长 → 可折叠设计 + 各区域独立滚动解决
- **[Group 内 Channel 定位]** 如果 channel 在某个折叠的 group 内，需要先展开 group → 高亮逻辑需包含自动展开父级折叠区域
- **[upstream_content 存储开销]** 新增一个 TEXT 字段会增加数据库存储 → 使用与 requestContent 相同的 truncateForLog() 截断策略（10KB 上限），控制存储量
- **[DB 迁移]** 新增字段需要 Drizzle migration → 使用 `ALTER TABLE ADD COLUMN` 默认 NULL，无需回填历史数据
