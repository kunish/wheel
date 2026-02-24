## Context

当前应用有 5 个底部导航项：Dashboard、Channels & Groups、Prices、Logs、Settings。其中 Channels & Groups 页面是一个两栏布局（左侧 Channels，右侧 Groups），使用 `@dnd-kit` 实现拖拽；Prices 页面是独立的表格式价格管理页。右上角有一个 "Models" 按钮弹出可用模型列表。

三个功能共享同一套后端模型数据（channels 包含 model 数组、groups 引用 channelId+modelName、prices 按 model name 存储价格），但前端分散在不同页面。

技术栈：React + Vite + react-router v7 + TanStack Query + Tailwind CSS + shadcn/ui + motion/react + i18next。

## Goals / Non-Goals

**Goals:**

- 将 Channels、Groups、Prices 合并到统一的 `/model` 页面
- 底部导航从 5 项变为 4 项（移除 Prices 入口，Channels & Groups 变为 Model）
- 在 Channel 和 Group 卡片中内联显示模型价格
- 价格管理功能（同步/添加/编辑/删除）整合到 Model 页面
- 旧路由兼容重定向

**Non-Goals:**

- 不修改后端 API（所有 `/api/v1/channel/*`、`/api/v1/group/*`、`/api/v1/model/price/*` 保持不变）
- 不重构 Channel/Group 的拖拽逻辑
- 不改变数据模型或数据库结构
- 不新增后端功能

## Decisions

### 1. 页面布局：三栏式 → 双栏 + Tab 方式

**选择**: 保持现有的双栏布局（Channels 左 | Groups 右），将 Prices 功能以新的方式整合：

- 在页面顶部工具栏添加 "Sync Prices" 和 "Add Price" 按钮（从 Prices 页面迁移）
- 在每个 ModelCard 组件中内联显示该模型的 input/output 价格（小字体）
- 保留点击价格可编辑的能力（弹出 Dialog）

**理由**: 三栏布局在移动端不可行（应用是 mobile-first 设计），双栏已经比较紧凑。价格信息附着在模型卡片上更自然，因为价格本质上是模型的属性。

**替代方案**: Tab 切换（Channels | Groups | Prices）— 放弃，因为这会破坏拖拽交互，拖拽需要同时看到 Channels 和 Groups。

### 2. 路由策略

**选择**: `/model` 作为主路由，`/channels` 和 `/prices` 重定向到 `/model`

**理由**:

- "Model" 是更高层次的抽象，涵盖了渠道（模型的来源）、分组（模型的调度）、定价（模型的成本）
- 保持旧路由重定向确保书签和外部链接不会失效
- `/groups` 已有重定向先例（当前重定向到 `/channels`）

### 3. 价格数据的获取策略

**选择**: 在 Model 页面中同时 query `model-prices` 数据，将价格 Map 传递给 Channel/Group 子组件

**理由**: TanStack Query 自动缓存和去重，添加一个额外的 query 不会造成性能问题。价格数据量通常不大（几百条），可以全量加载后在前端查找。

### 4. 导航缩减

**选择**: 底部导航从 5 项变为 4 项：Dashboard、Model、Logs、Settings

**理由**: 移除 Prices 独立入口后只剩 4 项，移动端底部导航 4 项是最佳实践（少于 5 项更容易点击，每个按钮更宽）。图标从 `Radio` 改为 `Boxes`（lucide）更能代表"模型"的含义。

### 5. 文件组织

**选择**:

- 将 `pages/channels.tsx` 重命名为 `pages/model.tsx`
- 将 `pages/channels/` 目录重命名为 `pages/model/`
- 将 `pages/prices.tsx` 中的价格表单组件迁移到 `pages/model/price-dialog.tsx`
- 删除 `pages/prices.tsx`
- 将 `pages/groups.tsx`（重定向组件）更新为重定向到 `/model`

**理由**: 文件名与路由保持一致是项目的现有惯例。

### 6. i18n 翻译命名空间

**选择**: 创建新的 `model.json` 翻译文件，合并 `channels.json` 和 `prices.json` 中的翻译键。保留旧文件但标记为废弃。

**替代方案**: 直接在 `channels.json` 中添加价格翻译 — 放弃，因为命名空间名称与实际功能不符。

**最终方案调整**: 将 `channels.json` 重命名为 `model.json`，将 `prices.json` 的内容合并进去，删除 `prices.json`。

## Risks / Trade-offs

- **[书签失效]** → 通过旧路由重定向缓解。`/channels`、`/channels?highlight=X`、`/prices` 都会正确重定向到 `/model`
- **[页面复杂度增加]** → Model 页面将包含更多状态和 query。通过将价格功能封装为独立子组件（PriceDialog、价格数据 hook）来管理复杂度
- **[ModelCard 信息密度]** → 在小卡片中同时显示模型名和价格可能拥挤。使用更小的字体和条件显示（仅当有价格数据时显示）来缓解
- **[导航项减少]** → 用户可能习惯了独立的 Prices 入口。但功能并未移除，只是整合到了 Model 页面的工具栏中
