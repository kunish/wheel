## ADDED Requirements

### Requirement: ModelCard displays model metadata

ModelCard 组件 SHALL 展示模型的 provider 图标、显示名称和 provider 名称。当元数据不可用时 SHALL 降级显示原始 modelId。

#### Scenario: Model with metadata available

- **WHEN** ModelCard 接收一个已知 modelId（如 "claude-opus-4-6"）
- **THEN** 显示 provider logo (20x20)、模型显示名称（如 "Claude Opus 4.6"）和 provider 名称（如 "Anthropic"）

#### Scenario: Model with unknown metadata

- **WHEN** ModelCard 接收一个未知 modelId
- **THEN** 显示原始 modelId 文本，不显示 logo 和 provider 名称

### Requirement: ModelCard supports removable variant

ModelCard SHALL 支持可选的删除按钮，点击后触发 `onRemove` 回调。

#### Scenario: Removable card

- **WHEN** ModelCard 传入 `onRemove` prop
- **THEN** 卡片右侧显示关闭按钮 (X icon)，点击触发回调

#### Scenario: Read-only card

- **WHEN** ModelCard 未传入 `onRemove` prop
- **THEN** 不显示关闭按钮

### Requirement: ModelCard adapts to dark mode

ModelCard 的 provider 图标 SHALL 在暗黑模式下使用 CSS `dark:invert` 保证可见性。

#### Scenario: Dark mode display

- **WHEN** 系统处于暗黑模式
- **THEN** provider 图标自动反色，文本使用暗黑模式颜色

### Requirement: Channels page uses ModelCard

Channels 页面 SHALL 在以下位置使用 ModelCard 替代 Badge + ModelBadge：

- 可拖拽模型列表中的每个模型条目
- Group 卡片内的 channel:model 映射条目
- Channel 表单中的模型 tags

#### Scenario: Draggable model list

- **WHEN** 用户查看未分组的模型列表
- **THEN** 每个模型以 ModelCard 形式展示，支持拖拽

#### Scenario: Group items display

- **WHEN** Group 包含 channel:model 映射条目
- **THEN** 每个条目使用 ModelCard 展示模型信息，附带 channel 名称标签

#### Scenario: Channel form tags

- **WHEN** 用户在 Channel 表单中添加模型
- **THEN** 已添加的模型以 ModelCard 形式展示，带删除按钮
