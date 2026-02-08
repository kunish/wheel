## Why

当前系统中模型仅以 raw ID 字符串展示（如 `gpt-4o`、`claude-sonnet-4-20250514`），缺乏可读性。models.dev 提供了丰富的模型元数据（显示名称、Provider Logo、描述、价格等），可以用来大幅提升模型在各页面中的展示质量。

## What Changes

- 新增后端接口：从 models.dev 拉取并缓存模型元数据（名称、provider、logo URL、cost 等）到 KV
- 新增前端 API 调用 + React Context/Hook：提供全局的模型元数据查询能力
- 新增 `ModelBadge` 通用组件：展示模型 logo + 显示名称，替代原始 ID 字符串
- 覆盖以下页面的模型展示：
  - **Channels 页**：channel 卡片中的模型列表、Create/Edit Channel 表单中 fetch 到的 models
  - **Prices 页**：价格表中模型名称列，增加 logo 和 provider 信息
  - **Logs 页**：日志列表中模型列
  - **Dashboard**：Channel Ranking 等图表中涉及模型的展示

## Capabilities

### New Capabilities

- `model-metadata`: 从 models.dev 获取、缓存模型元数据，并通过 API 提供给前端；前端通用组件 `ModelBadge` 展示模型 logo + 名称

### Modified Capabilities

_(无现有 spec 需要修改)_

## Impact

- **后端**：`apps/worker/src/routes/model.ts` 新增 metadata 接口；KV 缓存新增 `model-metadata` key
- **前端**：`apps/web/src/lib/api.ts` 新增调用；新增 `ModelBadge` 组件；修改 channels、prices、logs、dashboard 页面中模型展示
- **外部依赖**：`https://models.dev/api.json`（已有使用）、`https://models.dev/logos/{provider}.svg`
