## Context

Logs 页面（1400+ 行）和 Prices 页面目前使用手动状态管理（`useState` + `useMemo`）实现排序。当前排序实现存在以下问题：

1. **二态排序**：`toggleSort` 只在 asc/desc 之间切换，无法回到"不排序"状态
2. **TTFT 列未启用排序**：`SortField` 类型只包含 `inputTokens | outputTokens | useTime | cost`，`ftut` 被遗漏
3. **手动实现**：排序、列定义、渲染全部耦合在一个巨大的组件里
4. **无 grouping 支持**：无法按 model 或 channel 分组查看日志

现有 shadcn/ui Table 组件（`components/ui/table.tsx`）是纯样式封装（Table/TableHeader/TableBody/TableRow/TableHead/TableCell），TanStack Table 可以直接使用这些组件作为渲染层。

## Goals / Non-Goals

**Goals:**

- 引入 `@tanstack/react-table` 统一表格状态管理
- 所有可排序列支持三态排序：asc → desc → false（不排序）
- TTFT 列（字段 `ftut`）启用排序
- Logs 表格支持按 model 或 channel 进行 row grouping
- 保留现有 shadcn/ui Table 组件作为渲染层
- 保留现有的 WebSocket 实时推送、分页、筛选等功能不变

**Non-Goals:**

- 不改造 Prices 页面（结构简单，无排序需求，引入 TanStack Table 过度工程化）
- 不实现虚拟滚动（当前分页最大 100 条，无性能瓶颈）
- 不实现服务端排序（保持现有客户端排序模式）
- 不引入列隐藏/列拖拽等高级功能

## Decisions

### 1. 仅改造 Logs 页面

**选择**: 只在 Logs 页面引入 TanStack Table，Prices 页面保持不变。

**理由**: Prices 页面只有简单的搜索过滤，无排序需求，表格结构简单。引入 TanStack Table 会增加不必要的复杂度。如果未来 Prices 需要排序，可以单独迁移。

**替代方案**: 两个页面都改造 → 拒绝，因为 Prices 页面改造收益极低。

### 2. 使用 TanStack Table 的内置排序 + 三态切换

**选择**: 使用 `getSortedRowModel()` + `enableSortingRemoval: true`（默认行为），排序状态由 TanStack Table 管理。

**理由**: TanStack Table 的 `SortingState` 天然支持三态（排序字段在数组中 = 有排序，不在 = 无排序）。`column.getToggleSortingHandler()` 默认实现 asc → desc → false 循环。

**替代方案**: 继续手动管理排序状态 → 拒绝，需要自己实现三态逻辑且无法利用 TanStack 的 sorting model。

### 3. Grouping 实现方式

**选择**: 使用 TanStack Table 的 `getGroupedRowModel()` + `getExpandedRowModel()`，通过 UI 控件让用户选择分组字段（model/channel/none）。

**理由**: TanStack Table 的 grouping 功能成熟，支持聚合函数（count, sum 等），可以直接用于展示分组统计。

### 4. 保留 shadcn/ui Table 作为渲染层

**选择**: TanStack Table 只负责状态管理（headless），渲染继续使用现有的 `Table/TableHeader/TableBody/TableRow/TableHead/TableCell` 组件。

**理由**: 这正是 TanStack Table 的设计理念（headless UI）。现有组件的样式已经很好，无需替换。

### 5. 列定义提取

**选择**: 将列定义提取到 `logs/columns.tsx` 文件中，使用 `createColumnHelper<LogEntry>()` 定义。

**理由**: 列定义包含排序配置、cell 渲染、header 渲染等，集中管理便于维护。主组件只关注数据获取和布局。

## Risks / Trade-offs

- **包体积增加** → `@tanstack/react-table` gzipped ~15KB，可接受。且项目已使用 `@tanstack/react-query`，共享部分底层代码。
- **学习曲线** → TanStack Table API 需要熟悉。通过提取清晰的列定义文件降低认知负担。
- **WebSocket 实时推送兼容性** → 现有 `useWsEvent` 直接操作 react-query cache 注入新行，TanStack Table 的 `data` prop 变化会自动触发重新排序/分组，无需特殊处理。
- **Detail panel 导航** → 现有 `sortedLogs` 用于上下导航，改为使用 `table.getRowModel().rows` 获取排序后的行列表，功能等价。
