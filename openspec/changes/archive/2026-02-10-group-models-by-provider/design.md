## Context

Models are displayed as flat lists of `ModelCard` components in channel cards, group items, and model tag input. With 20+ models per channel, scanning is difficult. The `useModelMeta` hook already returns provider info (`provider`, `providerName`, `logoUrl`) for each model, making client-side grouping straightforward.

## Goals / Non-Goals

**Goals:**

- Group model cards by provider in all display locations
- Show provider header (logo + name + count) for each group
- Models without metadata fall into "Other" group at the end
- Keep existing drag-and-drop functionality in channels page

**Non-Goals:**

- Collapsible sections (not needed for typical 3-8 models per provider)
- Server-side grouping or new API endpoints
- Changing how models are stored or ordered

## Decisions

1. **Grouping utility function**: Create a `groupModelsByProvider` helper that takes model IDs and metadata map, returns `{ provider, providerName, logoUrl, models[] }[]` sorted by provider name with "Other" last.

2. **GroupedModelList component**: A presentational wrapper that calls the grouping utility and renders provider sections. Accepts a `renderModel` callback for custom rendering per location (drag handles, remove buttons, etc.).

3. **Provider section header**: Inline with the flow — a small label with provider logo (16px) + name + count badge, rendered before each group's model cards. No background or card wrapper to keep visual weight light.

4. **Threshold**: Only group when there are 4+ models. Below that, flat list is clearer.

## Risks / Trade-offs

- **Performance**: `groupModelsByProvider` runs on every render. For typical channel sizes (5-50 models), negligible. Uses `useMemo` to avoid re-computation.
- **Drag-and-drop**: `renderModel` callback preserves existing dnd-kit integration without GroupedModelList needing to know about drag state.
