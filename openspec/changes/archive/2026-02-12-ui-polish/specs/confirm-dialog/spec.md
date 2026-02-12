## ADDED Requirements

### Requirement: Destructive actions use AlertDialog confirmation

所有删除和不可逆操作 SHALL 使用 shadcn AlertDialog 组件替代原生 `window.confirm()`。AlertDialog SHALL 包含操作描述、后果说明和明确的取消/确认按钮。

#### Scenario: Delete API key confirmation

- **WHEN** 用户点击 Settings 页面 API key 行的删除按钮
- **THEN** SHALL 弹出 AlertDialog，标题为 "Delete API Key \"<name>\"?"
- **AND** 描述 SHALL 说明 "This action cannot be undone. Active integrations using this key will stop working."
- **AND** 确认按钮 SHALL 使用 destructive variant
- **AND** 取消按钮 SHALL 关闭弹窗且不执行任何操作

#### Scenario: Delete price confirmation

- **WHEN** 用户点击 Prices 页面某条价格记录的删除按钮
- **THEN** SHALL 弹出 AlertDialog，标题为 "Delete price for \"<modelName>\"?"
- **AND** 描述 SHALL 说明 "Historical logs using this model will show $0 cost."

#### Scenario: Clear group items confirmation

- **WHEN** 用户点击 Channels 页面 group 的清空操作
- **THEN** SHALL 弹出 AlertDialog 确认，而非原生 confirm()

#### Scenario: Delete channel confirmation

- **WHEN** 用户点击删除 channel 按钮
- **THEN** SHALL 弹出 AlertDialog 确认

#### Scenario: Delete group confirmation

- **WHEN** 用户点击删除 group 按钮
- **THEN** SHALL 弹出 AlertDialog 确认
