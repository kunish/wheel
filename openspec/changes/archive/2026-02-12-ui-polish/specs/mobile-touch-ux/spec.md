## ADDED Requirements

### Requirement: Touch drag-and-drop support on mobile

Channels 页面的拖拽功能 SHALL 在触摸设备上正常工作。

#### Scenario: Drag channel to group on touch device

- **WHEN** 用户在触摸设备上长按 channel 的拖拽手柄 250ms
- **THEN** 拖拽 SHALL 激活，显示 DragOverlay
- **AND** 拖拽过程中 SHALL 正常工作，包括拖放到 group

#### Scenario: Scroll does not trigger drag on touch

- **WHEN** 用户在触摸设备上快速滑动（未长按）
- **THEN** SHALL 执行正常滚动，不触发拖拽
- **AND** tolerance 设置为 5px 以区分滑动和拖拽意图

### Requirement: Drag handle activation distance prevents accidental drag

拖拽激活距离 SHALL 足够大以防止与相邻按钮的误触。

#### Scenario: Click collapse button near drag handle

- **WHEN** 用户点击紧邻拖拽手柄的折叠按钮
- **THEN** SHALL 执行折叠操作，不触发拖拽
- **AND** PointerSensor 激活距离 SHALL 为 8px（从当前 5px 增加）

### Requirement: Touch targets meet minimum size

所有可交互按钮的触摸目标 SHALL 不小于 36px（9 tailwind units）。

#### Scenario: Channel action buttons on mobile

- **WHEN** 在移动设备上查看 channel 卡片的操作按钮（编辑、删除、启用）
- **THEN** 每个按钮的最小尺寸 SHALL 为 h-9 w-9（36px）

### Requirement: Tables show horizontal scroll indicator on mobile

当表格内容超出视口宽度时 SHALL 显示视觉提示表明可水平滚动。

#### Scenario: Logs table on mobile viewport

- **WHEN** 日志表格在移动设备上显示，内容超出屏幕宽度
- **THEN** 表格 SHALL 被 overflow-x-auto 容器包裹
- **AND** 容器右侧 SHALL 显示从 background 到 transparent 的渐变遮罩提示

#### Scenario: Settings API keys table on mobile

- **WHEN** API Keys 表格在移动设备上显示
- **THEN** SHALL 同样被 overflow-x-auto 容器包裹

#### Scenario: Prices table on mobile

- **WHEN** Prices 表格在移动设备上显示
- **THEN** SHALL 同样被 overflow-x-auto 容器包裹

### Requirement: Channel dialog responsive on mobile

Channel 编辑对话框 SHALL 在移动端正确显示。

#### Scenario: Open channel dialog on mobile

- **WHEN** 用户在移动设备上打开创建/编辑 channel 的 Dialog
- **THEN** Dialog 宽度 SHALL 不超过 95vw
- **AND** Dialog SHALL 支持 overflow-y-auto 垂直滚动
