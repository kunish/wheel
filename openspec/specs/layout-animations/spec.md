## ADDED Requirements

### Requirement: Card expand/collapse animation

Channel 和 Group 卡片的展开/折叠 SHALL 带有平滑高度过渡。

#### Scenario: Expanding a collapsed card

- **WHEN** 用户点击 Channel 或 Group 卡片的展开按钮
- **THEN** 卡片内容区域 SHALL 通过高度动画（grid-template-rows 0fr → 1fr）平滑展开，duration 约 250ms

#### Scenario: Collapsing an expanded card

- **WHEN** 用户点击已展开卡片的折叠按钮
- **THEN** 卡片内容区域 SHALL 通过高度动画平滑收起，duration 约 200ms

### Requirement: Sort/reorder layout animation

列表排序变化时 SHALL 带有布局过渡动画。

#### Scenario: Dashboard ranking reorder

- **WHEN** Dashboard 数据刷新导致 Channel 或 Model 排名顺序变化
- **THEN** 条目 SHALL 通过 layout animation 平滑移动到新位置，而非瞬间跳变

#### Scenario: Table sort change

- **WHEN** 用户在表格中切换排序列
- **THEN** 行 SHALL 以平滑过渡移动到新位置（如果使用 motion layout），或至少以 fade 过渡刷新
