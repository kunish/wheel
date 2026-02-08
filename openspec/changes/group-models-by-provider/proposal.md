## Why

Model cards are currently displayed as flat lists. When a channel has 20+ models, the list is hard to scan — the user must read each card individually to find a specific provider's models. Grouping by provider (OpenAI, Anthropic, Google, etc.) makes the list instantly scannable and visually organized.

## What Changes

- Create a reusable `GroupedModelList` component that takes an array of model IDs and renders them grouped by provider using metadata from `useModelMeta`
- Models without metadata or with unknown providers are grouped under "Other"
- Each provider group shows a header with logo + provider name + model count
- Groups are collapsible for dense lists
- Replace flat model card lists with `GroupedModelList` in all locations: Channel card models, Group card items, ModelTagInput tags, and Model List dialog

## Capabilities

### New Capabilities

- `grouped-model-list`: Reusable component that groups model cards by provider with collapsible sections, provider headers with logos, and model counts

### Modified Capabilities

## Impact

- `apps/web/src/components/grouped-model-list.tsx` — new component
- `apps/web/src/app/(protected)/channels/page.tsx` — ChannelCard models, DroppableGroup items, ModelTagInput tags
- No API or backend changes required; provider info already available via `useModelMeta` hook
