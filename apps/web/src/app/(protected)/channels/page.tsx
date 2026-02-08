"use client"

import type { DragEndEvent, DragStartEvent } from "@dnd-kit/core"
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ChevronDown,
  Download,
  GripVertical,
  List,
  Loader2,
  Pencil,
  Plus,
  Search,
  Trash2,
  X,
} from "lucide-react"
import { useMemo, useState } from "react"
import { toast } from "sonner"
import { GroupedModelList } from "@/components/grouped-model-list"
import { ModelCard } from "@/components/model-card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { fuzzyLookup, useModelMetadataQuery } from "@/hooks/use-model-meta"
import {
  createChannel,
  createGroup,
  deleteChannel,
  deleteGroup,
  enableChannel,
  fetchChannelModelsPreview,
  getModelList,
  listChannels,
  listGroups,
  updateChannel,
  updateGroup,
} from "@/lib/api"
import { groupModelsByProvider } from "@/lib/group-models"
import { cn } from "@/lib/utils"

// ─── Type labels ───────────────────────────────

const TYPE_LABELS: Record<number, string> = {
  0: "OpenAI Chat",
  1: "OpenAI",
  2: "Anthropic",
  3: "Gemini",
  4: "Volcengine",
  5: "OpenAI Embedding",
}

const MODE_LABELS: Record<number, string> = {
  1: "RoundRobin",
  2: "Random",
  3: "Failover",
  4: "Weighted",
}

// ─── Types ─────────────────────────────────────

interface ChannelRecord {
  id: number
  name: string
  type: number
  enabled: boolean
  model: string[]
  customModel: string
  baseUrls: { url: string; delay: number }[]
  keys: { channelKey: string; remark: string }[]
  paramOverride: string | null
}

interface GroupItemForm {
  channelId: number
  modelName: string
  priority: number
  weight: number
}

interface GroupRecord {
  id: number
  name: string
  mode: number
  matchRegex: string
  firstTokenTimeOut: number
  items: GroupItemForm[]
}

interface ChannelFormData {
  id?: number
  name: string
  type: number
  enabled: boolean
  baseUrls: { url: string; delay: number }[]
  keys: { channelKey: string; remark: string }[]
  model: string[]
  customModel: string
  paramOverride: string
}

interface GroupFormData {
  id?: number
  name: string
  mode: number
  matchRegex: string
  firstTokenTimeOut: number
  items: GroupItemForm[]
}

const EMPTY_CHANNEL_FORM: ChannelFormData = {
  name: "",
  type: 1,
  enabled: true,
  baseUrls: [{ url: "", delay: 0 }],
  keys: [{ channelKey: "", remark: "" }],
  model: [],
  customModel: "",
  paramOverride: "",
}

const EMPTY_GROUP_FORM: GroupFormData = {
  name: "",
  mode: 1,
  matchRegex: "",
  firstTokenTimeOut: 30,
  items: [],
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

type DragData = DragDataModel | DragDataChannel

// ─── Main page ─────────────────────────────────

export default function ChannelsAndGroupsPage() {
  const queryClient = useQueryClient()

  // Channel state
  const [channelDialogOpen, setChannelDialogOpen] = useState(false)
  const [channelForm, setChannelForm] = useState<ChannelFormData>(EMPTY_CHANNEL_FORM)

  // Group state
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const [groupForm, setGroupForm] = useState<GroupFormData>(EMPTY_GROUP_FORM)
  const [modelListOpen, setModelListOpen] = useState(false)

  // Drag state
  const [activeDrag, setActiveDrag] = useState<DragData | null>(null)

  // Queries
  const { data: channelData, isLoading: channelsLoading } = useQuery({
    queryKey: ["channels"],
    queryFn: listChannels,
  })

  const { data: groupData, isLoading: groupsLoading } = useQuery({
    queryKey: ["groups"],
    queryFn: listGroups,
  })

  const { data: modelData } = useQuery({
    queryKey: ["model-list"],
    queryFn: getModelList,
    enabled: modelListOpen,
  })

  const channels = (channelData?.data?.channels ?? []) as ChannelRecord[]
  const groups = (groupData?.data?.groups ?? []) as GroupRecord[]
  const models = (modelData?.data?.models ?? []) as string[]

  // ─── Channel mutations ─────────────────────────

  const channelDeleteMut = useMutation({
    mutationFn: deleteChannel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["channels"] })
      toast.success("Channel deleted")
    },
  })

  const channelEnableMut = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => enableChannel(id, enabled),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["channels"] }),
  })

  const channelSaveMut = useMutation({
    mutationFn: (data: ChannelFormData) => (data.id ? updateChannel(data) : createChannel(data)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["channels"] })
      setChannelDialogOpen(false)
      toast.success(channelForm.id ? "Channel updated" : "Channel created")
    },
    onError: (err: Error) => toast.error(err.message),
  })

  // ─── Group mutations ───────────────────────────

  const groupDeleteMut = useMutation({
    mutationFn: deleteGroup,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groups"] })
      toast.success("Group deleted")
    },
  })

  const groupSaveMut = useMutation({
    mutationFn: (data: GroupFormData) => (data.id ? updateGroup(data) : createGroup(data)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groups"] })
      setGroupDialogOpen(false)
      toast.success(groupForm.id ? "Group updated" : "Group created")
    },
    onError: (err: Error) => toast.error(err.message),
  })

  // Quick-add item to group (from drag or button)
  const groupAddItemMut = useMutation({
    mutationFn: (data: { group: GroupRecord; newItems: GroupItemForm[] }) => {
      const merged = [...data.group.items, ...data.newItems]
      return updateGroup({
        id: data.group.id,
        name: data.group.name,
        mode: data.group.mode,
        matchRegex: data.group.matchRegex,
        firstTokenTimeOut: data.group.firstTokenTimeOut,
        items: merged,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groups"] })
      toast.success("Channel added to group")
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const groupClearMut = useMutation({
    mutationFn: (group: GroupRecord) =>
      updateGroup({
        id: group.id,
        name: group.name,
        mode: group.mode,
        matchRegex: group.matchRegex,
        firstTokenTimeOut: group.firstTokenTimeOut,
        items: [],
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groups"] })
      toast.success("Group cleared")
    },
    onError: (err: Error) => toast.error(err.message),
  })

  // ─── Drag handlers ─────────────────────────────

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))

  function handleDragStart(event: DragStartEvent) {
    setActiveDrag(event.active.data.current as DragData)
  }

  function handleDragEnd(event: DragEndEvent) {
    const { over } = event
    setActiveDrag(null)

    if (!over) return

    const dropData = over.data.current as { groupId: number } | undefined
    if (!dropData?.groupId) return

    const dragData = event.active.data.current as DragData
    const targetGroup = groups.find((g) => g.id === dropData.groupId)
    if (!targetGroup) return

    let newItems: GroupItemForm[] = []

    if (dragData.type === "model") {
      // Check for duplicates
      const exists = targetGroup.items.some(
        (it) => it.channelId === dragData.channelId && it.modelName === dragData.model,
      )
      if (exists) {
        toast.info("This model is already in this group")
        return
      }
      newItems = [
        {
          channelId: dragData.channelId,
          modelName: dragData.model,
          priority: 0,
          weight: 1,
        },
      ]
    } else if (dragData.type === "channel") {
      const ch = dragData.channel
      const modelNames = parseModels(ch.model)
      if (modelNames.length === 0) {
        toast.error("This channel has no models configured")
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
        }))
      if (newItems.length === 0) {
        toast.info("All models from this channel are already in this group")
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
      matchRegex: g.matchRegex ?? "",
      firstTokenTimeOut: g.firstTokenTimeOut,
      items: g.items ?? [],
    })
    setGroupDialogOpen(true)
  }

  const channelMap = new Map(channels.map((ch) => [ch.id, ch]))

  // ─── Render ────────────────────────────────────

  return (
    <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
      <div className="flex h-full flex-col gap-4">
        <div className="flex items-center justify-between">
          <h2 className="text-2xl font-bold tracking-tight">Channels & Groups</h2>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setModelListOpen(true)}>
              <List className="mr-2 h-4 w-4" /> Models
            </Button>
          </div>
        </div>

        <div className="grid min-h-0 flex-1 grid-cols-1 gap-6 lg:grid-cols-2">
          {/* ─── Left: Channels ───────────────── */}
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold">Channels</h3>
              <Button size="sm" onClick={openCreateChannel}>
                <Plus className="mr-1 h-3 w-3" /> Add
              </Button>
            </div>

            <ScrollArea className="flex-1">
              {channelsLoading ? (
                <p className="text-muted-foreground">Loading...</p>
              ) : channels.length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  No channels. Create one to get started.
                </p>
              ) : (
                <div className="flex flex-col gap-3">
                  {channels.map((ch) => (
                    <DraggableChannel
                      key={ch.id}
                      channel={ch}
                      onEdit={() => openEditChannel(ch)}
                      onDelete={() => {
                        if (confirm("Delete this channel?")) {
                          channelDeleteMut.mutate(ch.id)
                        }
                      }}
                      onToggle={(enabled) => channelEnableMut.mutate({ id: ch.id, enabled })}
                    />
                  ))}
                </div>
              )}
            </ScrollArea>
          </div>

          {/* ─── Right: Groups ────────────────── */}
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold">Groups</h3>
              <Button size="sm" onClick={openCreateGroup}>
                <Plus className="mr-1 h-3 w-3" /> Add
              </Button>
            </div>

            <ScrollArea className="flex-1">
              {groupsLoading ? (
                <p className="text-muted-foreground">Loading...</p>
              ) : groups.length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  No groups. Create one, then drag channels or models into it.
                </p>
              ) : (
                <div className="flex flex-col gap-3">
                  {groups.map((g) => (
                    <DroppableGroup
                      key={g.id}
                      group={g}
                      channelMap={channelMap}
                      onEdit={() => openEditGroup(g)}
                      onDelete={() => {
                        if (confirm("Delete this group?")) {
                          groupDeleteMut.mutate(g.id)
                        }
                      }}
                      onClear={() => {
                        if (confirm("Clear all channels from this group?")) {
                          groupClearMut.mutate(g)
                        }
                      }}
                      isOver={activeDrag !== null}
                    />
                  ))}
                </div>
              )}
            </ScrollArea>
          </div>
        </div>
      </div>

      {/* Drag overlay */}
      <DragOverlay>
        {activeDrag?.type === "model" && (
          <ModelCard modelId={activeDrag.model} className="cursor-grabbing shadow-lg" />
        )}
        {activeDrag?.type === "channel" && (
          <Card className="w-64 cursor-grabbing opacity-90 shadow-lg">
            <CardHeader className="p-3">
              <CardTitle className="text-sm">{activeDrag.channel.name}</CardTitle>
            </CardHeader>
          </Card>
        )}
      </DragOverlay>

      {/* ─── Channel Dialog ───────────────────── */}
      <ChannelDialog
        open={channelDialogOpen}
        onOpenChange={setChannelDialogOpen}
        form={channelForm}
        setForm={setChannelForm}
        onSave={() => channelSaveMut.mutate(channelForm)}
        isPending={channelSaveMut.isPending}
      />

      {/* ─── Group Dialog ─────────────────────── */}
      <GroupDialog
        open={groupDialogOpen}
        onOpenChange={setGroupDialogOpen}
        form={groupForm}
        setForm={setGroupForm}
        channelOptions={channels}
        onSave={() => groupSaveMut.mutate(groupForm)}
        isPending={groupSaveMut.isPending}
      />

      {/* ─── Model List Dialog ────────────────── */}
      <Dialog open={modelListOpen} onOpenChange={setModelListOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Available Models</DialogTitle>
          </DialogHeader>
          <div className="flex max-h-80 flex-col gap-1 overflow-y-auto">
            {models.length === 0 ? (
              <p className="text-muted-foreground py-4 text-center text-sm">No models available.</p>
            ) : (
              models.map((m) => (
                <div key={m} className="bg-muted rounded-md px-3 py-2 font-mono text-sm">
                  {m}
                </div>
              ))
            )}
          </div>
        </DialogContent>
      </Dialog>
    </DndContext>
  )
}

// ─── Helpers ───────────────────────────────────

function parseModels(model: string[]): string[] {
  if (!model || !Array.isArray(model)) return []
  return model.filter(Boolean)
}

// ─── Draggable Channel Card ────────────────────

function DraggableChannel({
  channel,
  onEdit,
  onDelete,
  onToggle,
}: {
  channel: ChannelRecord
  onEdit: () => void
  onDelete: () => void
  onToggle: (enabled: boolean) => void
}) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: `channel-${channel.id}`,
    data: { type: "channel", channel } satisfies DragDataChannel,
  })

  const modelNames = parseModels(channel.model)
  const [collapsed, setCollapsed] = useState(false)

  return (
    <Card ref={setNodeRef} className={cn("transition-opacity", isDragging && "opacity-40")}>
      <CardHeader className="flex flex-row items-start justify-between space-y-0 p-3 pb-1">
        <div className="flex items-start gap-2">
          <button
            {...attributes}
            {...listeners}
            className="text-muted-foreground hover:text-foreground mt-1 cursor-grab"
          >
            <GripVertical className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => setCollapsed(!collapsed)}
            className="flex items-start gap-1.5 text-left"
          >
            <ChevronDown
              className={cn(
                "text-muted-foreground mt-0.5 h-4 w-4 shrink-0 transition-transform",
                collapsed && "-rotate-90",
              )}
            />
            <div>
              <CardTitle className="text-sm">{channel.name}</CardTitle>
              <div className="mt-1 flex items-center gap-1.5">
                <Badge variant="secondary" className="text-xs">
                  {TYPE_LABELS[channel.type] ?? "Unknown"}
                </Badge>
                {collapsed && modelNames.length > 0 && (
                  <span className="text-muted-foreground text-xs">
                    {modelNames.length} model{modelNames.length !== 1 ? "s" : ""}
                  </span>
                )}
              </div>
            </div>
          </button>
        </div>
        <div className="flex items-center gap-1">
          <Switch checked={channel.enabled} onCheckedChange={onToggle} />
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit}>
            <Pencil className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onDelete}>
            <Trash2 className="text-destructive h-3 w-3" />
          </Button>
        </div>
      </CardHeader>
      {!collapsed && (
        <CardContent className={cn("p-3 pt-1", !channel.enabled && "opacity-50")}>
          <div className="mt-1">
            {modelNames.length === 0 ? (
              <span className="text-muted-foreground text-xs">No models</span>
            ) : (
              <GroupedModelList
                models={modelNames}
                renderModel={(m) => (
                  <DraggableModelTag
                    key={m}
                    model={m}
                    channelId={channel.id}
                    channelName={channel.name}
                  />
                )}
              />
            )}
          </div>
        </CardContent>
      )}
    </Card>
  )
}

// ─── Draggable Model Tag ───────────────────────

function DraggableModelTag({
  model,
  channelId,
  channelName,
}: {
  model: string
  channelId: number
  channelName: string
}) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: `model-${channelId}-${model}`,
    data: { type: "model", model, channelId, channelName } satisfies DragDataModel,
  })

  return (
    <ModelCard
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      modelId={model}
      className={cn("hover:bg-accent cursor-grab", isDragging && "opacity-40")}
    />
  )
}

// ─── Droppable Group Card ──────────────────────

function DroppableGroup({
  group,
  channelMap,
  onEdit,
  onDelete,
  onClear,
  isOver: dragActive,
}: {
  group: GroupRecord
  channelMap: Map<number, ChannelRecord>
  onEdit: () => void
  onDelete: () => void
  onClear: () => void
  isOver: boolean
}) {
  const { setNodeRef, isOver } = useDroppable({
    id: `group-${group.id}`,
    data: { groupId: group.id },
  })

  const { data: metaData } = useModelMetadataQuery()
  const [collapsed, setCollapsed] = useState(false)

  return (
    <Card
      ref={setNodeRef}
      className={cn(
        "transition-all",
        isOver && "ring-primary border-primary ring-2",
        dragActive && !isOver && "border-dashed",
      )}
    >
      <CardHeader className="flex flex-row items-start justify-between space-y-0 p-3 pb-1">
        <button
          type="button"
          onClick={() => setCollapsed(!collapsed)}
          className="flex items-start gap-1.5 text-left"
        >
          <ChevronDown
            className={cn(
              "text-muted-foreground mt-0.5 h-4 w-4 shrink-0 transition-transform",
              collapsed && "-rotate-90",
            )}
          />
          <div>
            <CardTitle className="text-sm">{group.name}</CardTitle>
            <div className="mt-1 flex items-center gap-1">
              <Badge variant="secondary" className="text-xs">
                {MODE_LABELS[group.mode] ?? "Unknown"}
              </Badge>
              {group.matchRegex && (
                <Badge variant="outline" className="font-mono text-xs">
                  {group.matchRegex}
                </Badge>
              )}
              {collapsed && group.items.length > 0 && (
                <span className="text-muted-foreground text-xs">
                  {group.items.length} item{group.items.length !== 1 ? "s" : ""}
                </span>
              )}
            </div>
          </div>
        </button>
        <div className="flex items-center gap-1">
          {group.items.length > 0 && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={onClear}
              title="Clear all"
            >
              <X className="h-3 w-3" />
            </Button>
          )}
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onEdit}>
            <Pencil className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onDelete}>
            <Trash2 className="text-destructive h-3 w-3" />
          </Button>
        </div>
      </CardHeader>
      {!collapsed && (
        <CardContent className="p-3 pt-1">
          <div className="text-muted-foreground mb-1 text-xs">
            Timeout: {group.firstTokenTimeOut}s
          </div>
          {group.items.length === 0 ? (
            <div
              className={cn(
                "text-muted-foreground rounded-md border border-dashed p-4 text-center text-xs",
                isOver && "bg-primary/5 border-primary",
              )}
            >
              {dragActive ? "Drop here to add" : "Drag channels or models here"}
            </div>
          ) : (
            <GroupItemList
              items={group.items}
              mode={group.mode}
              channelMap={channelMap}
              metadataMap={metaData?.data}
            />
          )}
        </CardContent>
      )}
    </Card>
  )
}

// ─── Group Item List (grouped by provider) ────

function GroupItemList({
  items,
  mode,
  channelMap,
  metadataMap,
}: {
  items: GroupItemForm[]
  mode: number
  channelMap: Map<number, ChannelRecord>
  metadataMap: Record<string, import("@/lib/api").ModelMeta> | undefined
}) {
  // Separate model items from "all" items
  const modelItems = items.filter((it) => it.modelName)
  const allItems = items.filter((it) => !it.modelName)
  const modelIds = modelItems.map((it) => it.modelName)
  const itemByModel = new Map(modelItems.map((it) => [it.modelName, it]))

  // Build resolved metadata map using fuzzy matching
  const resolvedMap = useMemo(() => {
    if (!metadataMap) return undefined
    const map: Record<string, import("@/lib/api").ModelMeta> = {}
    for (const id of modelIds) {
      const meta = fuzzyLookup(metadataMap, id)
      if (meta) map[id] = meta
    }
    return map
  }, [metadataMap, modelIds])

  const groups = useMemo(
    () => groupModelsByProvider(modelIds, resolvedMap),
    [modelIds, resolvedMap],
  )

  const shouldGroup = modelIds.length >= 4

  const renderModelCard = (modelName: string, idx: number) => {
    const item = itemByModel.get(modelName)!
    const ch = channelMap.get(item.channelId)
    const isDisabled = ch?.enabled === false
    return (
      <ModelCard
        key={`${item.channelId}-${modelName}-${idx}`}
        modelId={modelName}
        disabled={isDisabled}
      >
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
      <div className="flex flex-wrap gap-1.5">
        {modelItems.map((it, i) => renderModelCard(it.modelName, i))}
        {allItems.map((it, i) => renderAllBadge(it, i))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      {groups.map((g) => (
        <div key={g.provider}>
          <ProviderHeader logoUrl={g.logoUrl} name={g.providerName} count={g.models.length} />
          <div className="flex flex-wrap gap-1.5">
            {g.models.map((m, i) => renderModelCard(m, i))}
          </div>
        </div>
      ))}
      {allItems.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
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

// ─── Channel Dialog ────────────────────────────

function ChannelDialog({
  open,
  onOpenChange,
  form,
  setForm,
  onSave,
  isPending,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  form: ChannelFormData
  setForm: (f: ChannelFormData) => void
  onSave: () => void
  isPending: boolean
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{form.id ? "Edit Channel" : "Create Channel"}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-2">
            <Label>Name</Label>
            <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
          </div>

          <div className="flex flex-col gap-2">
            <Label>Provider Type</Label>
            <Select
              value={String(form.type)}
              onValueChange={(v) => setForm({ ...form, type: Number(v) })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(TYPE_LABELS).map(([val, label]) => (
                  <SelectItem key={val} value={val}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label>Base URL</Label>
            <Input
              placeholder="https://api.openai.com"
              value={form.baseUrls[0]?.url ?? ""}
              onChange={(e) =>
                setForm({
                  ...form,
                  baseUrls: [{ url: e.target.value, delay: form.baseUrls[0]?.delay ?? 0 }],
                })
              }
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>API Key</Label>
            <Input
              placeholder="sk-..."
              value={form.keys[0]?.channelKey ?? ""}
              onChange={(e) =>
                setForm({
                  ...form,
                  keys: [{ channelKey: e.target.value, remark: form.keys[0]?.remark ?? "" }],
                })
              }
            />
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <Label>Models</Label>
              <FetchModelsButton form={form} setForm={setForm} />
            </div>
            <ModelTagInput value={form.model} onChange={(model) => setForm({ ...form, model })} />
          </div>

          <div className="flex flex-col gap-2">
            <Label>Custom Models</Label>
            <Input
              value={form.customModel}
              onChange={(e) => setForm({ ...form, customModel: e.target.value })}
              placeholder="model-alias:actual-model, ..."
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>Parameter Override (JSON)</Label>
            <Textarea
              value={form.paramOverride}
              onChange={(e) => setForm({ ...form, paramOverride: e.target.value })}
              placeholder='{"temperature": 0.7}'
              rows={3}
            />
          </div>

          <Button className="mt-2" onClick={onSave} disabled={isPending || !form.name}>
            {isPending ? "Saving..." : "Save"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ─── Model Tag Input ────────────────────────────

function ModelTagInput({
  value,
  onChange,
}: {
  value: string[]
  onChange: (value: string[]) => void
}) {
  const [input, setInput] = useState("")
  const tags = value ?? []

  function addTags(raw: string) {
    const newTags = raw
      .split(/[,\n]/)
      .map((t) => t.trim())
      .filter(Boolean)
    if (newTags.length === 0) return
    const merged = [...new Set([...tags, ...newTags])]
    onChange(merged)
    setInput("")
  }

  function removeTag(tag: string) {
    onChange(tags.filter((t) => t !== tag))
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault()
      addTags(input)
    }
    if (e.key === "Backspace" && input === "" && tags.length > 0) {
      removeTag(tags[tags.length - 1])
    }
  }

  function handlePaste(e: React.ClipboardEvent<HTMLInputElement>) {
    e.preventDefault()
    addTags(e.clipboardData.getData("text"))
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          onBlur={() => {
            if (input.trim()) addTags(input)
          }}
          placeholder="Type model name, press Enter to add"
          className="flex-1"
        />
        <span className="text-muted-foreground text-xs whitespace-nowrap">
          {tags.length} model{tags.length !== 1 ? "s" : ""}
        </span>
      </div>
      {tags.length > 0 && (
        <div className="max-h-40 overflow-y-auto">
          <GroupedModelList
            models={tags}
            renderModel={(tag) => (
              <ModelCard key={tag} modelId={tag} onRemove={() => removeTag(tag)} />
            )}
          />
        </div>
      )}
    </div>
  )
}

// ─── Fetch Models Button ────────────────────────

function FetchModelsButton({
  form,
  setForm,
}: {
  form: ChannelFormData
  setForm: (f: ChannelFormData) => void
}) {
  const [loading, setLoading] = useState(false)

  const baseUrl = form.baseUrls[0]?.url?.trim()
  const key = form.keys[0]?.channelKey?.trim()
  const canFetch = !!baseUrl && !!key

  async function handleFetch() {
    if (!canFetch) {
      toast.error("Please fill in Base URL and API Key first")
      return
    }
    setLoading(true)
    try {
      const res = await fetchChannelModelsPreview({
        type: form.type,
        baseUrl,
        key,
      })
      const models = res.data.models
      if (models.length === 0) {
        toast.info("No models found from this provider")
        return
      }
      setForm({ ...form, model: models })
      toast.success(`Fetched ${models.length} models`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to fetch models")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button variant="outline" size="sm" onClick={handleFetch} disabled={!canFetch || loading}>
      {loading ? (
        <Loader2 className="mr-1 h-3 w-3 animate-spin" />
      ) : (
        <Download className="mr-1 h-3 w-3" />
      )}
      {loading ? "Fetching..." : "Fetch Models"}
    </Button>
  )
}

// ─── Group Dialog ──────────────────────────────

function GroupDialog({
  open,
  onOpenChange,
  form,
  setForm,
  channelOptions,
  onSave,
  isPending,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  form: GroupFormData
  setForm: (f: GroupFormData) => void
  channelOptions: { id: number; name: string; model: string[] }[]
  onSave: () => void
  isPending: boolean
}) {
  const [modelPickerOpen, setModelPickerOpen] = useState(false)
  // null = closed, -1 = adding new item, >= 0 = editing item at index
  const [editingItemIndex, setEditingItemIndex] = useState<number | null>(null)

  // Build channel→models mapping for the item model picker
  const channelModels = useMemo(() => {
    return channelOptions
      .filter((ch) => ch.model && ch.model.length > 0)
      .map((ch) => ({
        channelId: ch.id,
        channelName: ch.name,
        models: ch.model.filter(Boolean).sort(),
      }))
  }, [channelOptions])

  function updateItem(index: number, patch: Partial<GroupItemForm>) {
    const items = [...form.items]
    items[index] = { ...items[index], ...patch }
    setForm({ ...form, items })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{form.id ? "Edit Group" : "Create Group"}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-2">
            <Label>Name</Label>
            <div className="relative">
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. claude-opus-4-6"
                className="pr-28"
              />
              <button
                type="button"
                className="text-muted-foreground hover:text-accent-foreground hover:bg-accent absolute top-1/2 right-1.5 inline-flex -translate-y-1/2 items-center rounded-md px-2 py-1 text-xs transition-colors"
                onClick={() => setModelPickerOpen(true)}
              >
                <Search className="mr-1 h-3 w-3" />
                models.dev
              </button>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Label>Load Balancing Mode</Label>
            <Select
              value={String(form.mode)}
              onValueChange={(v) => setForm({ ...form, mode: Number(v) })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(MODE_LABELS).map(([val, label]) => (
                  <SelectItem key={val} value={val}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label>Model Match Regex</Label>
            <Input
              value={form.matchRegex}
              onChange={(e) => setForm({ ...form, matchRegex: e.target.value })}
              placeholder="^gpt-4.*"
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>First Token Timeout (seconds)</Label>
            <Input
              type="number"
              value={form.firstTokenTimeOut}
              onChange={(e) =>
                setForm({
                  ...form,
                  firstTokenTimeOut: Number(e.target.value),
                })
              }
            />
          </div>

          {/* Group Items */}
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <Label>Models</Label>
              <Button variant="outline" size="sm" onClick={() => setEditingItemIndex(-1)}>
                <Plus className="mr-1 h-3 w-3" /> Add
              </Button>
            </div>
            {form.items.length === 0 && (
              <p className="text-muted-foreground text-sm">
                No models added yet. Click Add or drag from the left panel.
              </p>
            )}
            {/* eslint-disable react/no-array-index-key -- items may have duplicate channelId+modelName */}
            {form.items.map((item, i) => (
              <div
                key={`${item.channelId}-${item.modelName}-${i}`}
                className="flex items-center gap-2 rounded-md border p-2"
              >
                <button
                  type="button"
                  onClick={() => setEditingItemIndex(i)}
                  className="shrink-0 cursor-pointer"
                >
                  <ModelCard modelId={item.modelName || "(empty)"} />
                </button>
                <Select
                  value={item.channelId ? String(item.channelId) : ""}
                  onValueChange={(v) => updateItem(i, { channelId: Number(v) })}
                >
                  <SelectTrigger className="w-36">
                    <SelectValue placeholder="Channel" />
                  </SelectTrigger>
                  <SelectContent>
                    {channelOptions.map((ch) => (
                      <SelectItem key={ch.id} value={String(ch.id)}>
                        {ch.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {form.mode === 3 && (
                  <Input
                    className="w-20"
                    type="number"
                    placeholder="Priority"
                    value={item.priority}
                    onChange={(e) => updateItem(i, { priority: Number(e.target.value) })}
                  />
                )}
                {form.mode === 4 && (
                  <Input
                    className="w-20"
                    type="number"
                    placeholder="Weight"
                    value={item.weight}
                    onChange={(e) => updateItem(i, { weight: Number(e.target.value) })}
                  />
                )}
                <Button
                  variant="ghost"
                  size="icon"
                  className="ml-auto shrink-0"
                  onClick={() =>
                    setForm({
                      ...form,
                      items: form.items.filter((_, j) => j !== i),
                    })
                  }
                >
                  <X className="h-4 w-4" />
                </Button>
              </div>
            ))}
            {/* eslint-enable react/no-array-index-key */}
          </div>

          <Button className="mt-2" onClick={onSave} disabled={isPending || !form.name}>
            {isPending ? "Saving..." : "Save"}
          </Button>
        </div>
      </DialogContent>

      <ModelPickerDialog
        open={modelPickerOpen}
        onOpenChange={setModelPickerOpen}
        onSelect={(modelId: string) => {
          setForm({ ...form, name: modelId })
          setModelPickerOpen(false)
        }}
      />

      <ChannelModelPickerDialog
        open={editingItemIndex !== null}
        onOpenChange={(open) => {
          if (!open) setEditingItemIndex(null)
        }}
        channelModels={channelModels}
        onSelect={(channelId: number, modelId: string) => {
          if (editingItemIndex !== null && editingItemIndex >= 0) {
            // Editing existing item — replace model and channel
            updateItem(editingItemIndex, { modelName: modelId, channelId })
          } else {
            // Adding new item
            setForm({
              ...form,
              items: [...form.items, { channelId, modelName: modelId, priority: 0, weight: 1 }],
            })
          }
          setEditingItemIndex(null)
        }}
      />
    </Dialog>
  )
}

// ─── Model Picker Dialog (models.dev) ──────────────

function ModelPickerDialog({
  open,
  onOpenChange,
  onSelect,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelect: (modelId: string) => void
}) {
  const { data, isLoading } = useModelMetadataQuery()
  const [search, setSearch] = useState("")

  const grouped = useMemo(() => {
    const allModels = data?.data
    if (!allModels) return {}
    const q = search.toLowerCase()
    const result: Record<string, { id: string; name: string; logoUrl: string }[]> = {}
    for (const [id, meta] of Object.entries(allModels)) {
      if (
        q &&
        !id.toLowerCase().includes(q) &&
        !meta.name.toLowerCase().includes(q) &&
        !meta.providerName.toLowerCase().includes(q)
      )
        continue
      const provider = meta.providerName || "Other"
      if (!result[provider]) result[provider] = []
      result[provider].push({ id, name: meta.name, logoUrl: meta.logoUrl })
    }
    // sort providers alphabetically, models alphabetically within
    for (const models of Object.values(result)) {
      models.sort((a, b) => a.id.localeCompare(b.id))
    }
    return result
  }, [data, search])

  const providerKeys = useMemo(() => Object.keys(grouped).sort(), [grouped])

  const totalCount = useMemo(
    () => providerKeys.reduce((s, k) => s + grouped[k].length, 0),
    [grouped, providerKeys],
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg overflow-hidden">
        <DialogHeader>
          <DialogTitle>Select Model from models.dev</DialogTitle>
        </DialogHeader>

        <div className="relative">
          <Search className="text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4" />
          <Input
            placeholder="Search models..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>

        <ScrollArea className="h-[50vh]">
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="text-muted-foreground h-5 w-5 animate-spin" />
            </div>
          ) : totalCount === 0 ? (
            <p className="text-muted-foreground py-8 text-center text-sm">No models found</p>
          ) : (
            <div className="flex flex-col gap-3 pr-3">
              {providerKeys.map((provider) => (
                <div key={provider}>
                  <p className="text-muted-foreground mb-1.5 px-1 text-xs font-semibold">
                    {provider}
                    <span className="text-muted-foreground/60 ml-1">
                      ({grouped[provider].length})
                    </span>
                  </p>
                  <div className="flex flex-col gap-0.5">
                    {grouped[provider].map((m) => (
                      <button
                        key={m.id}
                        type="button"
                        className="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors"
                        onClick={() => onSelect(m.id)}
                      >
                        <img
                          src={m.logoUrl}
                          alt=""
                          width={16}
                          height={16}
                          className="shrink-0 dark:invert"
                          onError={(e) => {
                            ;(e.target as HTMLImageElement).style.display = "none"
                          }}
                        />
                        <span className="truncate">{m.id}</span>
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

// ─── Channel Model Picker Dialog ────────────────

interface ChannelModelEntry {
  channelId: number
  channelName: string
  models: string[]
}

function ChannelModelPickerDialog({
  open,
  onOpenChange,
  channelModels,
  onSelect,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelModels: ChannelModelEntry[]
  onSelect: (channelId: number, modelId: string) => void
}) {
  const { data } = useModelMetadataQuery()
  const [search, setSearch] = useState("")

  // Flatten channel→model pairs, group by provider
  const grouped = useMemo(() => {
    const rawMap = data?.data
    const q = search.toLowerCase()
    const result: Record<
      string,
      { modelId: string; channelId: number; channelName: string; logoUrl: string }[]
    > = {}

    for (const ch of channelModels) {
      for (const modelId of ch.models) {
        const meta = rawMap ? fuzzyLookup(rawMap, modelId) : null
        const providerName = meta?.providerName || "Other"

        if (q) {
          const idMatch = modelId.toLowerCase().includes(q)
          const nameMatch = meta?.name.toLowerCase().includes(q)
          const providerMatch = providerName.toLowerCase().includes(q)
          const channelMatch = ch.channelName.toLowerCase().includes(q)
          if (!idMatch && !nameMatch && !providerMatch && !channelMatch) continue
        }

        if (!result[providerName]) result[providerName] = []
        result[providerName].push({
          modelId,
          channelId: ch.channelId,
          channelName: ch.channelName,
          logoUrl: meta?.logoUrl || "",
        })
      }
    }

    for (const items of Object.values(result)) {
      items.sort((a, b) => a.modelId.localeCompare(b.modelId))
    }
    return result
  }, [data, channelModels, search])

  const providerKeys = useMemo(
    () =>
      Object.keys(grouped)
        .sort()
        .sort((a, b) => (a === "Other" ? 1 : b === "Other" ? -1 : 0)),
    [grouped],
  )

  const totalCount = useMemo(
    () => providerKeys.reduce((s, k) => s + grouped[k].length, 0),
    [grouped, providerKeys],
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg overflow-hidden">
        <DialogHeader>
          <DialogTitle>Select Model</DialogTitle>
        </DialogHeader>

        <div className="relative">
          <Search className="text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4" />
          <Input
            placeholder="Search models..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>

        <ScrollArea className="h-[50vh]">
          {totalCount === 0 ? (
            <p className="text-muted-foreground py-8 text-center text-sm">
              {channelModels.length === 0
                ? "No channels have models configured"
                : "No models found"}
            </p>
          ) : (
            <div className="flex flex-col gap-3 pr-3">
              {providerKeys.map((provider) => (
                <div key={provider}>
                  <p className="text-muted-foreground mb-1.5 px-1 text-xs font-semibold">
                    {provider}
                    <span className="text-muted-foreground/60 ml-1">
                      ({grouped[provider].length})
                    </span>
                  </p>
                  <div className="flex flex-col gap-0.5">
                    {grouped[provider].map((m) => (
                      <button
                        key={`${m.channelId}-${m.modelId}`}
                        type="button"
                        className="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors"
                        onClick={() => onSelect(m.channelId, m.modelId)}
                      >
                        {m.logoUrl && (
                          <img
                            src={m.logoUrl}
                            alt=""
                            width={16}
                            height={16}
                            className="shrink-0 dark:invert"
                            onError={(e) => {
                              ;(e.target as HTMLImageElement).style.display = "none"
                            }}
                          />
                        )}
                        <span className="flex-1 truncate">{m.modelId}</span>
                        <span className="text-muted-foreground shrink-0 text-xs">
                          {m.channelName}
                        </span>
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
