## ADDED Requirements

### Requirement: Log table Model column links to Channels page

日志表格中的 Model 列 SHALL 渲染为可点击链接。点击后 SHALL 导航到 `/channels?highlight=<channelId>` 并定位到包含该模型的 channel。

#### Scenario: Click model in log table

- **WHEN** 用户点击日志表格中某行的 Model 名称
- **THEN** 浏览器 SHALL 导航到 `/channels?highlight=<channelId>`
- **AND** Channels 页面 SHALL 滚动到对应 channel 并高亮显示

#### Scenario: Model with no associated channel

- **WHEN** 日志记录中的 `channelId` 为空或对应 channel 已被删除
- **THEN** Model 名称 SHALL 显示为普通文本（不可点击）

### Requirement: Log table Channel column links to Channels page

日志表格中的 Channel 列 SHALL 渲染为可点击链接。点击后 SHALL 导航到 `/channels?highlight=<channelId>`。

#### Scenario: Click channel in log table

- **WHEN** 用户点击日志表格中某行的 Channel 名称
- **THEN** 浏览器 SHALL 导航到 `/channels?highlight=<channelId>`
- **AND** Channels 页面 SHALL 滚动到对应 channel 并高亮显示

### Requirement: Detail panel Model and Channel are clickable

DetailPanel Overview 中的模型流展示区域（`requestModelName → channel → actualModelName`）中的 Model 和 Channel 名称 SHALL 为可点击链接，行为与表格中一致。

#### Scenario: Click model in detail panel

- **WHEN** 用户点击详情面板中模型流的 requestModelName 或 actualModelName
- **THEN** 浏览器 SHALL 导航到 `/channels?highlight=<channelId>`

#### Scenario: Click channel in detail panel

- **WHEN** 用户点击详情面板中模型流的 channel 名称
- **THEN** 浏览器 SHALL 导航到 `/channels?highlight=<channelId>`

### Requirement: Channels page highlight and scroll to target

Channels 页面 SHALL 读取 URL 中的 `highlight` query 参数，定位到对应 channel 元素并执行高亮动画。

#### Scenario: Navigate with highlight param

- **WHEN** 用户导航到 `/channels?highlight=<channelId>`
- **THEN** 页面 SHALL 滚动到对应 channel 卡片的位置（`scrollIntoView` with `behavior: 'smooth', block: 'center'`）
- **AND** 该 channel 卡片 SHALL 显示临时高亮效果（ring 边框 + pulse 动画）
- **AND** 高亮效果 SHALL 在 3 秒后自动淡出

#### Scenario: Channel is in a collapsed section

- **WHEN** 目标 channel 处于折叠状态
- **THEN** 页面 SHALL 先自动展开包含该 channel 的区域
- **AND** 然后执行滚动和高亮

#### Scenario: Invalid highlight param

- **WHEN** `highlight` 参数指向不存在的 channelId
- **THEN** 页面 SHALL 正常加载，不执行任何滚动或高亮

#### Scenario: Highlight cleared after navigation

- **WHEN** 高亮动画完成后
- **THEN** URL 中的 `highlight` 参数 SHALL 被静默移除（使用 `router.replace` 不添加历史记录）
