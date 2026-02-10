## 1. Create grouping utility and GroupedModelList component

- [x] 1.1 Create `groupModelsByProvider(modelIds: string[], metadataMap)` utility in `apps/web/src/lib/group-models.ts` — returns `{ provider, providerName, logoUrl, models[] }[]` sorted alphabetically with "Other" last
- [x] 1.2 Create `apps/web/src/components/grouped-model-list.tsx` with props: `models: string[]`, `renderModel: (modelId: string) => ReactNode`, `className?: string`; uses `useModelMetadataQuery` + `groupModelsByProvider`; renders flat list when < 4 models, grouped with provider headers when >= 4
- [x] 1.3 Provider section header: inline label with logo (16x16, `dark:invert`), provider name, count in parentheses; muted text styling

## 2. Integrate into Channels page

- [x] 2.1 ChannelCard: replace flat `modelNames.map(m => <DraggableModelTag>)` with `<GroupedModelList models={modelNames} renderModel={m => <DraggableModelTag>} />`
- [x] 2.2 DroppableGroup items: replace flat `.map(item => <ModelCard>)` with GroupedModelList using `renderModel` that returns ModelCard with channel name / weight / priority children
- [x] 2.3 ModelTagInput tags: replace flat `tags.map(tag => <ModelCard onRemove>)` with `<GroupedModelList models={tags} renderModel={tag => <ModelCard onRemove>} />`

## 3. Verify

- [x] 3.1 Run TypeScript compile check (`pnpm --filter web exec tsc --noEmit`)
- [x] 3.2 Visual verification: provider groups display correctly with logos, names, counts; flat list for < 4 models
