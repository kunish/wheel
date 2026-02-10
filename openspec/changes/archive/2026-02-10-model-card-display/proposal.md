## Why

Channels 和 Groups 页面中的模型列表使用 `<Badge>` + `<ModelBadge>` 展示，只显示一个小图标和模型名称。当模型数量较多时，密集的 badge 排列难以快速识别模型来源和区分不同 provider。改用丰富的卡片形式可以更好地展示模型的 provider 归属、名称等元信息，提升可读性和管理效率。

## What Changes

- 创建 `ModelCard` 组件替代现有的 `Badge + ModelBadge` 组合，展示模型图标、名称和 provider 信息
- Channels 页面的模型列表（channel 表单中的 tags 区域和可拖拽模型列表）使用 `ModelCard`
- Groups 页面的 group items（channel:model 映射条目）使用 `ModelCard`
- 保留 `ModelBadge` 用于其他场景（logs、dashboard 等行内展示）

## Capabilities

### New Capabilities

- `model-card`: 丰富的模型卡片组件，展示模型图标、显示名称和 provider 信息，支持可删除、可拖拽等交互变体

### Modified Capabilities

_(none)_

## Impact

- `apps/web/src/components/model-card.tsx` — 新组件
- `apps/web/src/app/(protected)/channels/page.tsx` — 替换 Badge 展示为 ModelCard
- `apps/web/src/hooks/use-model-meta.ts` — 已有，ModelCard 复用
