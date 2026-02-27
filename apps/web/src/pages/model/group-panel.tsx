import type { GroupFormData, GroupItemForm } from "./group-dialog"
import type { ChannelRecord, DragDataGroup, GroupRecord, ModelPrice } from "./types"
import type { ModelMeta, ModelProfile } from "@/lib/api-client"
import { useDroppable } from "@dnd-kit/core"
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import {
  ChevronDown,
  ChevronsDownUp,
  ChevronsUpDown,
  GripVertical,
  Pencil,
  Plus,
  Trash2,
  X,
} from "lucide-react"
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { ConfirmDeleteDialog } from "@/components/confirm-delete-dialog"
import { ModelCard } from "@/components/model-card"
import { ModelSourceBadge } from "@/components/model-source-badge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { fuzzyLookup, useModelMetadataQuery } from "@/hooks/use-model-meta"
import { cn } from "@/lib/utils"
import { EMPTY_GROUP_FORM } from "./group-dialog"
import { useGroupMutations } from "./use-group-mutations"

const GroupDialog = lazy(() => import("./group-dialog"))

// ─── Props ─────────────────────────────────────

export interface GroupPanelProps {
  priceMap: Map<string, ModelPrice>
  channels: ChannelRecord[]
  profileId: number | undefined
  profileList: ModelProfile[]
  groups: GroupRecord[]
  groupsLoading: boolean
  activeDrag: { type: string } | null
  hoverGroupId: number | null
  onProfileChange: (id: number) => void
}

// ─── GroupPanel ────────────────────────────────

export function GroupPanel({
  priceMap,
  channels,
  profileId,
  profileList,
  groups,
  groupsLoading,
  activeDrag,
  hoverGroupId,
  onProfileChange,
}: GroupPanelProps) {
  const { t } = useTranslation("model")

  // Group state
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const [groupForm, setGroupForm] = useState<GroupFormData>(EMPTY_GROUP_FORM)
  const [groupsCollapsed, setGroupsCollapsed] = useState(true)

  // Delete confirmation state
  const [deleteGroupConfirm, setDeleteGroupConfirm] = useState<GroupRecord | null>(null)
  const [clearGroupConfirm, setClearGroupConfirm] = useState<GroupRecord | null>(null)

  // Mutations
  const { groupDeleteMut, groupSaveMut, groupRemoveItemMut, groupClearMut } = useGroupMutations({
    activeProfileId: profileId,
    onSaveSuccess: () => setGroupDialogOpen(false),
    groupForm,
  })

  const channelMap = useMemo(() => new Map(channels.map((ch) => [ch.id, ch])), [channels])
  const groupIds = useMemo(() => groups.map((g) => `sortable-group-${g.id}`), [groups])

  // ─── Group helpers ─────────────────────────────

  function openCreateGroup() {
    setGroupForm(EMPTY_GROUP_FORM)
    setGroupDialogOpen(true)
  }

  function openEditGroup(g: GroupRecord) {
    setGroupForm({
      id: g.id,
      name: g.name,
      mode: g.mode,
      firstTokenTimeOut: g.firstTokenTimeOut,
      sessionKeepTime: g.sessionKeepTime ?? 0,
      items: (g.items ?? []).map((it) => ({ ...it, enabled: it.enabled ?? true })),
    })
    setGroupDialogOpen(true)
  }

  // ─── Render ────────────────────────────────────

  return (
    <>
      <div className="flex min-h-0 flex-col gap-3">
        <div className="flex shrink-0 items-center justify-between">
          <div className="flex items-center gap-2">
            <h3 className="text-lg font-semibold">{t("groups")}</h3>
            {profileList.length > 0 && (
              <Select
                value={profileId !== undefined ? String(profileId) : undefined}
                onValueChange={(v) => onProfileChange(Number(v))}
              >
                <SelectTrigger className="h-8 w-40 text-xs">
                  <SelectValue placeholder={t("profile.switcherLabel")} />
                </SelectTrigger>
                <SelectContent>
                  {profileList.map((p) => (
                    <SelectItem key={p.id} value={String(p.id)}>
                      {p.name} ({p.groupCount ?? 0})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
          <div className="flex items-center gap-1">
            {groups.length > 0 && (
              <Button
                variant="ghost"
                size="icon"
                className="h-9 w-9"
                onClick={() => setGroupsCollapsed((v) => !v)}
                title={groupsCollapsed ? t("expandAll") : t("collapseAll")}
                aria-label={groupsCollapsed ? t("expandAll") : t("collapseAll")}
              >
                {groupsCollapsed ? (
                  <ChevronsUpDown className="h-4 w-4" />
                ) : (
                  <ChevronsDownUp className="h-4 w-4" />
                )}
              </Button>
            )}
            <Button size="sm" onClick={openCreateGroup}>
              <Plus className="mr-1 h-3 w-3" /> {t("actions.add", { ns: "common" })}
            </Button>
          </div>
        </div>

        <ScrollArea className="min-h-0 flex-1">
          {groupsLoading || profileId === undefined ? (
            <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
          ) : groups.length === 0 ? (
            <p className="text-muted-foreground text-sm">{t("emptyGroups")}</p>
          ) : (
            <SortableContext items={groupIds} strategy={verticalListSortingStrategy}>
              <div className="flex flex-col gap-3">
                {groups.map((g) => (
                  <SortableGroup
                    key={g.id}
                    group={g}
                    channelMap={channelMap}
                    onEdit={() => openEditGroup(g)}
                    onDelete={() => setDeleteGroupConfirm(g)}
                    onClear={() => setClearGroupConfirm(g)}
                    onRemoveItem={(itemIndex) => groupRemoveItemMut.mutate({ group: g, itemIndex })}
                    isOver={activeDrag !== null}
                    hoverGroupId={hoverGroupId}
                    forceCollapsed={groupsCollapsed}
                    priceMap={priceMap}
                  />
                ))}
              </div>
            </SortableContext>
          )}
        </ScrollArea>
      </div>

      {/* ─── Delete Confirmations ────────────────── */}
      <ConfirmDeleteDialog
        open={!!deleteGroupConfirm}
        onOpenChange={(open) => !open && setDeleteGroupConfirm(null)}
        title={t("deleteGroupTitle", { name: deleteGroupConfirm?.name })}
        description={t("deleteGroupDesc")}
        cancelLabel={t("actions.cancel", { ns: "common" })}
        confirmLabel={t("actions.delete", { ns: "common" })}
        onConfirm={() => {
          if (deleteGroupConfirm) groupDeleteMut.mutate(deleteGroupConfirm.id)
          setDeleteGroupConfirm(null)
        }}
      />

      <ConfirmDeleteDialog
        open={!!clearGroupConfirm}
        onOpenChange={(open) => !open && setClearGroupConfirm(null)}
        title={t("clearGroupTitle", { name: clearGroupConfirm?.name })}
        description={t("clearGroupDesc")}
        cancelLabel={t("actions.cancel", { ns: "common" })}
        confirmLabel={t("clearAllAction")}
        onConfirm={() => {
          if (clearGroupConfirm) groupClearMut.mutate(clearGroupConfirm)
          setClearGroupConfirm(null)
        }}
      />

      {/* ─── Group Dialog ─────────────────────── */}
      <Suspense fallback={null}>
        <GroupDialog
          open={groupDialogOpen}
          onOpenChange={(open) => {
            setGroupDialogOpen(open)
            if (!open) setTimeout(() => setGroupForm(EMPTY_GROUP_FORM), 200)
          }}
          form={groupForm}
          setForm={setGroupForm}
          channelOptions={channels}
          onSave={() => groupSaveMut.mutate(groupForm)}
          isPending={groupSaveMut.isPending}
        />
      </Suspense>
    </>
  )
}

// ─── Sortable & Droppable Group Card ──────────

function SortableGroup({
  group,
  channelMap,
  onEdit,
  onDelete,
  onClear,
  onRemoveItem,
  isOver: dragActive,
  hoverGroupId,
  forceCollapsed,
  priceMap,
}: {
  group: GroupRecord
  channelMap: Map<number, ChannelRecord>
  onEdit: () => void
  onDelete: () => void
  onClear: () => void
  onRemoveItem: (itemIndex: number) => void
  isOver: boolean
  hoverGroupId: number | null
  forceCollapsed?: boolean
  priceMap: Map<string, ModelPrice>
}) {
  const { t } = useTranslation("model")
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: `sortable-group-${group.id}`,
    data: { type: "group", groupId: group.id } satisfies DragDataGroup,
  })

  const { setNodeRef: dropRef, isOver } = useDroppable({
    id: `group-${group.id}`,
    data: { groupId: group.id },
  })

  const { data: metaData } = useModelMetadataQuery()
  const [collapsed, setCollapsed] = useState(true)
  const expandedByDragRef = useRef(false)
  const hoverTimerRef = useRef<ReturnType<typeof setTimeout>>(null)

  // Parent-driven hover detection (more reliable than useDroppable isOver
  // because sortable collision detection can shadow the droppable)
  const isHovered = hoverGroupId === group.id

  // Sync with parent's expand/collapse all toggle
  const prevForceCollapsedRef = useRef(forceCollapsed)
  if (prevForceCollapsedRef.current !== forceCollapsed) {
    prevForceCollapsedRef.current = forceCollapsed
    if (forceCollapsed !== undefined) setCollapsed(forceCollapsed)
  }

  // Auto-expand when dragging over a collapsed group
  useEffect(() => {
    if (isHovered && collapsed && dragActive) {
      hoverTimerRef.current = setTimeout(() => {
        setCollapsed(false)
        expandedByDragRef.current = true
      }, 500)
    } else if (!isHovered) {
      if (hoverTimerRef.current) {
        clearTimeout(hoverTimerRef.current)
        hoverTimerRef.current = null
      }
    }
    return () => {
      if (hoverTimerRef.current) clearTimeout(hoverTimerRef.current)
    }
  }, [isHovered, collapsed, dragActive])

  // Restore collapsed state when drag ends (if we auto-expanded it)
  useEffect(() => {
    if (!dragActive && expandedByDragRef.current) {
      expandedByDragRef.current = false
      // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect -- intentional: restore UI state after drag ends
      setCollapsed(true)
    }
  }, [dragActive])

  const style = {
    transform: CSS.Translate.toString(transform),
    transition,
  }

  return (
    <Card
      ref={(node) => {
        setNodeRef(node)
        dropRef(node)
      }}
      style={style}
      className={cn(
        "relative mx-[1px] gap-0 overflow-hidden shadow-sm transition-shadow hover:shadow-md",
        (isOver || isHovered) && "border-primary border-2",
        dragActive && !isOver && !isHovered && "border-dashed",
        isDragging && "opacity-30",
      )}
    >
      <div className="absolute top-0 bottom-0 left-0 w-1.5 bg-gradient-to-b from-purple-400 to-indigo-400" />
      <CardHeader
        className={cn(
          "flex flex-row items-center justify-between space-y-0 px-3",
          collapsed ? "py-2.5" : "pt-2.5 pb-1",
        )}
      >
        <div className="flex items-center gap-2 pl-1.5">
          <button
            {...attributes}
            {...listeners}
            className="text-muted-foreground hover:text-foreground hover:bg-accent cursor-grab touch-none rounded p-1"
          >
            <GripVertical className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => setCollapsed(!collapsed)}
            className="flex items-center gap-1.5 text-left"
          >
            <ChevronDown
              className={cn(
                "text-muted-foreground h-4 w-4 shrink-0 transition-transform",
                collapsed && "-rotate-90",
              )}
            />
            <div>
              <CardTitle className="text-sm font-semibold">{group.name}</CardTitle>
              <div className="mt-1 flex items-center gap-1.5">
                <Badge
                  variant={
                    group.mode === 1
                      ? "lime"
                      : group.mode === 2
                        ? "sky"
                        : group.mode === 3
                          ? "orange"
                          : group.mode === 4
                            ? "pink"
                            : "secondary"
                  }
                  className="rounded-full px-2 py-0 text-[10px] font-normal"
                >
                  {t(`modeLabels.${group.mode}`, { defaultValue: t("unknown") })}
                </Badge>
                {collapsed && group.items.length > 0 && (
                  <span className="bg-secondary/30 text-muted-foreground rounded-full px-2 py-0 text-[10px]">
                    {t("itemCount", { count: group.items.length })}
                  </span>
                )}
              </div>
            </div>
          </button>
        </div>
        <div className="flex items-center gap-1 pr-1">
          {group.items.length > 0 && (
            <Button
              variant="ghost"
              size="icon"
              className="text-muted-foreground h-8 w-8"
              onClick={onClear}
              title={t("clearAll")}
              aria-label={t("clearAll")}
            >
              <X className="h-4 w-4" />
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="text-muted-foreground h-8 w-8"
            aria-label="Edit group"
            onClick={onEdit}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-muted-foreground hover:text-destructive h-8 w-8"
            aria-label="Delete group"
            onClick={onDelete}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </CardHeader>
      <div
        className={cn(
          "grid transition-[grid-template-rows] duration-250",
          collapsed ? "grid-rows-[0fr]" : "grid-rows-[1fr]",
        )}
      >
        <div className="overflow-hidden">
          <CardContent className="px-3 pt-0 pb-2.5">
            <div className="text-muted-foreground mb-1 text-xs">
              {t("timeout", { seconds: group.firstTokenTimeOut })}
            </div>
            {group.items.length === 0 ? (
              <div
                className={cn(
                  "text-muted-foreground rounded-md border border-dashed p-4 text-center text-xs",
                  isOver && "bg-primary/5 border-primary",
                )}
              >
                {dragActive ? t("dropHere") : t("dragHint")}
              </div>
            ) : (
              <GroupItemList
                items={group.items}
                mode={group.mode}
                channelMap={channelMap}
                metadataMap={metaData?.data}
                priceMap={priceMap}
                onRemoveItem={onRemoveItem}
              />
            )}
          </CardContent>
        </div>
      </div>
    </Card>
  )
}

// ─── Group Item List (grouped by provider) ────

function GroupItemList({
  items,
  mode,
  channelMap,
  metadataMap,
  priceMap,
  onRemoveItem,
}: {
  items: GroupItemForm[]
  mode: number
  channelMap: Map<number, ChannelRecord>
  metadataMap: Record<string, ModelMeta> | undefined
  priceMap: Map<string, ModelPrice>
  onRemoveItem?: (itemIndex: number) => void
}) {
  // Separate model items from "all" items
  const modelItems = items.filter((it) => it.modelName)
  const allItems = items.filter((it) => !it.modelName)
  const modelIds = modelItems.map((it) => it.modelName)

  const itemIndexMap = useMemo(() => {
    const map = new Map<GroupItemForm, number>()
    for (const [i, it] of items.entries()) {
      map.set(it, i)
    }
    return map
  }, [items])

  // Build resolved metadata map using fuzzy matching
  const resolvedMap = useMemo(() => {
    if (!metadataMap) return undefined
    const map: Record<string, ModelMeta> = {}
    for (const id of modelIds) {
      const meta = fuzzyLookup(metadataMap, id)
      if (meta) map[id] = meta
    }
    return map
  }, [metadataMap, modelIds])

  // Group items by provider, preserving full item references to handle
  // duplicate model names across different channels correctly.
  const groupedItems = useMemo(() => {
    interface ItemGroup {
      provider: string
      providerName: string
      logoUrl: string | null
      items: GroupItemForm[]
    }
    if (!resolvedMap) {
      return [
        { provider: "other", providerName: "Other", logoUrl: null, items: modelItems },
      ] as ItemGroup[]
    }
    const groups = new Map<string, ItemGroup>()
    for (const it of modelItems) {
      const meta = resolvedMap[it.modelName]
      const key = meta?.provider ?? "other"
      let group = groups.get(key)
      if (!group) {
        group = {
          provider: key,
          providerName: meta?.providerName ?? "Other",
          logoUrl: meta?.logoUrl ?? null,
          items: [],
        }
        groups.set(key, group)
      }
      group.items.push(it)
    }
    return Array.from(groups.values()).sort((a, b) => {
      if (a.provider === "other") return 1
      if (b.provider === "other") return -1
      return a.providerName.localeCompare(b.providerName)
    }) as ItemGroup[]
  }, [modelItems, resolvedMap])

  const shouldGroup = modelIds.length >= 4

  const renderModelCard = (item: GroupItemForm, idx: number) => {
    const ch = channelMap.get(item.channelId)
    const isDisabled = ch?.enabled === false
    const price = priceMap.get(item.modelName)
    const originalIndex = itemIndexMap.get(item)
    const fetchedModels = ch?.fetchedModel ?? []
    const isApiFetched = fetchedModels.includes(item.modelName)
    return (
      <ModelCard
        key={`${item.channelId}-${item.modelName}-${idx}`}
        modelId={item.modelName}
        disabled={isDisabled}
        price={price ? { inputPrice: price.inputPrice, outputPrice: price.outputPrice } : undefined}
        onRemove={
          onRemoveItem && originalIndex !== undefined
            ? () => onRemoveItem(originalIndex)
            : undefined
        }
      >
        <ModelSourceBadge modelId={item.modelName} isApiFetched={isApiFetched} />
        <span className="text-muted-foreground text-[10px]">
          {ch?.name ?? `#${item.channelId}`}
        </span>
        {mode === 4 && <span className="text-muted-foreground text-[10px]">w:{item.weight}</span>}
        {mode === 3 && <span className="text-muted-foreground text-[10px]">p:{item.priority}</span>}
      </ModelCard>
    )
  }

  const renderAllBadge = (item: GroupItemForm, i: number) => {
    const ch = channelMap.get(item.channelId)
    const isDisabled = ch?.enabled === false
    return (
      <Badge
        key={`${item.channelId}-all-${i}`}
        variant="outline"
        className={cn("text-xs", isDisabled && "opacity-40")}
      >
        {ch?.name ?? `#${item.channelId}`}: all
        {mode === 4 && <span className="text-muted-foreground ml-1">w:{item.weight}</span>}
        {mode === 3 && <span className="text-muted-foreground ml-1">p:{item.priority}</span>}
      </Badge>
    )
  }

  if (!shouldGroup) {
    return (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-1.5">
        {modelItems.map((it, i) => renderModelCard(it, i))}
        {allItems.map((it, i) => renderAllBadge(it, i))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      {groupedItems.map((g) => (
        <div key={g.provider}>
          <ProviderHeader logoUrl={g.logoUrl} name={g.providerName} count={g.items.length} />
          <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-1.5">
            {g.items.map((it, i) => renderModelCard(it, i))}
          </div>
        </div>
      ))}
      {allItems.length > 0 && (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-1.5">
          {allItems.map((it, i) => renderAllBadge(it, i))}
        </div>
      )}
    </div>
  )
}

// ─── Provider Header (shared) ──────────────────

function ProviderHeader({
  logoUrl,
  name,
  count,
}: {
  logoUrl: string | null
  name: string
  count: number
}) {
  const [logoError, setLogoError] = useState(false)
  const showLogo = logoUrl && !logoError
  return (
    <div className="mb-1 flex items-center gap-1.5">
      {showLogo && (
        <img
          src={logoUrl}
          alt={name}
          width={16}
          height={16}
          className="shrink-0 dark:invert"
          onError={() => setLogoError(true)}
        />
      )}
      <span className="text-muted-foreground text-xs">
        {name} ({count})
      </span>
    </div>
  )
}
