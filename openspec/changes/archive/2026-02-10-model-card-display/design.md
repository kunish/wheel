## Context

Channels 页面使用 `<Badge variant="outline">` 包裹 `<ModelBadge>` 展示模型列表。ModelBadge 仅显示 16x16 图标 + 模型名称文本。在模型数量多时，密集排列缺乏层次感。需要创建 ModelCard 组件提供更丰富的视觉信息。

现有组件依赖链：

- `ModelBadge` → `useModelMeta(modelId)` → models.dev metadata (name, provider, providerName, logoUrl)
- Channels 页面使用场景：可拖拽模型列表 (`DraggableModel`)、Group items、Channel 表单 tags

## Goals / Non-Goals

**Goals:**

- 创建 `ModelCard` 组件，展示模型图标、显示名称、provider 名称
- 支持交互变体：可删除 (tags)、可拖拽 (model list)、只读 (group items)
- 替换 channels 页面中 Badge 形式的模型展示
- 保持与现有 `useModelMeta` hook 的复用

**Non-Goals:**

- 不修改 `ModelBadge` 组件本身（dashboard/logs 等行内场景继续使用）
- 不修改后端 API 或数据结构
- 不添加模型详情弹窗或跳转功能

## Decisions

1. **组件结构**: `ModelCard` 作为独立组件放在 `components/model-card.tsx`，内部复用 `useModelMeta` 获取元数据。支持 `onRemove` prop 控制删除按钮，`className` 用于外部样式扩展。

2. **卡片样式**: 使用 `border rounded-lg p-2` 的紧凑卡片形式，水平排列 logo(20x20) + 文本区域(name + provider)。保持紧凑以适应 `flex-wrap` 网格布局。

3. **替换范围**: 三处使用 — DraggableModel、DroppableGroup items、ModelTagInput tags。DraggableModel 保留 dnd-kit 的 ref/attributes/listeners 由外部传入。

## Risks / Trade-offs

- [增加卡片尺寸] → 模型数量多时占用更多空间。使用紧凑设计 + `max-h` 滚动容器缓解。
- [元数据加载延迟] → 已有 1h staleTime 缓存，首次加载期间降级显示 modelId 原始名称。
