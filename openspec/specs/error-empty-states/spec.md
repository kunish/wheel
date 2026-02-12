## ADDED Requirements

### Requirement: Query error states display inline error banner

当 useQuery 请求失败时，对应的数据区域 SHALL 显示 inline 错误信息和重试按钮，而非静默失败或显示空白。

#### Scenario: Dashboard stats query fails

- **WHEN** Dashboard 页面的 stats/chart/heatmap 查询返回错误
- **THEN** 对应的 Card 区域 SHALL 显示错误信息文本和 "Retry" 按钮
- **AND** 点击 Retry SHALL 重新发起查询

#### Scenario: Logs query fails

- **WHEN** Logs 页面的日志列表查询返回错误
- **THEN** 表格区域 SHALL 显示错误信息和 "Retry" 按钮
- **AND** SHALL 显示具体错误消息（如 error.message）

### Requirement: Loading states use skeleton placeholders

数据加载中 SHALL 显示骨架屏或 spinner，而非空白或纯文字 "Loading..."。

#### Scenario: Dashboard initial load

- **WHEN** Dashboard 页面首次加载，stats 数据尚未返回
- **THEN** TotalSection 的 4 个统计卡片 SHALL 显示 Skeleton 占位
- **AND** 活动热力图区域 SHALL 显示居中的 Loader spinner

#### Scenario: Logs detail panel loading

- **WHEN** 用户点击日志行打开详情面板，detail 数据加载中
- **THEN** 详情面板 SHALL 显示匹配面板结构的 Skeleton 组件
- **AND** SHALL 不再显示纯文字 "Loading..."

### Requirement: Empty states provide contextual guidance

空状态 SHALL 区分"无数据"和"无匹配结果"，并提供相应的引导文案。

#### Scenario: Logs page with no data at all

- **WHEN** 数据库中没有任何日志记录且无筛选条件
- **THEN** 空状态 SHALL 显示 "No logs yet. Logs will appear here after API requests."

#### Scenario: Logs page with active filters but no matches

- **WHEN** 用户应用了筛选条件但无匹配结果
- **THEN** 空状态 SHALL 显示 "No logs match your filters"
- **AND** SHALL 显示一个 "Clear filters" 按钮

#### Scenario: Dashboard channel/model ranking with no data

- **WHEN** Dashboard 的 channel 或 model 排名无数据
- **THEN** 空状态 SHALL 显示引导文案 "Make API requests through configured channels to see stats here"
