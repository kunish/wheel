## Why

日志详情的 Messages 标签页目前只解析并展示了请求体中的 `messages` 数组和响应体中的 `content`/`tool_calls`/`finish_reason`，但后端实际存储了完整的请求和响应 JSON。对于调试 function calling、参数调优、成本分析等场景，用户需要看到请求中的 `tools` 定义、`tool_choice`、`temperature`、`max_tokens` 等参数，以及响应中的 `usage` 详情、`id`、`model` 等元数据。这些信息当前只能通过切换到 Raw JSON 视图手动查找，体验很差。

## What Changes

- 在 Messages 标签页的会话视图顶部，新增**请求参数摘要区域**，展示 `model`、`stream`、`temperature`、`max_tokens`、`top_p`、`frequency_penalty`、`presence_penalty`、`response_format`、`seed` 等关键请求参数
- 新增 **Tools 定义列表**，当请求包含 `tools` 数组时，以可折叠列表形式展示每个 tool 的 `function.name`、`function.description`、`function.parameters`（JSON Schema），并显示 `tool_choice` 设置
- 在 ResponseBlock 底部新增**响应元数据区域**，展示 `id`、`model`、`created`、`system_fingerprint`
- 在 ResponseBlock 中新增 **Usage 详情区域**，展示 `usage.prompt_tokens`、`usage.completion_tokens`、`usage.total_tokens`，以及 `prompt_tokens_details`（`cached_tokens`）和 `completion_tokens_details`（`reasoning_tokens`）等子字段
- 补充相应的 i18n 翻译 key（中文和英文）

## Capabilities

### New Capabilities

- `log-request-params-display`: 解析并展示请求体中的非 messages 参数（model、temperature、max_tokens、stream 等配置参数）
- `log-tools-display`: 解析并展示请求体中的 tools 定义列表和 tool_choice 设置
- `log-response-metadata`: 解析并展示响应体中的元数据（id、model、created、system_fingerprint）和 usage 详情（含 token 细分）

### Modified Capabilities

_(无需修改现有 spec 的需求)_

## Impact

- **前端文件**: `apps/web/src/pages/logs.tsx` — 修改 `MessagesTabContent`、`ResponseBlock` 组件，新增 `RequestParamsSummary`、`ToolsDefinitionList`、`ResponseMetadata`、`UsageDetails` 组件
- **类型定义**: 扩展 `ParsedResponse` 接口以包含 usage、id、model 等字段；新增 `ParsedRequestParams` 接口
- **i18n**: `apps/web/src/i18n/locales/en/logs.json` 和 `zh/logs.json` — 新增翻译 key
- **无后端改动**: 后端已存储完整 JSON，仅需前端解析展示
