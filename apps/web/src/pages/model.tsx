import type { DragEndEvent, DragOverEvent, DragStartEvent } from "@dnd-kit/core"
import type { GroupItemForm } from "./model/group-dialog"
import type { PriceFormData } from "./model/price-dialog"
import type { ChannelRecord, DragData, GroupRecord, ModelPrice } from "./model/types"
import type { ModelProfile } from "@/lib/api-client"
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  TouchSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core"
import { arrayMove } from "@dnd-kit/sortable"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ChevronDown,
  GitBranch,
  GripVertical,
  Layers,
  List,
  Pencil,
  RefreshCw,
  Trash2,
  X,
} from "lucide-react"
import { lazy, Suspense, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ConfirmDeleteDialog } from "@/components/confirm-delete-dialog"
import { ModelCard } from "@/components/model-card"
import { ModelSourceBadge } from "@/components/model-source-badge"
import { ProviderIcon } from "@/components/provider-icon"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Switch } from "@/components/ui/switch"
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
import { ChannelPanel } from "./model/channel-panel"
import { GroupPanel } from "./model/group-panel"
import { EMPTY_PRICE_FORM } from "./model/price-dialog"
import { parseModels } from "./model/types"
import { useChannelMutations } from "./model/use-channel-mutations"
import { useGroupMutations } from "./model/use-group-mutations"
import { usePriceMutations } from "./model/use-price-mutations"
import { useProfileMutations } from "./model/use-profile-mutations"

// ─── Lazy-loaded Dialog components ──────────────

const PriceDialog = lazy(() => import("./model/price-dialog"))
const ProfileManageDialog = lazy(() => import("./model/profile-manage-dialog"))
const RoutingRulesSheet = lazy(() => import("./model/routing-rules-sheet"))

// ─── Main page ─────────────────────────────────

export default function ModelPage() {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()

  // Drag state
  const [activeDrag, setActiveDrag] = useState<DragData | null>(null)
  const [hoverGroupId, setHoverGroupId] = useState<number | null>(null)

  // Model list dialog
  const [modelListOpen, setModelListOpen] = useState(false)

  // Price state
  const [priceDialogOpen, setPriceDialogOpen] = useState(false)
  const [priceForm, setPriceForm] = useState<PriceFormData>(EMPTY_PRICE_FORM)
  const [editingPriceId, setEditingPriceId] = useState<number | null>(null)
  const [deletePriceConfirm, setDeletePriceConfirm] = useState<ModelPrice | null>(null)

  // Profile / routing state
  const [profileDialogOpen, setProfileDialogOpen] = useState(false)
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

  // ─── Shared queries ───────────────────────────

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

  const { data: priceData } = useQuery({
    queryKey: ["model-prices"],
    queryFn: listModelPrices,
    staleTime: 60_000,
  })

  const { data: healthData } = useQuery({
    queryKey: ["channel-health"],
    queryFn: getChannelHealth,
    refetchInterval: 30_000,
  })

  const { data: updateTimeData } = useQuery({
    queryKey: ["price-update-time"],
    queryFn: getLastPriceUpdateTime,
  })

  // ─── Derived data ─────────────────────────────

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

  // ─── Mutation hooks for DnD ───────────────────

  const { channelReorderMut } = useChannelMutations({})

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

  const { groupAddItemMut, groupReorderMut, invalidateGroupQueries } = useGroupMutations({
    activeProfileId,
  })

  const { setActiveProfileId } = useProfileMutations({
    onActivateSuccess: () => invalidateGroupQueries(),
  })

  useEffect(() => {
    if (profileList.length === 0 || !settingsData) return
    if (activeProfileId === undefined || !profileList.some((p) => p.id === activeProfileId)) {
      setActiveProfileId(profileList[0].id)
    }
  }, [profileList, activeProfileId, settingsData, setActiveProfileId])

  // ─── Drag handlers ────────────────────────────

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
          <ChannelPanel
            priceMap={priceMap}
            channels={channels}
            channelsLoading={channelsLoading}
            healthMap={healthMap}
          />

          {/* ─── Right: Groups ────────────────── */}
          <GroupPanel
            priceMap={priceMap}
            channels={channels}
            profileId={activeProfileId}
            profileList={profileList}
            groups={groups}
            groupsLoading={groupsLoading}
            activeDrag={activeDrag}
            hoverGroupId={hoverGroupId}
            onProfileChange={setActiveProfileId}
          />
        </div>
      </div>

      {/* Drag overlay */}
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

// ─── Drag Overlay Components ───────────────────

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
          <button type="button" className="text-muted-foreground cursor-grabbing rounded p-1">
            <GripVertical className="h-4 w-4" />
          </button>
          <div className="flex items-center gap-1.5 text-left">
            <ChevronDown className="text-muted-foreground h-4 w-4 shrink-0 -rotate-90" />
            <div>
              <CardTitle className="text-sm font-semibold">{channel.name}</CardTitle>
              <div className="mt-1 flex items-center gap-1.5">
                <Badge
                  variant="secondary"
                  className="bg-secondary/60 text-muted-foreground inline-flex items-center gap-1 rounded-full px-2 py-0 text-[10px] font-normal"
                >
                  <ProviderIcon channelType={channel.type} size={12} />
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
          <button type="button" className="text-muted-foreground cursor-grabbing rounded p-1">
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
