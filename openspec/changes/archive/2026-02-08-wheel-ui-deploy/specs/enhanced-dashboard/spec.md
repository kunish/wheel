## ADDED Requirements

### Requirement: 全局统计概览

系统 SHALL 在首页仪表盘展示全局统计数据：总请求数、总 token 消耗、总成本、活跃 Channel 数量、活跃 Group 数量。数据通过 `GET /api/v1/stats/global` 获取。

#### Scenario: 展示全局统计

- **WHEN** 用户访问首页仪表盘
- **THEN** 系统展示统计卡片，包含总请求数、总输入/输出 token、总成本、活跃 Channel/Group 数量

### Requirement: 请求趋势图表

系统 SHALL 在仪表盘展示请求量和成本的时间趋势图表，支持按天/周/月切换时间范围。使用 Recharts 渲染。

#### Scenario: 查看每日请求趋势

- **WHEN** 用户选择"按天"视图
- **THEN** 系统展示最近 7 天每天的请求数量折线图和成本柱状图

#### Scenario: 切换时间范围

- **WHEN** 用户从"按天"切换到"按月"
- **THEN** 图表更新为按月聚合的数据视图

### Requirement: Channel 管理增强

系统 SHALL 提供增强的 Channel 管理界面，支持：拖拽排序、批量启用/禁用、模型列表可视化、延迟指标展示。

#### Scenario: Channel 列表展示

- **WHEN** 用户访问 Channel 管理页
- **THEN** 系统以卡片或表格形式展示所有 Channel，包含名称、类型图标、启用状态、模型数量、API Key 数量

#### Scenario: 快速启用/禁用 Channel

- **WHEN** 用户点击 Channel 卡片上的启用/禁用开关
- **THEN** 系统立即更新 Channel 状态，UI 实时反映变更

### Requirement: Group 管理增强

系统 SHALL 提供增强的 Group 管理界面，支持：可视化负载均衡配置、Channel 分配拖拽、权重/优先级直观编辑。

#### Scenario: 可视化负载均衡模式

- **WHEN** 用户编辑 Group 配置
- **THEN** 系统根据所选负载均衡模式（RoundRobin/Failover/Weighted）展示不同的配置 UI——Weighted 模式显示权重滑块，Failover 模式显示优先级排序

#### Scenario: Channel 分配编辑

- **WHEN** 用户在 Group 编辑页添加或移除 Channel
- **THEN** 系统在可用 Channel 列表和已分配 Channel 列表间提供拖拽或选择交互

### Requirement: 请求日志查看

系统 SHALL 提供请求日志列表页，支持分页、按模型/Channel/状态筛选、查看单条日志详情（包含完整的重试链路）。

#### Scenario: 查看日志列表

- **WHEN** 用户访问日志页
- **THEN** 系统展示分页的日志列表，每条显示时间、模型、Channel、token 数、耗时、成本、状态

#### Scenario: 查看日志详情

- **WHEN** 用户点击某条日志
- **THEN** 系统展示完整详情，包含请求/响应内容摘要和所有 Channel 重试尝试的时间线

#### Scenario: 按条件筛选

- **WHEN** 用户选择筛选条件（如模型="gpt-4o"，状态="失败"）
- **THEN** 日志列表实时过滤，仅显示符合条件的记录

### Requirement: 暗色模式支持

系统 SHALL 支持亮色和暗色两种主题模式，默认跟随系统设置，用户可手动切换。

#### Scenario: 跟随系统主题

- **WHEN** 用户首次访问，浏览器设置为暗色模式
- **THEN** 系统自动使用暗色主题渲染界面

#### Scenario: 手动切换主题

- **WHEN** 用户点击导航栏的主题切换按钮
- **THEN** 系统立即切换主题模式，并持久化用户偏好

### Requirement: 响应式布局

系统 SHALL 在桌面端（≥1024px）和移动端（<768px）均提供良好的使用体验。移动端使用底部导航栏替代侧边栏。

#### Scenario: 桌面端布局

- **WHEN** 用户在宽屏设备（≥1024px）访问
- **THEN** 系统展示侧边栏导航 + 主内容区的标准管理后台布局

#### Scenario: 移动端布局

- **WHEN** 用户在移动设备（<768px）访问
- **THEN** 侧边栏收起为底部导航栏，内容区全宽展示，表格/卡片适配移动端交互
