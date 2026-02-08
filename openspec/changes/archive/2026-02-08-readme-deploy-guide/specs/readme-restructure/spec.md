## ADDED Requirements

### Requirement: README 顶部 Banner 区域

README SHALL 在标题下方包含项目简介、badge 行和一键部署按钮区域。

#### Scenario: 用户首次打开 README

- **WHEN** 用户浏览 README 顶部
- **THEN** SHALL 看到：项目名称、一句话描述、License/Stars badge、Deploy 按钮

### Requirement: 特性亮点章节

README SHALL 包含结构化的特性亮点列表，展示项目核心能力。

#### Scenario: 用户了解项目功能

- **WHEN** 用户查看 Features 章节
- **THEN** SHALL 列出核心特性：多 LLM 提供商支持、负载均衡、流式转发、API Key 管理、仪表盘管理界面、成本追踪

### Requirement: 部署方式渐进式展示

README SHALL 按从简单到复杂的顺序展示部署方式：一键部署 → Docker → 手动部署。

#### Scenario: 新用户选择部署方式

- **WHEN** 用户阅读部署章节
- **THEN** SHALL 看到三种部署方式按难度递增排列，每种方式有简要说明和操作步骤

### Requirement: 环境变量参考表

README SHALL 包含完整的环境变量参考表格。

#### Scenario: 用户查找环境变量配置

- **WHEN** 用户查看环境变量章节
- **THEN** SHALL 看到表格包含：变量名、适用组件（Worker/Web）、描述、是否必填、默认值
