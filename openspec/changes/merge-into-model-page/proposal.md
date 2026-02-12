## Why

当前应用中"模型"相关功能分散在三个地方：Channels & Groups 页面管理渠道和分组，Prices 页面管理模型定价，右上角按钮显示模型列表。这种分散的布局让用户在管理模型时需要频繁切换页面，体验割裂。将它们统一到一个 "Model"（模型）页面可以提供一站式的模型管理体验。

## What Changes

- **BREAKING** 将 `/channels` 路由改为 `/model`，页面标题从 "Channels & Groups" 改为 "Model"（模型）
- 将 Prices 页面的所有功能（同步价格列表、添加/修改/删除价格）整合到 Model 页面中
- 在 Channel 卡片和 Group 卡片中内联显示模型价格信息
- 将右上角"模型列表"按钮的功能整合到页面中，替换为更有用的功能入口
- **BREAKING** 移除 `/prices` 路由和独立的 Prices 导航入口
- 底部导航栏中 "Channels & Groups" 改为 "Model"，"Prices" 入口移除，图标更新
- 旧路由 `/channels` 和 `/prices` 设置重定向到 `/model`
- 更新 i18n 翻译（en/zh-CN）

## Capabilities

### New Capabilities

- `unified-model-page`: 将 Channels、Groups、Prices 三个功能区统一到单一 Model 页面，包括路由变更、导航更新、价格整合显示

### Modified Capabilities

（无现有规范需要修改）

## Impact

- **路由**: `/channels` → `/model`（主路由），移除 `/prices`，添加兼容重定向
- **前端页面**: `pages/channels.tsx` 重构为 `pages/model.tsx`，删除 `pages/prices.tsx`
- **导航组件**: `app-layout.tsx` 中导航项从 5 个减少到 4 个
- **i18n**: 更新 `common.json`（导航标签）、重命名/合并 `channels.json` 和 `prices.json` 的翻译
- **路由配置**: `routes.tsx` 和 `protected-layout.tsx` 中的路由转场定义需更新
- **后端 API**: 无变更，所有 API 端点保持不变
