## 1. Create ModelCard Component

- [x] 1.1 Create `apps/web/src/components/model-card.tsx` with props: `modelId: string`, `onRemove?: () => void`, `className?: string`
- [x] 1.2 Use `useModelMeta(modelId)` to resolve metadata; render logo (20x20, dark:invert), display name, provider name; fallback to raw modelId when metadata unavailable
- [x] 1.3 Render optional remove button (X icon) when `onRemove` is provided

## 2. Replace Badge in Channels Page

- [x] 2.1 Replace `DraggableModel` component: use ModelCard instead of `Badge > ModelBadge`, preserve dnd-kit ref/attributes/listeners
- [x] 2.2 Replace `DroppableGroup` items: use ModelCard for each group item, show channel name as prefix label
- [x] 2.3 Replace `ModelTagInput` tags: use ModelCard with `onRemove` instead of `Badge > ModelBadge > X button`

## 3. Verify

- [x] 3.1 Run TypeScript compile check (`pnpm --filter web exec tsc --noEmit`)
- [x] 3.2 Visual verification: model cards display correctly in both light and dark mode
