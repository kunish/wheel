## ADDED Requirements

### Requirement: Merge Request and Response into single tab

DetailPanel SHALL 将原来的 "Request" 和 "Response" 两个独立 Tab 合并为一个 "Messages" Tab。合并后的 Tab 内 SHALL 垂直排列展示 Request 和 Response 内容。

#### Scenario: View merged messages tab

- **WHEN** 用户点击 "Messages" Tab
- **THEN** SHALL 依次垂直显示：Request (Original) 区域、Request (Upstream) 区域、Response 区域
- **AND** 每个区域 SHALL 有独立的标题栏

#### Scenario: Tab list update

- **WHEN** DetailPanel 渲染 Tab 列表
- **THEN** Tab 列表 SHALL 为：Overview / Messages / Retry Timeline
- **AND** 原来的 "Request" 和 "Response" 独立 Tab SHALL 不再存在

### Requirement: Each section has independent controls

合并后的 Messages Tab 中，Request 和 Response 区域 SHALL 各自保留独立的搜索和复制功能。

#### Scenario: Search within request section

- **WHEN** 用户在 Request 区域的搜索框中输入关键词
- **THEN** 仅 Request 区域的内容 SHALL 高亮匹配文本
- **AND** Response 区域 SHALL 不受影响

#### Scenario: Copy response content

- **WHEN** 用户点击 Response 区域的复制按钮
- **THEN** 仅 Response 的内容 SHALL 被复制到剪贴板

### Requirement: Sections are collapsible

Messages Tab 中的 Request 和 Response 区域 SHALL 支持独立折叠/展开。

#### Scenario: Collapse request section

- **WHEN** 用户点击 Request 区域标题栏的折叠按钮
- **THEN** Request 内容 SHALL 被折叠隐藏
- **AND** Response 区域 SHALL 保持当前展开/折叠状态不变
- **AND** 标题栏 SHALL 保持可见以便重新展开

#### Scenario: Default state

- **WHEN** Messages Tab 首次打开
- **THEN** Request 和 Response 区域 SHALL 均为展开状态

### Requirement: Visual separator between sections

各内容区域之间 SHALL 有清晰的视觉分隔。

#### Scenario: Separator rendering

- **WHEN** Messages Tab 显示多个内容区域
- **THEN** 各区域之间 SHALL 有分隔线或适当的间距
- **AND** 每个区域的标题栏 SHALL 使用不同的背景色或边框以区分

### Requirement: Store upstream request body in database

Worker 在将请求转发给上游 provider 后 SHALL 将转换后的请求体存储到 `relay_logs` 表的 `upstream_content` 字段。

#### Scenario: Request with model name and parameter transformation

- **WHEN** 用户请求模型 `gpt-4o`，channel 将其映射为 `gpt-4o-2024-11-20` 并注入 `max_tokens` 参数
- **THEN** `request_content` SHALL 存储用户原始请求体（包含 `model: "gpt-4o"`）
- **AND** `upstream_content` SHALL 存储转换后的请求体（包含 `model: "gpt-4o-2024-11-20"` 和注入的参数）

#### Scenario: Request with cross-format conversion

- **WHEN** 用户以 OpenAI 格式发送请求，channel 类型为 Anthropic（需要格式转换）
- **THEN** `upstream_content` SHALL 存储转换为 Anthropic 格式后的请求体

#### Scenario: Upstream content truncation

- **WHEN** 转换后的请求体超过截断限制
- **THEN** `upstream_content` SHALL 使用与 `request_content` 相同的 `truncateForLog()` 策略截断

#### Scenario: No transformation applied

- **WHEN** 请求直接透传无任何转换（模型名、参数均不变）
- **THEN** `upstream_content` SHALL 为 NULL 或空字符串（避免冗余存储）

### Requirement: Display upstream request in Messages tab

Messages Tab SHALL 展示上游请求区域（"Request (Upstream)"），当 `upstreamContent` 存在且与原始请求不同时。

#### Scenario: Upstream content available and differs

- **WHEN** 日志的 `upstreamContent` 存在且不为空
- **THEN** Messages Tab SHALL 在 Request (Original) 和 Response 之间显示 "Request (Upstream)" 区域
- **AND** 该区域默认 SHALL 为折叠状态

#### Scenario: Upstream content not available

- **WHEN** 日志的 `upstreamContent` 为 NULL 或空（历史数据或无转换）
- **THEN** Messages Tab SHALL 不显示 "Request (Upstream)" 区域
- **AND** 仅显示 Request (Original) 和 Response

#### Scenario: Upstream section has independent controls

- **WHEN** "Request (Upstream)" 区域展开
- **THEN** 该区域 SHALL 拥有独立的搜索和复制功能，与其他区域互不影响
