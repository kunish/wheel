## Why

设置页面当前使用 Tab 切换（API Keys / Account / System / Backup），在功能不多的情况下增加了不必要的导航层级，用户需要来回切换 tab 才能找到需要的设置项。将所有设置功能整合到单页面中，以卡片分区的方式呈现，可以提升可发现性和操作效率。

## What Changes

- 移除 Tab 导航结构（Radix UI Tabs）
- 将 4 个 tab 内容（API Keys、Account、System Config、Backup）以垂直排列的卡片区域呈现在同一页面
- 各区域保持独立性，使用清晰的标题和分隔
- 保留所有现有功能不变（CRUD、表单、导入导出等）

## Capabilities

### New Capabilities

- `settings-single-page-layout`: 将设置页面从 tab 布局重构为单页面卡片布局，所有功能区垂直排列

### Modified Capabilities

_(无现有 spec 需要修改)_

## Impact

- **受影响文件**: `apps/web/src/app/(protected)/settings/page.tsx`（主设置页面，~792 行）
- **UI 变更**: 移除 tab 导航，改为垂直滚动的卡片布局
- **API**: 无变化
- **依赖**: 可能不再需要 `@radix-ui/react-tabs`（如果没有其他页面使用）
