## ADDED Requirements

### Requirement: TanStack Table column definitions

Logs 页面 SHALL 使用 `@tanstack/react-table` 的 `createColumnHelper<LogEntry>()` 定义所有表格列。列定义 SHALL 提取到独立的 `columns.tsx` 文件中。

#### Scenario: Column definition structure

- **WHEN** Logs 页面渲染表格
- **THEN** 列定义 SHALL 包含以下列：Time, Model, Channel, Input Tokens, Output Tokens, TTFT, Latency, Cost, Status, Actions
- **AND** 每列 SHALL 指定 `accessorKey` 或 `accessorFn` 以及 `cell` 渲染函数

### Requirement: TanStack Table instance creation

Logs 页面 SHALL 使用 `useReactTable()` 创建表格实例，配置 `getCoreRowModel`、`getSortedRowModel`、`getGroupedRowModel` 和 `getExpandedRowModel`。

#### Scenario: Table instance initialization

- **WHEN** LogsPage 组件挂载
- **THEN** SHALL 创建一个 TanStack Table 实例
- **AND** 实例 SHALL 使用 `logs` 数组作为 `data` prop
- **AND** 实例 SHALL 使用提取的列定义作为 `columns` prop

### Requirement: Headless rendering with shadcn/ui Table

TanStack Table 实例 SHALL 通过现有 shadcn/ui `Table`、`TableHeader`、`TableBody`、`TableRow`、`TableHead`、`TableCell` 组件进行渲染。SHALL 使用 `flexRender` 渲染 header 和 cell 内容。

#### Scenario: Table renders with shadcn/ui components

- **WHEN** 表格数据加载完成
- **THEN** SHALL 使用 `table.getHeaderGroups()` 渲染表头
- **AND** SHALL 使用 `table.getRowModel().rows` 渲染数据行
- **AND** 渲染结果的 DOM 结构和样式 SHALL 与迁移前保持一致

### Requirement: Grouping support

Logs 表格 SHALL 支持按 `requestModelName`（Model）或 `channelName`（Channel）进行行分组。SHALL 提供 UI 控件让用户选择分组方式或关闭分组。

#### Scenario: Enable grouping by model

- **WHEN** 用户在分组控件中选择 "Model"
- **THEN** 表格行 SHALL 按 `requestModelName` 分组显示
- **AND** 每个分组 SHALL 显示可展开/折叠的分组行
- **AND** 分组行 SHALL 显示分组名称和该组的行数

#### Scenario: Disable grouping

- **WHEN** 用户在分组控件中选择 "None"
- **THEN** 表格 SHALL 恢复为扁平列表显示
- **AND** 所有行 SHALL 正常展示

#### Scenario: Grouping with sorting

- **WHEN** 用户同时启用了分组和排序
- **THEN** 组内的行 SHALL 按当前排序规则排序
- **AND** 分组本身的顺序 SHALL 保持稳定

### Requirement: Detail panel navigation compatibility

迁移到 TanStack Table 后，日志详情侧栏的上下导航 SHALL 继续正常工作，使用 `table.getRowModel().rows` 获取当前可见行顺序。

#### Scenario: Navigate between logs in detail panel

- **WHEN** 用户在详情面板中点击上/下导航按钮
- **THEN** SHALL 按照 TanStack Table 当前排序/分组后的行顺序导航
- **AND** 导航行为 SHALL 与迁移前一致
