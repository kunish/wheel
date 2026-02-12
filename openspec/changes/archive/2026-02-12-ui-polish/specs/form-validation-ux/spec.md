## ADDED Requirements

### Requirement: Password change requires minimum length

修改密码时 SHALL 验证新密码的最小长度。

#### Scenario: Password too short

- **WHEN** 用户输入少于 8 个字符的新密码并提交
- **THEN** SHALL 显示错误提示 "Password must be at least 8 characters"
- **AND** SHALL 不发送修改请求

#### Scenario: Password meets requirements

- **WHEN** 用户输入 8 个或更多字符的新密码且确认密码匹配
- **THEN** SHALL 正常提交修改请求

### Requirement: System config inputs constrain to valid numbers

系统配置中的数值设置 SHALL 使用 number 类型输入并限制有效范围。

#### Scenario: System config number input

- **WHEN** 系统配置表单渲染数值类型的设置项
- **THEN** 输入框 SHALL 使用 `type="number"` 且 `min="0"`
- **AND** SHALL 不接受非数字字符

### Requirement: Channel dialog resets form on close

Channel 创建/编辑对话框关闭时 SHALL 重置表单状态。

#### Scenario: Close channel dialog without saving

- **WHEN** 用户打开编辑 channel 弹窗，修改了表单内容，然后点击外部或 ESC 关闭
- **THEN** 表单状态 SHALL 被重置为初始值
- **AND** 下次打开同一 channel 的编辑弹窗 SHALL 显示最新的已保存数据

#### Scenario: Close group dialog without saving

- **WHEN** 用户关闭 group 编辑弹窗而未保存
- **THEN** 表单状态 SHALL 同样被重置

### Requirement: Enable/disable toggle shows pending state

Channel 的启用/禁用开关 SHALL 在请求进行中显示 pending 状态。

#### Scenario: Toggle channel while request pending

- **WHEN** 用户点击 channel 启用开关，API 请求尚在进行中
- **THEN** Switch 组件 SHALL 显示 disabled 状态
- **AND** SHALL 阻止重复点击直到请求完成
