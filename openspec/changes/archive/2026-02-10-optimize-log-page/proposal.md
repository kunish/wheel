## Why

日志详情面板当前存在几个体验短板：Model 和 Channel 只是静态文本展示，无法快速跳转到 Channels & Groups 页面定位对应配置；详情中仅显示美化后的模型名（如 "Claude Sonnet 4.5"），缺少实际 API 模型标识符（如 `claude-sonnet-4-5-20250929`），不便于调试和排查；Request 和 Response 分为两个独立 Tab，频繁切换增加了认知负担。

## What Changes

- 日志表格和详情面板中的 Model 名称和 Channel 名称增加可点击交互，点击后跳转到 `/channels` 页面并自动定位、高亮对应的 Channel 或 Group
- 日志详情 Overview 中的模型流展示（`requestModelName → channel → actualModelName`）增加实际模型 ID 的显示，美化名与原始 ID 同时可见
- 将 Request Tab 和 Response Tab 合并为单个 "Request & Response" Tab，上下排列展示，减少切换操作
- 日志详情中 Request 和 Response 区域支持展示原始请求体内容（raw body），并新增展示实际发送给上游 provider 的转换后请求体（经模型名替换、参数注入等处理）

## Capabilities

### New Capabilities

- `log-channel-navigation`: 从日志页面点击 Model/Channel 快速跳转到 Channels & Groups 页面并定位高亮
- `log-actual-model-display`: 日志详情中同时展示美化模型名和实际模型标识符
- `log-request-response-merge`: 合并 Request 和 Response 为单个 Tab 展示，支持展示原始请求体和实际上游请求体

### Modified Capabilities

_(无需修改现有 spec 的需求定义)_

## Impact

- **前端代码**: `apps/web/src/app/(protected)/logs/page.tsx` — DetailPanel 组件、CodeBlock 组件、Tab 结构
- **前端代码**: `apps/web/src/app/(protected)/channels/page.tsx` — 增加滚动定位和高亮支持（通过 URL hash 或 query param）
- **组件**: `ModelBadge` 组件可能需要增加点击交互支持
- **路由**: `/channels` 页面需要支持 `?highlight=channel:<id>` 或类似的定位参数
- **后端代码**: `apps/worker/src/relay/handler.ts` — 在日志写入时增加存储实际上游请求体（`upstreamContent` 字段）
- **数据库**: `relay_logs` 表新增 `upstream_content` TEXT 字段
- **API**: `GET /api/log/:id` 返回数据自动包含新字段，无需额外改动
