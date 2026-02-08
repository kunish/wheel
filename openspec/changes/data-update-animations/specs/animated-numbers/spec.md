## ADDED Requirements

### Requirement: Animated number transitions

系统 SHALL 提供 `AnimatedNumber` 组件，当数值变化时平滑过渡到新值，而非硬切换。

#### Scenario: Dashboard stat card number updates

- **WHEN** Dashboard 统计卡片接收到新的数值（通过 TanStack Query refetch）
- **THEN** 数字 SHALL 通过弹性动画（spring）从旧值平滑过渡到新值，duration 不超过 500ms

#### Scenario: Number format preservation

- **WHEN** AnimatedNumber 组件接收 `formatter` 属性（如千分位、小数、百分比）
- **THEN** 动画过程中每帧的中间值 SHALL 使用相同的 formatter 进行格式化显示

#### Scenario: Rapid consecutive updates

- **WHEN** 数值在前一次动画未完成时再次变化
- **THEN** 动画 SHALL 从当前中间值平滑过渡到最新值，不产生跳变

### Requirement: Dashboard ranking bar transitions

Dashboard 排名区域的条形图宽度 SHALL 在数据变化时平滑过渡。

#### Scenario: Channel ranking re-sort

- **WHEN** 用户切换排名排序方式（cost/count）或数据刷新导致排名变化
- **THEN** 条形图宽度 SHALL 通过 CSS transition 平滑变化到新百分比宽度

#### Scenario: Model stats bar update

- **WHEN** 模型统计数据刷新
- **THEN** 各模型的使用量条形图宽度 SHALL 平滑过渡，duration 约 300ms
