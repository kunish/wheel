## ADDED Requirements

### Requirement: New log row slide-in animation

日志页通过 WebSocket 收到的新行 SHALL 带有滑入动画。

#### Scenario: Real-time log insertion

- **WHEN** WebSocket 推送 `log-created` 事件且用户在第一页无过滤条件
- **THEN** 新日志行 SHALL 从顶部滑入（translateY + opacity），duration 约 300ms

#### Scenario: Batch log refresh

- **WHEN** 用户点击 "X new logs available" 按钮刷新日志
- **THEN** 新加载的日志 SHALL 以渐入（fade-in）方式出现

### Requirement: New logs badge pulse

"有新日志"提示按钮 SHALL 带有视觉脉冲效果吸引注意。

#### Scenario: Pending logs notification

- **WHEN** pendingCount > 0 且显示 "X new logs available" 按钮
- **THEN** 按钮 SHALL 带有脉冲动画（pulse/glow）直到用户点击

### Requirement: CRUD list item animations

所有列表页的增删操作 SHALL 带有过渡动画。

#### Scenario: Item creation fade-in

- **WHEN** 用户创建新的 channel/group/apikey/price 条目
- **THEN** 新条目 SHALL 以 fade-in + slight scale 动画出现，duration 约 200ms

#### Scenario: Item deletion fade-out

- **WHEN** 用户删除一个列表条目
- **THEN** 该条目 SHALL 以 fade-out 动画消失后从 DOM 移除，duration 约 200ms

#### Scenario: Item update highlight

- **WHEN** 用户编辑并保存一个列表条目
- **THEN** 该条目 SHALL 短暂高亮闪烁（subtle highlight flash）以确认更新成功
