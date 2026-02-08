## 1. Backend - Model Metadata API

- [x] 1.1 在 `apps/worker/src/routes/model.ts` 新增 `GET /metadata` 端点：从 KV 读取缓存，若无则 fetch `models.dev/api.json`，将嵌套结构扁平化为 `Record<modelId, { name, provider, providerName, logoUrl }>`，存入 KV（TTL 24h），返回结果
- [x] 1.2 处理 models.dev 不可用的情况：fetch 失败时返回空 map `{}`

## 2. Frontend - API & Hook

- [x] 2.1 在 `apps/web/src/lib/api.ts` 新增 `getModelMetadata()` 函数，调用 `GET /api/v1/model/metadata`
- [x] 2.2 新建 `apps/web/src/hooks/use-model-meta.ts`，导出 `useModelMetadataQuery()`（TanStack Query，staleTime 1h）和 `useModelMeta(modelId)` hook

## 3. Frontend - ModelBadge Component

- [x] 3.1 新建 `apps/web/src/components/model-badge.tsx`，接受 `modelId` prop，内部调用 `useModelMeta`，展示 provider logo（16x16）+ 显示名称，fallback 到原始 ID；logo 加载失败时隐藏

## 4. Pages Integration

- [x] 4.1 **Channels 页** (`channels/page.tsx`)：channel 卡片中的 model badges 使用 `ModelBadge`
- [x] 4.2 **Prices 页** (`prices/page.tsx`)：价格表模型名称列使用 `ModelBadge`
- [x] 4.3 **Logs 页** (`logs/page.tsx`)：日志表模型列使用 `ModelBadge`
- [x] 4.4 **Dashboard 页** (`dashboard/page.tsx`)：涉及模型名称展示的地方使用 `ModelBadge`

## 5. Verification

- [x] 5.1 `pnpm build` 通过，无 TypeScript 错误
