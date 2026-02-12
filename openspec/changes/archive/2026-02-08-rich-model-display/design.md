## Context

当前系统已经从 `models.dev/api.json` 同步价格数据（`model.ts /update-price`），但只提取了 `cost.input` / `cost.output`，丢弃了模型的显示名称、provider 信息等元数据。前端所有页面（channels、prices、logs、dashboard）都以原始 model ID 字符串展示模型，缺乏可读性。

`models.dev/api.json` 返回格式：`Record<providerName, { name, models: Record<modelKey, { id, name, cost, ... }> }>`。Logo 通过 `https://models.dev/logos/{provider}.svg` 获取。

## Goals / Non-Goals

**Goals:**

- 后端从 models.dev 拉取模型元数据并缓存到 KV，提供 API 给前端
- 前端提供通用的模型元数据查询能力和 `ModelBadge` 展示组件
- 覆盖 channels、prices、logs、dashboard 页面的模型展示

**Non-Goals:**

- 不做模型搜索/筛选功能
- 不修改模型数据存储方式（model 字段仍为 comma-separated string）
- 不缓存到 D1 数据库，仅用 KV 缓存

## Decisions

### 1. 元数据存储方案：KV 缓存

**选择**：将 models.dev 的完整元数据处理后存入 KV（key: `model-metadata`，TTL 24h）。

**理由**：模型元数据是只读参考数据，更新频率低（每天级别），KV 的读取性能优秀且已在项目中广泛使用。不值得为此增加 D1 表。

**替代方案**：D1 新建表 → 过重，写入慢，数据本质是缓存。

### 2. 数据结构：扁平化 map

**选择**：后端将 models.dev 的嵌套结构扁平化为 `Record<modelId, ModelMeta>`，其中 `ModelMeta = { name, provider, providerName, logoUrl }`。

**理由**：前端按 model ID 查询最频繁，扁平 map 查询 O(1)。保持精简，只提取展示所需的字段。

### 3. 前端数据获取：TanStack Query + Context

**选择**：用 `useQuery` 获取元数据 map，通过自定义 hook `useModelMeta(modelId)` 提供单个模型的元数据查询。

**理由**：复用已有的 TanStack Query 基础设施，staleTime 设长（1h），避免重复请求。不需要 Context Provider，直接用 query cache 即可。

### 4. 展示组件：ModelBadge

**选择**：`ModelBadge` 组件接受 `modelId: string`，内部调用 `useModelMeta` 获取元数据，展示 provider logo（16x16 img）+ 显示名称。如果元数据中无该模型，fallback 到原始 ID。

**理由**：统一所有页面的模型展示，一个组件解决。Logo 使用 models.dev 的 SVG CDN，无需本地存储。

## Risks / Trade-offs

- **models.dev API 不可用** → 缓存有 24h TTL，且 fallback 到原始 model ID，不影响核心功能
- **新模型未收录** → ModelBadge 自动 fallback 显示原始 ID，无损降级
- **Logo 加载失败** → img onerror 隐藏图标，只显示文字
