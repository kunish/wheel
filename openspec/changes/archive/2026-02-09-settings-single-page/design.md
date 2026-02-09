## Context

当前设置页面 (`apps/web/src/app/(protected)/settings/page.tsx`, ~792 行) 使用 Radix UI Tabs 将 4 个功能区分为独立 tab：API Keys、Account、System、Backup。所有 section 组件已经是独立函数，每个返回独立的 Card 结构，仅在主 `SettingsPage()` 中通过 Tabs 组件组合。

## Goals / Non-Goals

**Goals:**

- 移除 Tab 导航，将所有 section 垂直排列在同一页面
- 为 API Keys section 添加 Card 包裹，保持与其他 section 一致
- 保留所有现有功能和交互不变
- 保留现有组件分区结构

**Non-Goals:**

- 不重构各 section 的内部实现
- 不拆分为多个文件（单文件结构保持不变）
- 不修改 API 接口或数据层

## Decisions

### 1. 布局方式：垂直卡片堆叠

所有 4 个 section 组件按 API Keys → System → Account → Backup 顺序垂直排列。每个 section 已经使用 `Card` 组件包裹（除 API Keys 外），保持现有结构。

API Keys section 需要额外用 Card 包裹，使其与其他 section 视觉一致。

**理由**: 最小改动量，仅需修改主 `SettingsPage()` 组件的 ~30 行代码。

### 2. 排序逻辑

顺序为：API Keys → Account → System → Backup

- API Keys 最常用，放在顶部
- Account 次常用（修改用户名/密码）
- System 配置较少变动
- Backup 最不频繁使用，放底部

### 3. 移除 Tabs 依赖

从该文件移除 `Tabs, TabsContent, TabsList, TabsTrigger` 的 import。不删除 `ui/tabs.tsx` 组件文件，因为可能有其他页面使用。

## Risks / Trade-offs

- [页面变长] → 功能不多（4 个 section），滚动量可接受
- [Tabs 组件可能无其他引用] → 保留不删除，影响为零
