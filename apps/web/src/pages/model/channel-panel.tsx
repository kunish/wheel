import type { ChannelFormData } from "./channel-dialog"
import type { ChannelRecord, DragDataChannel, DragDataModel, ModelPrice } from "./types"
import { useDraggable } from "@dnd-kit/core"
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
} from "lucide-react"
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"
import { ConfirmDeleteDialog } from "@/components/confirm-delete-dialog"
import { GroupedModelList } from "@/components/grouped-model-list"
import { ModelCard } from "@/components/model-card"
import { ModelSourceBadge } from "@/components/model-source-badge"
import { ProviderIcon } from "@/components/provider-icon"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"
import { EMPTY_CHANNEL_FORM } from "./channel-dialog"
import { CodexChannelDetail } from "./codex-channel-detail"
import { isRuntimeChannelType, OUTBOUND_CURSOR_CHANNEL_TYPE } from "./codex-channel-draft"
import { CursorChannelDetail } from "./cursor-channel-detail"

// ─── Inline delete confirmation ────────────────
// Re-uses ConfirmDeleteDialog from the shared component

import { parseModels } from "./types"
import { useChannelMutations } from "./use-channel-mutations"

const ChannelDialog = lazy(() => import("./channel-dialog"))

// ─── Props ─────────────────────────────────────

export interface ChannelPanelProps {
  priceMap: Map<string, ModelPrice>
  channels: ChannelRecord[]
  channelsLoading: boolean
  healthMap: Record<string, number>
}

// ─── ChannelPanel ──────────────────────────────

export function ChannelPanel({
  priceMap,
  channels,
  channelsLoading,
  healthMap,
}: ChannelPanelProps) {
  const { t } = useTranslation("model")
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

  // Delete confirmation state
  const [deleteChannelConfirm, setDeleteChannelConfirm] = useState<ChannelRecord | null>(null)

  // Mutations
  const { channelDeleteMut, channelEnableMut, channelSaveMut } = useChannelMutations({
    onSaveSuccess: () => setChannelDialogOpen(false),
    channelForm,
  })

  const channelIds = useMemo(() => channels.map((ch) => `sortable-channel-${ch.id}`), [channels])

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

  // ─── Render ────────────────────────────────────

  return (
    <>
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

      {/* ─── Delete Confirmation ────────────────── */}
      {deleteChannelConfirm && (
        <ConfirmDeleteInline
          title={t("deleteChannelTitle", { name: deleteChannelConfirm.name })}
          description={t("deleteChannelDesc")}
          onConfirm={() => {
            channelDeleteMut.mutate(deleteChannelConfirm.id)
            setDeleteChannelConfirm(null)
          }}
          onCancel={() => setDeleteChannelConfirm(null)}
        />
      )}

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
    </>
  )
}

function ConfirmDeleteInline({
  title,
  description,
  onConfirm,
  onCancel,
}: {
  title: string
  description: string
  onConfirm: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("model")
  return (
    <ConfirmDeleteDialog
      open
      onOpenChange={(open) => !open && onCancel()}
      title={title}
      description={description}
      cancelLabel={t("actions.cancel", { ns: "common" })}
      confirmLabel={t("actions.delete", { ns: "common" })}
      onConfirm={onConfirm}
    />
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
                  className="bg-secondary/60 text-muted-foreground inline-flex items-center gap-1 rounded-full px-2 py-0 text-[10px] font-normal"
                >
                  <ProviderIcon channelType={channel.type} size={12} />
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
            {channel.type === OUTBOUND_CURSOR_CHANNEL_TYPE ? (
              <CursorChannelDetail channelId={channel.id} />
            ) : isRuntimeChannelType(channel.type) ? (
              <CodexChannelDetail
                channelId={channel.id}
                channelType={channel.type}
                modelCount={modelNames.length}
              />
            ) : null}
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
