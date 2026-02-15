## ADDED Requirements

### Requirement: logs.tsx 拆分为子组件

`pages/logs.tsx` SHALL 拆分为以下独立文件：

- `pages/logs/index.tsx` — 主页面容器，编排子组件
- `pages/logs/log-table.tsx` — 日志表格 + 分页逻辑
- `pages/logs/log-filters.tsx` — 筛选条件面板
- `pages/logs/log-detail-panel.tsx` — 日志详情侧边栏
- `pages/logs/log-replay-dialog.tsx` — 请求重放对话框

每个子组件 SHALL 通过 props 接收数据和回调，不直接访问全局状态。

#### Scenario: 日志页面功能不变

- **WHEN** 用户访问日志页面
- **THEN** 所有现有功能（表格浏览、筛选、详情查看、重放）SHALL 与拆分前行为完全一致

#### Scenario: 子组件独立渲染

- **WHEN** 筛选条件变化
- **THEN** 仅 log-table 重新渲染，log-detail-panel 和 log-replay-dialog SHALL 不受影响

### Requirement: 日志查询逻辑提取为 hook

日志数据获取逻辑 SHALL 提取为 `hooks/use-log-query.ts`，封装 TanStack Query 调用、分页状态、筛选参数管理。

#### Scenario: hook 提供完整的查询接口

- **WHEN** 日志页面挂载
- **THEN** useLogQuery SHALL 返回 `{ data, isLoading, filters, setFilters, pagination, setPagination }`

### Requirement: activity-section 子组件独立文件

`activity-section.tsx` 中的以下组件 SHALL 提取为独立文件：

- `HubModelList` → `dashboard/hub-model-list.tsx`
- `HubChannelList` → `dashboard/hub-channel-list.tsx`
- `DataPanelPopover` → `dashboard/data-panel-popover.tsx`
- `InlineStats` → `dashboard/inline-stats.tsx`

#### Scenario: activity-section 主文件精简

- **WHEN** 拆分完成
- **THEN** `activity-section.tsx` SHALL 不超过 400 行，仅包含 ActivitySection 组件和日期导航逻辑

### Requirement: 日期导航逻辑提取为 hook

`ActivitySection` 中的日期计算和视图切换逻辑 SHALL 提取为 `hooks/use-date-navigation.ts`。

#### Scenario: hook 管理日期状态

- **WHEN** 用户切换周/月/年视图或导航到上一期/下一期
- **THEN** useDateNavigation SHALL 管理当前视图类型、当前日期范围、导航函数（prev/next/reset）

### Requirement: 统计汇总计算提取为工具函数

三个视图（周/月/年）中重复的统计汇总计算 SHALL 提取为 `computePeriodTotals(days)` 工具函数，返回 `{ req, inTokens, outTokens, cost }`。

#### Scenario: 三个视图使用同一函数

- **WHEN** 周/月/年视图需要计算汇总统计
- **THEN** 三个视图 SHALL 调用同一个 computePeriodTotals 函数，传入各自的数据源

### Requirement: PeriodNavBar 可复用组件

三个视图底部重复的导航栏 SHALL 提取为 `PeriodNavBar` 组件，接受 `label`、`onPrev`、`onNext`、`disableNext`、`onReset`、`resetLabel`、`onViewLogs` props。

#### Scenario: 三个视图使用同一导航栏

- **WHEN** 周/月/年视图渲染底部导航
- **THEN** 三个视图 SHALL 使用同一个 PeriodNavBar 组件

### Requirement: ModelPickerBase 可复用组件

`model-picker-dialog.tsx` 和 `channel-model-picker-dialog.tsx` 的公共逻辑 SHALL 提取为 `ModelPickerBase` 组件，包含搜索框、按 provider 分组、排序、带 logo 的列表项渲染。两个对话框 SHALL 复用 ModelPickerBase。

#### Scenario: 单选和多选模式

- **WHEN** ModelPickerBase 接收 `multiSelect={false}`
- **THEN** 点击模型项 SHALL 直接选中并关闭
- **WHEN** ModelPickerBase 接收 `multiSelect={true}`
- **THEN** 点击模型项 SHALL 切换选中状态，支持批量选择

### Requirement: Month 视图统一使用 InlineStats

Month 视图中手动渲染的统计行 SHALL 替换为 `InlineStats` 组件，与 Week 和 Year 视图保持一致。

#### Scenario: 三个视图统计行样式一致

- **WHEN** 用户查看周/月/年任一视图的统计汇总
- **THEN** 三个视图的统计行 SHALL 使用相同的 InlineStats 组件，样式完全一致
