## ADDED Requirements

### Requirement: Logout requires confirmation

登出操作 SHALL 需要用户确认，防止误触。

#### Scenario: Click logout button

- **WHEN** 用户点击顶栏的登出按钮
- **THEN** SHALL 弹出确认对话框 "Are you sure you want to logout?"
- **AND** 确认后 SHALL 执行登出并跳转到登录页
- **AND** 取消 SHALL 关闭对话框且不执行任何操作

### Requirement: Auth check shows loading spinner

认证检查期间 SHALL 显示加载指示器而非空白页面。

#### Scenario: Protected layout auth check

- **WHEN** 用户访问受保护的页面，认证状态尚未确定
- **THEN** SHALL 显示全屏居中的 Loader2 spinner
- **AND** SHALL 不返回 null（避免白屏闪烁）

### Requirement: API Key creation dialog prevents accidental close

新创建的 API Key 显示时 SHALL 阻止用户意外关闭弹窗导致 key 丢失。

#### Scenario: Try to close dialog while created key is shown

- **WHEN** 用户刚创建了 API Key，弹窗显示了新 key 的值
- **AND** 用户尝试点击弹窗外部、按 ESC、或点击关闭按钮
- **THEN** 弹窗 SHALL 不关闭
- **AND** SHALL 显示提示 "Please copy the key before closing"

#### Scenario: Close dialog after copying key

- **WHEN** 用户已经点击了复制按钮（key 已复制到剪贴板）
- **THEN** 弹窗 SHALL 允许正常关闭

### Requirement: Login button shows loading spinner

登录按钮在请求进行中 SHALL 显示 spinner 动画。

#### Scenario: Login request in progress

- **WHEN** 用户点击 Sign In 按钮，登录请求正在处理中
- **THEN** 按钮 SHALL 显示 Loader2 spinner + "Signing in..." 文字
- **AND** 按钮 SHALL 处于 disabled 状态
