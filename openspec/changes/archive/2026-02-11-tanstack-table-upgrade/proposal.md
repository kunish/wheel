## Why

Logs 页面和 Prices 页面目前使用手动状态管理实现排序、过滤和分页，代码冗长且难以扩展。排序逻辑仅支持升序/降序两态切换，无法回到"不排序"状态。TTFT 列虽然展示了数据，但不支持排序。引入 TanStack Table 可以统一表格逻辑，支持三态排序（升序 → 降序 → 不排序），并为后续 grouping 等高级功能打下基础。

## What Changes

- 安装 `@tanstack/react-table` 依赖
- 重构 Logs 页面表格，使用 TanStack Table 的 `useReactTable` 管理列定义、排序状态
- 重构 Prices 页面表格，使用 TanStack Table 管理列定义和排序
- 所有可排序列支持三态排序切换：升序 → 降序 → 不排序（点击表头循环切换）
- TTFT 列启用排序功能（字段 `ftut`）
- 支持 TanStack Table 的 grouping 能力（Logs 页面可按 model/channel 分组）
- 移除 `log-filters.ts` 中的手动 `sortLogs` 函数，改用 TanStack Table 内置排序
- 更新 `SortableHead` 组件以适配 TanStack Table 的 header API

## Capabilities

### New Capabilities

- `tanstack-table-integration`: TanStack Table 集成层，包含列定义模式、三态排序、grouping 支持以及与现有 shadcn/ui Table 组件的适配

### Modified Capabilities

- `log-table-ux`: 排序行为从二态（asc/desc）变为三态（asc → desc → none），TTFT 列新增排序支持，新增 grouping 能力

## Impact

- **依赖**: 新增 `@tanstack/react-table` 包
- **代码**: `apps/web/src/app/(protected)/logs/page.tsx`（主要重构）、`apps/web/src/app/(protected)/prices/page.tsx`（适配）、`apps/web/src/app/(protected)/logs/log-filters.ts`（移除 sortLogs）
- **API**: 无后端变更，纯前端重构
- **Breaking**: 无，排序行为变更对用户透明（三态是超集）
