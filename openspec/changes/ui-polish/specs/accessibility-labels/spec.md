## ADDED Requirements

### Requirement: Top bar buttons have accessible labels

顶栏的所有 icon-only 按钮 SHALL 包含 aria-label 属性。

#### Scenario: Theme toggle button

- **WHEN** 屏读器用户聚焦主题切换按钮
- **THEN** SHALL 播报 "Switch to light mode" 或 "Switch to dark mode"（根据当前主题）

#### Scenario: Logout button

- **WHEN** 屏读器用户聚焦登出按钮
- **THEN** SHALL 播报 "Logout"

#### Scenario: Mobile menu button

- **WHEN** 屏读器用户聚焦移动端汉堡菜单按钮
- **THEN** SHALL 播报 "Open navigation menu"

### Requirement: Action buttons in tables have accessible labels

表格中的 icon-only 操作按钮 SHALL 包含描述目标对象的 aria-label。

#### Scenario: API key copy button

- **WHEN** 屏读器用户聚焦 API key 表格中的复制按钮
- **THEN** SHALL 播报 "Copy API key <keyName>"

#### Scenario: Price edit button

- **WHEN** 屏读器用户聚焦 Prices 表格中的编辑按钮
- **THEN** SHALL 播报 "Edit price for <modelName>"

#### Scenario: Price delete button

- **WHEN** 屏读器用户聚焦 Prices 表格中的删除按钮
- **THEN** SHALL 播报 "Delete price for <modelName>"

### Requirement: Form search inputs have labels

搜索输入框 SHALL 包含 sr-only 的 Label 或 aria-label。

#### Scenario: Prices search input

- **WHEN** 屏读器用户聚焦 Prices 页面搜索框
- **THEN** SHALL 播报 "Search models"
