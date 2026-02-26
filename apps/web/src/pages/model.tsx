import type { DragEndEvent, DragOverEvent, DragStartEvent } from "@dnd-kit/core"
import type { ChannelFormData } from "./model/channel-dialog"
import type { GroupFormData, GroupItemForm } from "./model/group-dialog"
import type { PriceFormData } from "./model/price-dialog"
import type { ModelMeta, ModelProfile } from "@/lib/api-client"
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  TouchSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core"
import {
  arrayMove,
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ChevronDown,
  ChevronsDownUp,
  ChevronsUpDown,
  GitBranch,
  GripVertical,
  Layers,
  List,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  X,
} from "lucide-react"
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"
import { toast } from "sonner"
import { ConfirmDeleteDialog } from "@/components/confirm-delete-dialog"
import { GroupedModelList } from "@/components/grouped-model-list"
import { ModelCard } from "@/components/model-card"
import { ModelSourceBadge } from "@/components/model-source-badge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { fuzzyLookup, useModelMetadataQuery } from "@/hooks/use-model-meta"
import { useProfilesQuery } from "@/hooks/use-profiles"
import {
  getChannelHealth,
  getLastPriceUpdateTime,
  getModelList,
  getSettings,
  listChannels,
  listGroups,
  listModelPrices,
} from "@/lib/api-client"
import { cn } from "@/lib/utils"
import { EMPTY_CHANNEL_FORM } from "./model/channel-dialog"
import { EMPTY_GROUP_FORM } from "./model/group-dialog"
import { EMPTY_PRICE_FORM } from "./model/price-dialog"
import { useChannelMutations } from "./model/use-channel-mutations"
import { useGroupMutations } from "./model/use-group-mutations"
import { usePriceMutations } from "./model/use-price-mutations"
import { useProfileMutations } from "./model/use-profile-mutations"

// ─── Lazy-loaded Dialog components ──────────────

const ChannelDialog = lazy(() => import("./model/channel-dialog"))

const GroupDialog = lazy(() => import("./model/group-dialog"))

const PriceDialog = lazy(() => import("./model/price-dialog"))

const ProfileManageDialog = lazy(() => import("./model/profile-manage-dialog"))

const RoutingRulesSheet = lazy(() => import("./model/routing-rules-sheet"))

// ─── Types ─────────────────────────────────────

interface ModelPrice {
  id: number
  name: string
  inputPrice: number
  outputPrice: number
  source: string
  createdAt: string
  updatedAt: string
}

interface ChannelRecord {
  id: number
  name: string
  type: number
  enabled: boolean
  model: string[]
  fetchedModel: string[]
  customModel: string
  baseUrls: { url: string; delay: number }[]
  keys: { channelKey: string; remark: string }[]
  paramOverride: string | null
}

interface GroupRecord {
  id: number
  name: string
  mode: number
  firstTokenTimeOut: number
  sessionKeepTime: number
  order: number
  items: GroupItemForm[]
}

// ─── Drag data types ───────────────────────────

interface DragDataModel {
  type: "model"
  model: string
  channelId: number
  channelName: string
}

interface DragDataChannel {
  type: "channel"
  channel: ChannelRecord
}

interface DragDataGroup {
  type: "group"
  groupId: number
}

type DragData = DragDataModel | DragDataChannel | DragDataGroup

// ─── Main page ─────────────────────────────────

export default function ModelPage() {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  // Highlight support: read ?highlight=<channelId> from URL
  const highlightChannelId = searchParams.get("highlight")
    ? Number(searchParams.get("highlight"))
    : null
  const [highlightedId, setHighlightedId] = useState<number | null>(null)
  const channelRefsRef = useRef<Map<number, HTMLDivElement>>(new Map())

  // Channel state
  const [channelDialogOpen, setChannelDialogOpen] = useState(false)
  const [channelForm, setChannelForm] = useState<ChannelFormData>(EMPTY_CHANNEL_FORM)
  const [channelsCollapsed, setChannelsCollapsed] = useState(true)

  // Group state
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const [groupForm, setGroupForm] = useState<GroupFormData>(EMPTY_GROUP_FORM)
  const [modelListOpen, setModelListOpen] = useState(false)
  const [groupsCollapsed, setGroupsCollapsed] = useState(true)

  // Drag state
  const [activeDrag, setActiveDrag] = useState<DragData | null>(null)
  const [hoverGroupId, setHoverGroupId] = useState<number | null>(null)

  // Delete confirmation state
  const [deleteChannelConfirm, setDeleteChannelConfirm] = useState<ChannelRecord | null>(null)
  const [deleteGroupConfirm, setDeleteGroupConfirm] = useState<GroupRecord | null>(null)
  const [clearGroupConfirm, setClearGroupConfirm] = useState<GroupRecord | null>(null)

  // Price state
  const [priceDialogOpen, setPriceDialogOpen] = useState(false)
  const [priceForm, setPriceForm] = useState<PriceFormData>(EMPTY_PRICE_FORM)
  const [editingPriceId, setEditingPriceId] = useState<number | null>(null)
  const [deletePriceConfirm, setDeletePriceConfirm] = useState<ModelPrice | null>(null)

  // Profile state
  const [profileDialogOpen, setProfileDialogOpen] = useState(false)

  // Routing rules state
  const [routingRulesOpen, setRoutingRulesOpen] = useState(false)

  // Read active_profile_id from backend settings
  const { data: settingsData } = useQuery({
    queryKey: ["settings"],
    queryFn: getSettings,
  })
  const activeProfileId = useMemo(() => {
    const raw = settingsData?.data?.settings?.active_profile_id
    if (raw === undefined || raw === "0") return undefined
    const n = Number(raw)
    return Number.isNaN(n) || n === 0 ? undefined : n
  }, [settingsData])

  // Queries
  const { data: channelData, isLoading: channelsLoading } = useQuery({
    queryKey: ["channels"],
    queryFn: listChannels,
  })

  const { data: groupData, isLoading: groupsLoading } = useQuery({
    queryKey: ["groups", activeProfileId],
    queryFn: () => listGroups(activeProfileId),
    enabled: activeProfileId !== undefined,
  })

  const { data: modelData } = useQuery({
    queryKey: ["model-list"],
    queryFn: getModelList,
    enabled: modelListOpen,
    staleTime: 60_000,
  })

  // Price queries
  const { data: priceData } = useQuery({
    queryKey: ["model-prices"],
    queryFn: listModelPrices,
    staleTime: 60_000,
  })

  const { data: updateTimeData } = useQuery({
    queryKey: ["price-update-time"],
    queryFn: getLastPriceUpdateTime,
  })

  const { data: healthData } = useQuery({
    queryKey: ["channel-health"],
    queryFn: getChannelHealth,
    refetchInterval: 30_000,
  })

  // ─── Extracted mutation hooks ──────────────────

  const { channelDeleteMut, channelEnableMut, channelSaveMut, channelReorderMut } =
    useChannelMutations({
      onSaveSuccess: () => setChannelDialogOpen(false),
      channelForm,
    })

  const {
    groupDeleteMut,
    groupSaveMut,
    groupAddItemMut,
    groupClearMut,
    groupRemoveItemMut,
    groupReorderMut,
    invalidateGroupQueries,
  } = useGroupMutations({
    activeProfileId,
    onSaveSuccess: () => setGroupDialogOpen(false),
    groupForm,
  })

  const { setActiveProfileId } = useProfileMutations({
    onActivateSuccess: () => invalidateGroupQueries(),
  })

  const { syncPriceMut, createPriceMut, updatePriceMut, deletePriceMut } = usePriceMutations({
    onCreateSuccess: () => {
      setPriceForm(EMPTY_PRICE_FORM)
      setPriceDialogOpen(false)
    },
    onUpdateSuccess: () => {
      setPriceDialogOpen(false)
      setEditingPriceId(null)
    },
  })

  // ─── Derived data ─────────────────────────────

  // Profile query
  const { data: profileData } = useProfilesQuery()
  const profileList = useMemo(() => {
    const raw = (profileData?.data?.profiles ?? []) as ModelProfile[]
    const defaultIdx = raw.findIndex((p) => p.isBuiltin && p.name === "Default")
    if (defaultIdx > 0) {
      const copy = [...raw]
      const [def] = copy.splice(defaultIdx, 1)
      copy.unshift(def)
      return copy
    }
    return raw
  }, [profileData])
  useEffect(() => {
    if (profileList.length === 0 || !settingsData) return
    if (activeProfileId === undefined || !profileList.some((p) => p.id === activeProfileId)) {
      setActiveProfileId(profileList[0].id)
    }
  }, [profileList, activeProfileId, settingsData, setActiveProfileId])

  const channels = useMemo(
    () => (channelData?.data?.channels ?? []) as ChannelRecord[],
    [channelData],
  )
  const groups = useMemo(() => (groupData?.data?.groups ?? []) as GroupRecord[], [groupData])
  const models = useMemo(() => (modelData?.data?.models ?? []) as string[], [modelData])
  const priceList = useMemo(() => (priceData?.data?.models ?? []) as ModelPrice[], [priceData])
  const priceMap = useMemo(() => {
    const map = new Map<string, ModelPrice>()
    for (const p of priceList) map.set(p.name, p)
    return map
  }, [priceList])

  const healthMap = useMemo<Record<string, number>>(
    () => healthData?.data?.health ?? {},
    [healthData],
  )

  function formatDateTime(dateStr: string | undefined): string | null {
    if (!dateStr) return null
    return new Date(dateStr).toLocaleString("en-US", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    })
  }

  const lastSync = formatDateTime(updateTimeData?.data?.lastUpdateTime ?? undefined)

  // ─── Highlight scroll logic ───────────────────
  const setChannelRef = useCallback((id: number, el: HTMLDivElement | null) => {
    if (el) channelRefsRef.current.set(id, el)
    else channelRefsRef.current.delete(id)
  }, [])

  useEffect(() => {
    if (!highlightChannelId || channelsLoading || channels.length === 0) return
    // Verify channel exists
    if (!channels.some((ch) => ch.id === highlightChannelId)) return

    // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect -- intentional: sync highlight from URL param
    setHighlightedId(highlightChannelId)
    requestAnimationFrame(() => {
      const el = channelRefsRef.current.get(highlightChannelId)
      if (el) {
        el.scrollIntoView({ behavior: "smooth", block: "center" })
      }
    })

    // Clear highlight after 3s and remove query param
    const timer = setTimeout(() => {
      setHighlightedId(null)
      const params = new URLSearchParams(searchParams.toString())
      params.delete("highlight")
      const query = params.toString()
      setSearchParams(query ? `?${query}` : "", { replace: true })
    }, 3000)
    return () => clearTimeout(timer)
  }, [highlightChannelId, channelsLoading, channels, searchParams, setSearchParams])

  // ─── Drag handlers ─────────────────────────────

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 1 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
  )

  function handleDragStart(event: DragStartEvent) {
    setActiveDrag(event.active.data.current as DragData)
    setHoverGroupId(null)
  }

  function handleDragOver(event: DragOverEvent) {
    const { over } = event
    if (!over) {
      setHoverGroupId(null)
      return
    }
    // Match both "group-123" (droppable) and "sortable-group-123" (sortable) ids
    const overId = String(over.id)
    let groupId: number | null = null
    if (overId.startsWith("group-")) {
      groupId = Number(overId.replace("group-", ""))
    } else if (overId.startsWith("sortable-group-")) {
      groupId = Number(overId.replace("sortable-group-", ""))
    }
    setHoverGroupId(groupId)
  }

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event
    setActiveDrag(null)
    setHoverGroupId(null)

    if (!over) return

    const dragData = active.data.current as DragData

    // ─── Group reorder ─────────────────────────
    if (dragData.type === "group") {
      const activeId = String(active.id)
      const overId = String(over.id)
      if (activeId === overId) return

      if (!activeId.startsWith("sortable-group-") || !overId.startsWith("sortable-group-")) return

      const oldIndex = groups.findIndex((g) => `sortable-group-${g.id}` === activeId)
      const newIndex = groups.findIndex((g) => `sortable-group-${g.id}` === overId)
      if (oldIndex === -1 || newIndex === -1 || oldIndex === newIndex) return

      const reordered = arrayMove(groups, oldIndex, newIndex)
      // Optimistic update — prevent snap-back before API responds
      const groupQueryKey = ["groups", activeProfileId]
      queryClient.setQueryData(groupQueryKey, (old: any) =>
        old ? { ...old, data: { ...old.data, groups: reordered } } : old,
      )
      groupReorderMut.mutate(reordered.map((g) => g.id))
      return
    }

    // ─── Channel reorder ──────────────────────
    if (dragData.type === "channel") {
      const activeId = String(active.id)
      const overId = String(over.id)

      if (activeId.startsWith("sortable-channel-") && overId.startsWith("sortable-channel-")) {
        if (activeId === overId) return
        const oldIndex = channels.findIndex((ch) => `sortable-channel-${ch.id}` === activeId)
        const newIndex = channels.findIndex((ch) => `sortable-channel-${ch.id}` === overId)
        if (oldIndex === -1 || newIndex === -1 || oldIndex === newIndex) return

        const reordered = arrayMove(channels, oldIndex, newIndex)
        // Optimistic update — prevent snap-back before API responds
        queryClient.setQueryData(["channels"], (old: any) =>
          old ? { ...old, data: { ...old.data, channels: reordered } } : old,
        )
        channelReorderMut.mutate(reordered.map((ch) => ch.id))
        return
      }
    }

    // ─── Cross-area drop (channel/model → group) ─
    const dropData = over.data.current as { groupId: number } | undefined
    if (!dropData?.groupId) return

    const targetGroup = groups.find((g) => g.id === dropData.groupId)
    if (!targetGroup) return

    let newItems: GroupItemForm[] = []

    if (dragData.type === "model") {
      // Check for duplicates
      const exists = targetGroup.items.some(
        (it) => it.channelId === dragData.channelId && it.modelName === dragData.model,
      )
      if (exists) {
        toast.info(t("toast.modelAlreadyInGroup"))
        return
      }
      newItems = [
        {
          channelId: dragData.channelId,
          modelName: dragData.model,
          priority: 0,
          weight: 1,
          enabled: true,
        },
      ]
    } else if (dragData.type === "channel") {
      const ch = dragData.channel
      const modelNames = parseModels(ch.model)
      if (modelNames.length === 0) {
        toast.error(t("toast.channelNoModels"))
        return
      }
      // Filter out duplicates
      newItems = modelNames
        .filter(
          (m) => !targetGroup.items.some((it) => it.channelId === ch.id && it.modelName === m),
        )
        .map((m) => ({
          channelId: ch.id,
          modelName: m,
          priority: 0,
          weight: 1,
          enabled: true,
        }))
      if (newItems.length === 0) {
        toast.info(t("toast.allModelsAlreadyInGroup"))
        return
      }
    }

    groupAddItemMut.mutate({ group: targetGroup, newItems })
  }

  // ─── Channel helpers ───────────────────────────

  function openCreateChannel() {
    setChannelForm(EMPTY_CHANNEL_FORM)
    setChannelDialogOpen(true)
  }

  function openEditChannel(ch: ChannelRecord) {
    setChannelForm({
      id: ch.id,
      name: ch.name,
      type: ch.type,
      enabled: ch.enabled,
      baseUrls: ch.baseUrls?.length ? ch.baseUrls : [{ url: "", delay: 0 }],
      keys: ch.keys?.length ? ch.keys : [{ channelKey: "", remark: "" }],
      model: ch.model ?? [],
      fetchedModel: ch.fetchedModel ?? [],
      customModel: ch.customModel ?? "",
      paramOverride: ch.paramOverride ?? "",
    })
    setChannelDialogOpen(true)
  }

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

  const channelMap = useMemo(() => new Map(channels.map((ch) => [ch.id, ch])), [channels])
  const groupIds = useMemo(() => groups.map((g) => `sortable-group-${g.id}`), [groups])
  const channelIds = useMemo(() => channels.map((ch) => `sortable-channel-${ch.id}`), [channels])
  const renderSortableGroups = (
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
  )

  // ─── Render ────────────────────────────────────

  return (
    <DndContext
      sensors={sensors}
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
      onDragEnd={handleDragEnd}
    >
      <div className="flex min-h-0 flex-1 flex-col gap-4">
        <div className="flex shrink-0 items-center justify-between">
          <h2 className="text-2xl font-bold tracking-tight">{t("pageTitle")}</h2>
          <div className="flex items-center gap-2">
            {lastSync && (
              <span className="text-muted-foreground hidden text-xs sm:inline">
                {t("price.lastSynced", { time: lastSync })}
              </span>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => syncPriceMut.mutate()}
              disabled={syncPriceMut.isPending}
            >
              <RefreshCw
                className={`mr-2 h-4 w-4 ${syncPriceMut.isPending ? "animate-spin" : ""}`}
              />
              {t("price.syncPrices")}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setModelListOpen(true)}>
              <List className="mr-2 h-4 w-4" /> {t("models")}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setProfileDialogOpen(true)}>
              <Layers className="mr-2 h-4 w-4" /> {t("profile.title")}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setRoutingRulesOpen(true)}>
              <GitBranch className="mr-2 h-4 w-4" /> {t("routingRules.title")}
            </Button>
          </div>
        </div>

        <div className="grid min-h-0 flex-1 grid-cols-1 gap-6 lg:grid-cols-2">
          {/* ─── Left: Channels ───────────────── */}
          <div className="flex min-h-0 flex-col gap-3">
            <div className="flex shrink-0 items-center justify-between">
              <h3 className="text-lg font-semibold">{t("channels")}</h3>
              <div className="flex items-center gap-1">
                {channels.length > 0 && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-9 w-9"
                    onClick={() => setChannelsCollapsed((v) => !v)}
                    title={channelsCollapsed ? t("expandAll") : t("collapseAll")}
                    aria-label={channelsCollapsed ? t("expandAll") : t("collapseAll")}
                  >
                    {channelsCollapsed ? (
                      <ChevronsUpDown className="h-4 w-4" />
                    ) : (
                      <ChevronsDownUp className="h-4 w-4" />
                    )}
                  </Button>
                )}
                <Button size="sm" onClick={openCreateChannel}>
                  <Plus className="mr-1 h-3 w-3" /> {t("actions.add", { ns: "common" })}
                </Button>
              </div>
            </div>

            <ScrollArea className="min-h-0 flex-1">
              {channelsLoading ? (
                <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
              ) : channels.length === 0 ? (
                <p className="text-muted-foreground text-sm">{t("emptyChannels")}</p>
              ) : (
                <SortableContext items={channelIds} strategy={verticalListSortingStrategy}>
                  <div className="flex flex-col gap-3">
                    {channels.map((ch) => (
                      <DraggableChannel
                        key={ch.id}
                        channel={ch}
                        highlighted={highlightedId === ch.id}
                        refCallback={(el) => setChannelRef(ch.id, el)}
                        onEdit={() => openEditChannel(ch)}
                        onDelete={() => setDeleteChannelConfirm(ch)}
                        onToggle={(enabled) => channelEnableMut.mutate({ id: ch.id, enabled })}
                        enablePending={channelEnableMut.isPending}
                        forceCollapsed={channelsCollapsed}
                        priceMap={priceMap}
                        healthStatus={healthMap[String(ch.id)]}
                      />
                    ))}
                  </div>
                </SortableContext>
              )}
            </ScrollArea>
          </div>

          {/* ─── Right: Groups ────────────────── */}
          <div className="flex min-h-0 flex-col gap-3">
            <div className="flex shrink-0 items-center justify-between">
              <div className="flex items-center gap-2">
                <h3 className="text-lg font-semibold">{t("groups")}</h3>
                {profileList.length > 0 && (
                  <Select
                    value={activeProfileId !== undefined ? String(activeProfileId) : undefined}
                    onValueChange={(v) => setActiveProfileId(Number(v))}
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
              {groupsLoading || activeProfileId === undefined ? (
                <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
              ) : groups.length === 0 ? (
                <p className="text-muted-foreground text-sm">{t("emptyGroups")}</p>
              ) : (
                renderSortableGroups
              )}
            </ScrollArea>
          </div>
        </div>
      </div>

      {/* Drag overlay — reuse original element structure, no drop animation */}
      <DragOverlay dropAnimation={null}>
        {activeDrag?.type === "model" && (
          <ModelCard
            modelId={activeDrag.model}
            className="cursor-grabbing shadow-lg"
            price={priceMap.get(activeDrag.model)}
          >
            <ModelSourceBadge
              modelId={activeDrag.model}
              isApiFetched={
                channels
                  .find((c) => c.id === activeDrag.channelId)
                  ?.fetchedModel?.includes(activeDrag.model) ?? false
              }
            />
          </ModelCard>
        )}
        {activeDrag?.type === "channel" && <ChannelOverlay channel={activeDrag.channel} />}
        {activeDrag?.type === "group" && (
          <GroupOverlay group={groups.find((g) => g.id === activeDrag.groupId)} />
        )}
      </DragOverlay>

      {/* ─── Delete Confirmations ────────────────── */}
      <ConfirmDeleteDialog
        open={!!deleteChannelConfirm}
        onOpenChange={(open) => !open && setDeleteChannelConfirm(null)}
        title={t("deleteChannelTitle", { name: deleteChannelConfirm?.name })}
        description={t("deleteChannelDesc")}
        cancelLabel={t("actions.cancel", { ns: "common" })}
        confirmLabel={t("actions.delete", { ns: "common" })}
        onConfirm={() => {
          if (deleteChannelConfirm) channelDeleteMut.mutate(deleteChannelConfirm.id)
          setDeleteChannelConfirm(null)
        }}
      />

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

      {/* ─── Channel Dialog ───────────────────── */}
      <Suspense fallback={null}>
        <ChannelDialog
          open={channelDialogOpen}
          onOpenChange={(open) => {
            setChannelDialogOpen(open)
            if (!open) setTimeout(() => setChannelForm(EMPTY_CHANNEL_FORM), 200)
          }}
          form={channelForm}
          setForm={setChannelForm}
          onSave={() => channelSaveMut.mutate(channelForm)}
          isPending={channelSaveMut.isPending}
        />
      </Suspense>

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

      {/* ─── Model List Dialog ────────────────── */}
      <Dialog open={modelListOpen} onOpenChange={setModelListOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("availableModels")}</DialogTitle>
          </DialogHeader>
          <div className="flex max-h-80 flex-col gap-1 overflow-y-auto">
            {models.length === 0 ? (
              <p className="text-muted-foreground py-4 text-center text-sm">
                {t("noModelsAvailable")}
              </p>
            ) : (
              models.map((m) => {
                const price = priceMap.get(m)
                return (
                  <div
                    key={m}
                    className="bg-muted flex items-center justify-between rounded-md px-3 py-2 font-mono text-sm"
                  >
                    <span className="truncate">{m}</span>
                    {price ? (
                      <span className="text-muted-foreground ml-2 shrink-0 text-xs">
                        ↓{price.inputPrice.toFixed(2)} ↑{price.outputPrice.toFixed(2)}
                      </span>
                    ) : (
                      <span className="text-muted-foreground/50 ml-2 shrink-0 text-xs">-</span>
                    )}
                  </div>
                )
              })
            )}
          </div>
        </DialogContent>
      </Dialog>

      {/* ─── Price Dialog ────────────────────────── */}
      <Suspense fallback={null}>
        <PriceDialog
          open={priceDialogOpen}
          onOpenChange={(open: boolean) => {
            setPriceDialogOpen(open)
            if (!open) {
              setTimeout(() => {
                setPriceForm(EMPTY_PRICE_FORM)
                setEditingPriceId(null)
              }, 200)
            }
          }}
          form={priceForm}
          onChange={setPriceForm}
          onSubmit={() => {
            if (editingPriceId) {
              updatePriceMut.mutate({ id: editingPriceId, form: priceForm })
            } else {
              createPriceMut.mutate(priceForm)
            }
          }}
          isPending={editingPriceId ? updatePriceMut.isPending : createPriceMut.isPending}
          title={editingPriceId ? t("price.editModelPrice") : t("price.addModelPrice")}
          submitLabel={
            editingPriceId
              ? t("actions.save", { ns: "common" })
              : t("actions.create", { ns: "common" })
          }
          nameReadonly={!!editingPriceId}
        />
      </Suspense>

      {/* ─── Delete Price Confirmation ────────────── */}
      <ConfirmDeleteDialog
        open={!!deletePriceConfirm}
        onOpenChange={(open) => !open && setDeletePriceConfirm(null)}
        title={t("price.deleteDialog.title", { name: deletePriceConfirm?.name })}
        description={t("price.deleteDialog.description")}
        cancelLabel={t("actions.cancel", { ns: "common" })}
        confirmLabel={t("actions.delete", { ns: "common" })}
        onConfirm={() => {
          if (deletePriceConfirm) deletePriceMut.mutate(deletePriceConfirm.name)
          setDeletePriceConfirm(null)
        }}
      />

      {/* ─── Profile Manage Dialog ────────────── */}
      <Suspense fallback={null}>
        <ProfileManageDialog open={profileDialogOpen} onOpenChange={setProfileDialogOpen} />
      </Suspense>

      {/* ─── Routing Rules Sheet ────────────── */}
      <Suspense fallback={null}>
        <RoutingRulesSheet
          open={routingRulesOpen}
          onOpenChange={setRoutingRulesOpen}
          groupNames={groups.map((g) => g.name).filter(Boolean)}
          modelNames={[...new Set(channels.flatMap((ch) => parseModels(ch.model)))]}
        />
      </Suspense>
    </DndContext>
  )
}

// ─── Helpers ───────────────────────────────────

function parseModels(model: string[]): string[] {
  if (!model || !Array.isArray(model)) return []
  return model.filter(Boolean)
}

// ─── Drag Overlay Components (reuse original structure) ──

function ChannelOverlay({ channel }: { channel: ChannelRecord }) {
  const { t } = useTranslation("model")
  const modelNames = parseModels(channel.model)

  return (
    <Card className="relative mx-[1px] gap-0 overflow-hidden shadow-lg">
      <div
        className={cn(
          "absolute top-0 bottom-0 left-0 w-1.5",
          channel.enabled
            ? "bg-gradient-to-b from-lime-400 to-green-400"
            : "bg-muted-foreground/30",
        )}
      />
      <CardHeader className="flex flex-row items-center justify-between space-y-0 px-3 py-2.5">
        <div className="flex items-center gap-2 pl-1.5">
          <button className="text-muted-foreground cursor-grabbing rounded p-1">
            <GripVertical className="h-4 w-4" />
          </button>
          <div className="flex items-center gap-1.5 text-left">
            <ChevronDown className="text-muted-foreground h-4 w-4 shrink-0 -rotate-90" />
            <div>
              <CardTitle className="text-sm font-semibold">{channel.name}</CardTitle>
              <div className="mt-1 flex items-center gap-1.5">
                <Badge
                  variant="secondary"
                  className="bg-secondary/60 text-muted-foreground rounded-full px-2 py-0 text-[10px] font-normal"
                >
                  {t(`typeLabels.${channel.type}`, { defaultValue: t("unknown") })}
                </Badge>
                {modelNames.length > 0 && (
                  <span className="bg-secondary/30 text-muted-foreground rounded-full px-2 py-0 text-[10px]">
                    {t("modelCount", { count: modelNames.length })}
                  </span>
                )}
              </div>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1 pr-1 opacity-0">
          <Switch checked={channel.enabled} className="scale-90" />
          <Button variant="ghost" size="icon" className="h-8 w-8">
            <Pencil className="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8">
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </CardHeader>
      <div className="grid grid-rows-[0fr]">
        <div className="overflow-hidden">
          <CardContent className="px-3 pt-0 pb-2.5" />
        </div>
      </div>
    </Card>
  )
}

function GroupOverlay({ group }: { group: GroupRecord | undefined }) {
  const { t } = useTranslation("model")
  if (!group) return null

  return (
    <Card className="relative mx-[1px] gap-0 overflow-hidden shadow-lg">
      <div className="absolute top-0 bottom-0 left-0 w-1.5 bg-gradient-to-b from-purple-400 to-indigo-400" />
      <CardHeader className="flex flex-row items-center justify-between space-y-0 px-3 py-2.5">
        <div className="flex items-center gap-2 pl-1.5">
          <button className="text-muted-foreground cursor-grabbing rounded p-1">
            <GripVertical className="h-4 w-4" />
          </button>
          <div className="flex items-center gap-1.5 text-left">
            <ChevronDown className="text-muted-foreground h-4 w-4 shrink-0 -rotate-90" />
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
                {group.items.length > 0 && (
                  <span className="bg-secondary/30 text-muted-foreground rounded-full px-2 py-0 text-[10px]">
                    {t("itemCount", { count: group.items.length })}
                  </span>
                )}
              </div>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1 pr-1 opacity-0">
          {group.items.length > 0 && (
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <X className="h-4 w-4" />
            </Button>
          )}
          <Button variant="ghost" size="icon" className="h-8 w-8">
            <Pencil className="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8">
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </CardHeader>
      <div className="grid grid-rows-[0fr]">
        <div className="overflow-hidden">
          <CardContent className="px-3 pt-0 pb-2.5" />
        </div>
      </div>
    </Card>
  )
}

// ─── Draggable Channel Card ────────────────────

function DraggableChannel({
  channel,
  highlighted,
  refCallback,
  onEdit,
  onDelete,
  onToggle,
  enablePending,
  forceCollapsed,
  priceMap,
  healthStatus,
}: {
  channel: ChannelRecord
  highlighted?: boolean
  refCallback?: (el: HTMLDivElement | null) => void
  onEdit: () => void
  onDelete: () => void
  onToggle: (enabled: boolean) => void
  enablePending?: boolean
  forceCollapsed?: boolean
  priceMap: Map<string, ModelPrice>
  healthStatus?: number
}) {
  const { t } = useTranslation("model")
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: `sortable-channel-${channel.id}`,
    data: { type: "channel", channel } satisfies DragDataChannel,
  })

  const modelNames = parseModels(channel.model)
  const fetchedSet = useMemo(() => new Set(channel.fetchedModel ?? []), [channel.fetchedModel])
  const [collapsed, setCollapsed] = useState(true)

  // Sync with parent's expand/collapse all toggle
  const prevForceCollapsedRef = useRef(forceCollapsed)
  if (prevForceCollapsedRef.current !== forceCollapsed) {
    prevForceCollapsedRef.current = forceCollapsed
    if (forceCollapsed !== undefined) setCollapsed(forceCollapsed)
  }

  // Auto-expand if highlighted while collapsed
  const prevHighlightedRef = useRef(highlighted)
  if (prevHighlightedRef.current !== highlighted) {
    prevHighlightedRef.current = highlighted
    if (highlighted && collapsed) setCollapsed(false)
  }

  const style = {
    transform: CSS.Translate.toString(transform),
    transition,
  }

  return (
    <Card
      ref={(node) => {
        setNodeRef(node)
        refCallback?.(node)
      }}
      style={style}
      className={cn(
        "relative mx-[1px] gap-0 overflow-hidden shadow-sm transition-shadow hover:shadow-md",
        isDragging && "opacity-30",
        highlighted && "border-primary animate-pulse border-2",
      )}
    >
      <div
        className={cn(
          "absolute top-0 bottom-0 left-0 w-1.5",
          channel.enabled
            ? "bg-gradient-to-b from-lime-400 to-green-400"
            : "bg-muted-foreground/30",
        )}
      />
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
              <div className="flex items-center gap-1.5">
                <CardTitle className="text-sm font-semibold">{channel.name}</CardTitle>
                {healthStatus !== undefined && healthStatus > 0 && (
                  <span
                    className={cn(
                      "inline-block h-2 w-2 shrink-0 rounded-full",
                      healthStatus === 1 && "bg-green-500",
                      healthStatus === 2 && "bg-yellow-500",
                      healthStatus === 3 && "bg-red-500",
                    )}
                    title={
                      healthStatus === 1 ? "Healthy" : healthStatus === 2 ? "Degraded" : "Down"
                    }
                  />
                )}
              </div>
              <div className="mt-1 flex items-center gap-1.5">
                <Badge
                  variant="secondary"
                  className="bg-secondary/60 text-muted-foreground rounded-full px-2 py-0 text-[10px] font-normal"
                >
                  {t(`typeLabels.${channel.type}`, { defaultValue: t("unknown") })}
                </Badge>
                {collapsed && modelNames.length > 0 && (
                  <span className="bg-secondary/30 text-muted-foreground rounded-full px-2 py-0 text-[10px]">
                    {t("modelCount", { count: modelNames.length })}
                  </span>
                )}
              </div>
            </div>
          </button>
        </div>
        <div className="flex items-center gap-1 pr-1">
          <Switch
            checked={channel.enabled}
            onCheckedChange={onToggle}
            disabled={enablePending}
            className="scale-90"
          />
          <Button
            variant="ghost"
            size="icon"
            className="text-muted-foreground h-8 w-8"
            aria-label="Edit channel"
            onClick={onEdit}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-muted-foreground hover:text-destructive h-8 w-8"
            aria-label="Delete channel"
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
          <CardContent className={cn("px-3 pt-0 pb-2.5", !channel.enabled && "opacity-50")}>
            <div className="mt-1">
              {modelNames.length === 0 ? (
                <span className="text-muted-foreground text-xs">{t("noModels")}</span>
              ) : (
                <GroupedModelList
                  models={modelNames}
                  renderModel={(m) => (
                    <DraggableModelTag
                      key={m}
                      model={m}
                      channelId={channel.id}
                      channelName={channel.name}
                      priceMap={priceMap}
                      isApiFetched={fetchedSet.has(m)}
                    />
                  )}
                />
              )}
            </div>
          </CardContent>
        </div>
      </div>
    </Card>
  )
}

// ─── Draggable Model Tag ───────────────────────

function DraggableModelTag({
  model,
  channelId,
  channelName,
  priceMap,
  isApiFetched,
}: {
  model: string
  channelId: number
  channelName: string
  priceMap: Map<string, ModelPrice>
  isApiFetched: boolean
}) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: `model-${channelId}-${model}`,
    data: { type: "model", model, channelId, channelName } satisfies DragDataModel,
  })

  const price = priceMap.get(model)

  return (
    <ModelCard
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      modelId={model}
      className={cn(
        "hover:bg-accent cursor-grab touch-none",
        isDragging && "border-dashed opacity-30",
      )}
      price={price ? { inputPrice: price.inputPrice, outputPrice: price.outputPrice } : undefined}
    >
      <ModelSourceBadge modelId={model} isApiFetched={isApiFetched} />
    </ModelCard>
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
    items.forEach((it, i) => map.set(it, i))
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
