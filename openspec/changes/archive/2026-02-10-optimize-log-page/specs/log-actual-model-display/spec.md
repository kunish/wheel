## ADDED Requirements

### Requirement: Display actual model ID alongside beautified name

日志详情面板 Overview 中的模型流可视化区域 SHALL 同时展示美化模型名和实际模型 ID。当美化名与原始 ID 不同时，原始 ID SHALL 以小字体显示在美化名下方。

#### Scenario: Model with different beautified name

- **WHEN** 详情面板展示一个 modelId 为 `claude-sonnet-4-5-20250929`、美化名为 "Claude Sonnet 4.5" 的日志
- **THEN** 模型流中 SHALL 显示 "Claude Sonnet 4.5" 作为主要名称
- **AND** 下方 SHALL 以 `text-xs text-muted-foreground font-mono` 样式显示 `claude-sonnet-4-5-20250929`

#### Scenario: Model with no beautified name (fallback)

- **WHEN** 模型的 fuzzyLookup 未找到匹配的元数据
- **THEN** 模型流中 SHALL 只显示原始 modelId
- **AND** 不显示额外的小字体行（因为没有区别）

#### Scenario: Model where beautified name equals ID

- **WHEN** 模型的美化名与原始 ID 完全相同
- **THEN** 模型流中 SHALL 只显示一次名称
- **AND** 不显示重复的小字体行

### Requirement: Actual model ID is copyable

模型流中显示的实际模型 ID SHALL 支持点击复制到剪贴板。

#### Scenario: Copy actual model ID

- **WHEN** 用户点击模型流中的实际模型 ID 文本
- **THEN** 该 ID SHALL 被复制到剪贴板
- **AND** SHALL 显示简短的 "Copied" 确认提示

### Requirement: Log table model tooltip shows actual ID

日志表格中的 ModelBadge SHALL 在 hover 时通过 Tooltip 显示实际模型 ID。

#### Scenario: Hover model badge in table

- **WHEN** 用户将鼠标悬停在日志表格中的 ModelBadge 上
- **THEN** Tooltip SHALL 显示原始 modelId
- **AND** 美化名 SHALL 继续作为 badge 的显示文本
