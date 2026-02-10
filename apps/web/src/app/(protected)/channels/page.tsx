"use client"

import type { DragEndEvent, DragStartEvent } from "@dnd-kit/core"
import type { ChannelFormData } from "./_components/channel-dialog"
import type { GroupFormData, GroupItemForm } from "./_components/group-dialog"
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
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ChevronDown,
  ChevronsDownUp,
  ChevronsUpDown,
  GripVertical,
  List,
  Pencil,
  Plus,
  Trash2,
  X,
} from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import dynamic from "next/dynamic"
import { useRouter, useSearchParams } from "next/navigation"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { toast } from "sonner"
import { GroupedModelList } from "@/components/grouped-model-list"
import { ModelCard } from "@/components/model-card"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Switch } from "@/components/ui/switch"
import { fuzzyLookup, useModelMetadataQuery } from "@/hooks/use-model-meta"
import {
  createChannel,
  createGroup,
  deleteChannel,
  deleteGroup,
  enableChannel,
  getModelList,
  listChannels,
  listGroups,
  reorderGroups,
  updateChannel,
  updateGroup,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { EMPTY_CHANNEL_FORM } from "./_components/channel-dialog"
import { EMPTY_GROUP_FORM } from "./_components/group-dialog"

// ─── Lazy-loaded Dialog components ──────────────

const ChannelDialog = dynamic(() => import("./_components/channel-dialog"), {
  loading: () => null,
})

const GroupDialog = dynamic(() => import("./_components/group-dialog"), {
  loading: () => null,
})

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

export default function ChannelsAndGroupsPage() {
  const queryClient = useQueryClient()
  const searchParams = useSearchParams()
  const router = useRouter()

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

  // Delete confirmation state
  const [deleteChannelConfirm, setDeleteChannelConfirm] = useState<ChannelRecord | null>(null)
  const [deleteGroupConfirm, setDeleteGroupConfirm] = useState<GroupRecord | null>(null)
  const [clearGroupConfirm, setClearGroupConfirm] = useState<GroupRecord | null>(null)

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

  const channels = useMemo(
    () => (channelData?.data?.channels ?? []) as ChannelRecord[],
    [channelData],
  )
  const groups = useMemo(() => (groupData?.data?.groups ?? []) as GroupRecord[], [groupData])
  const models = useMemo(() => (modelData?.data?.models ?? []) as string[], [modelData])

  // ─── Highlight scroll logic ───────────────────
  const setChannelRef = useCallback((id: number, el: HTMLDivElement | null) => {
    if (el) channelRefsRef.current.set(id, el)
    else channelRefsRef.current.delete(id)
  }, [])

  useEffect(() => {
    if (!highlightChannelId || channelsLoading || channels.length === 0) return
    // Verify channel exists
    if (!channels.some((ch) => ch.id === highlightChannelId)) return

    // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect -- responding to URL param change
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
      router.replace(query ? `?${query}` : window.location.pathname, { scroll: false })
    }, 3000)
    return () => clearTimeout(timer)
  }, [highlightChannelId, channelsLoading, channels, searchParams, router])

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
        firstTokenTimeOut: data.group.firstTokenTimeOut,
        sessionKeepTime: data.group.sessionKeepTime ?? 0,
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
        firstTokenTimeOut: group.firstTokenTimeOut,
        items: [],
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["groups"] })
      toast.success("Group cleared")
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const groupReorderMut = useMutation({
    mutationFn: reorderGroups,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["groups"] }),
    onError: (err: Error) => toast.error(err.message),
  })

  // ─── Drag handlers ─────────────────────────────

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
  )

  function handleDragStart(event: DragStartEvent) {
    setActiveDrag(event.active.data.current as DragData)
  }

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event
    setActiveDrag(null)

    if (!over) return

    const dragData = active.data.current as DragData

    // ─── Group reorder ─────────────────────────
    if (dragData.type === "group") {
      const activeId = String(active.id)
      const overId = String(over.id)
      if (activeId === overId) return

      // Both must be sortable-group-* ids
      if (!activeId.startsWith("sortable-group-") || !overId.startsWith("sortable-group-")) return

      const oldIndex = groups.findIndex((g) => `sortable-group-${g.id}` === activeId)
      const newIndex = groups.findIndex((g) => `sortable-group-${g.id}` === overId)
      if (oldIndex === -1 || newIndex === -1 || oldIndex === newIndex) return

      const reordered = arrayMove(groups, oldIndex, newIndex)
      groupReorderMut.mutate(reordered.map((g) => g.id))
      return
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
      firstTokenTimeOut: g.firstTokenTimeOut,
      sessionKeepTime: g.sessionKeepTime ?? 0,
      items: g.items ?? [],
    })
    setGroupDialogOpen(true)
  }

  const channelMap = useMemo(() => new Map(channels.map((ch) => [ch.id, ch])), [channels])
  const groupIds = useMemo(() => groups.map((g) => `sortable-group-${g.id}`), [groups])

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
              <div className="flex items-center gap-1">
                {channels.length > 0 && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-9 w-9"
                    onClick={() => setChannelsCollapsed((v) => !v)}
                    title={channelsCollapsed ? "Expand all" : "Collapse all"}
                  >
                    {channelsCollapsed ? (
                      <ChevronsUpDown className="h-4 w-4" />
                    ) : (
                      <ChevronsDownUp className="h-4 w-4" />
                    )}
                  </Button>
                )}
                <Button size="sm" onClick={openCreateChannel}>
                  <Plus className="mr-1 h-3 w-3" /> Add
                </Button>
              </div>
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
                  <AnimatePresence initial={false}>
                    {channels.map((ch) => (
                      <motion.div
                        key={ch.id}
                        initial={{ opacity: 0, scale: 0.95 }}
                        animate={{ opacity: 1, scale: 1 }}
                        exit={{ opacity: 0, scale: 0.95 }}
                        transition={{ duration: 0.2 }}
                      >
                        <DraggableChannel
                          channel={ch}
                          highlighted={highlightedId === ch.id}
                          refCallback={(el) => setChannelRef(ch.id, el)}
                          onEdit={() => openEditChannel(ch)}
                          onDelete={() => setDeleteChannelConfirm(ch)}
                          onToggle={(enabled) => channelEnableMut.mutate({ id: ch.id, enabled })}
                          enablePending={channelEnableMut.isPending}
                          forceCollapsed={channelsCollapsed}
                        />
                      </motion.div>
                    ))}
                  </AnimatePresence>
                </div>
              )}
            </ScrollArea>
          </div>

          {/* ─── Right: Groups ────────────────── */}
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold">Groups</h3>
              <div className="flex items-center gap-1">
                {groups.length > 0 && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-9 w-9"
                    onClick={() => setGroupsCollapsed((v) => !v)}
                    title={groupsCollapsed ? "Expand all" : "Collapse all"}
                  >
                    {groupsCollapsed ? (
                      <ChevronsUpDown className="h-4 w-4" />
                    ) : (
                      <ChevronsDownUp className="h-4 w-4" />
                    )}
                  </Button>
                )}
                <Button size="sm" onClick={openCreateGroup}>
                  <Plus className="mr-1 h-3 w-3" /> Add
                </Button>
              </div>
            </div>

            <ScrollArea className="flex-1">
              {groupsLoading ? (
                <p className="text-muted-foreground">Loading...</p>
              ) : groups.length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  No groups. Create one, then drag channels or models into it.
                </p>
              ) : (
                <SortableContext items={groupIds} strategy={verticalListSortingStrategy}>
                  <div className="flex flex-col gap-3">
                    <AnimatePresence initial={false}>
                      {groups.map((g) => (
                        <motion.div
                          key={g.id}
                          initial={{ opacity: 0, scale: 0.95 }}
                          animate={{ opacity: 1, scale: 1 }}
                          exit={{ opacity: 0, scale: 0.95 }}
                          transition={{ duration: 0.2 }}
                        >
                          <SortableGroup
                            group={g}
                            channelMap={channelMap}
                            onEdit={() => openEditGroup(g)}
                            onDelete={() => setDeleteGroupConfirm(g)}
                            onClear={() => setClearGroupConfirm(g)}
                            isOver={activeDrag !== null}
                            forceCollapsed={groupsCollapsed}
                          />
                        </motion.div>
                      ))}
                    </AnimatePresence>
                  </div>
                </SortableContext>
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
        {activeDrag?.type === "group" && (
          <Card className="w-64 cursor-grabbing opacity-90 shadow-lg">
            <CardHeader className="p-3">
              <CardTitle className="text-sm">
                {groups.find((g) => g.id === activeDrag.groupId)?.name}
              </CardTitle>
            </CardHeader>
          </Card>
        )}
      </DragOverlay>

      {/* ─── Delete Confirmations ────────────────── */}
      <AlertDialog
        open={!!deleteChannelConfirm}
        onOpenChange={(open) => !open && setDeleteChannelConfirm(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Delete channel &ldquo;{deleteChannelConfirm?.name}&rdquo;?
            </AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. The channel and its configuration will be permanently
              removed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                if (deleteChannelConfirm) channelDeleteMut.mutate(deleteChannelConfirm.id)
                setDeleteChannelConfirm(null)
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!deleteGroupConfirm}
        onOpenChange={(open) => !open && setDeleteGroupConfirm(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Delete group &ldquo;{deleteGroupConfirm?.name}&rdquo;?
            </AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. The group and all its routing rules will be permanently
              removed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                if (deleteGroupConfirm) groupDeleteMut.mutate(deleteGroupConfirm.id)
                setDeleteGroupConfirm(null)
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!clearGroupConfirm}
        onOpenChange={(open) => !open && setClearGroupConfirm(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Clear all items from &ldquo;{clearGroupConfirm?.name}&rdquo;?
            </AlertDialogTitle>
            <AlertDialogDescription>
              All channels and models will be removed from this group. The group itself will remain.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                if (clearGroupConfirm) groupClearMut.mutate(clearGroupConfirm)
                setClearGroupConfirm(null)
              }}
            >
              Clear All
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* ─── Channel Dialog ───────────────────── */}
      <ChannelDialog
        open={channelDialogOpen}
        onOpenChange={(open) => {
          setChannelDialogOpen(open)
          if (!open) setChannelForm(EMPTY_CHANNEL_FORM)
        }}
        form={channelForm}
        setForm={setChannelForm}
        onSave={() => channelSaveMut.mutate(channelForm)}
        isPending={channelSaveMut.isPending}
      />

      {/* ─── Group Dialog ─────────────────────── */}
      <GroupDialog
        open={groupDialogOpen}
        onOpenChange={(open) => {
          setGroupDialogOpen(open)
          if (!open) setGroupForm(EMPTY_GROUP_FORM)
        }}
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
  highlighted,
  refCallback,
  onEdit,
  onDelete,
  onToggle,
  enablePending,
  forceCollapsed,
}: {
  channel: ChannelRecord
  highlighted?: boolean
  refCallback?: (el: HTMLDivElement | null) => void
  onEdit: () => void
  onDelete: () => void
  onToggle: (enabled: boolean) => void
  enablePending?: boolean
  forceCollapsed?: boolean
}) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: `channel-${channel.id}`,
    data: { type: "channel", channel } satisfies DragDataChannel,
  })

  const modelNames = parseModels(channel.model)
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

  return (
    <Card
      ref={(node) => {
        setNodeRef(node)
        refCallback?.(node)
      }}
      className={cn(
        "transition-all",
        isDragging && "opacity-40",
        highlighted && "ring-primary animate-pulse ring-2",
      )}
    >
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
          <Switch checked={channel.enabled} onCheckedChange={onToggle} disabled={enablePending} />
          <Button variant="ghost" size="icon" className="h-9 w-9" onClick={onEdit}>
            <Pencil className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-9 w-9" onClick={onDelete}>
            <Trash2 className="text-destructive h-3 w-3" />
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

// ─── Sortable & Droppable Group Card ──────────

function SortableGroup({
  group,
  channelMap,
  onEdit,
  onDelete,
  onClear,
  isOver: dragActive,
  forceCollapsed,
}: {
  group: GroupRecord
  channelMap: Map<number, ChannelRecord>
  onEdit: () => void
  onDelete: () => void
  onClear: () => void
  isOver: boolean
  forceCollapsed?: boolean
}) {
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

  // Sync with parent's expand/collapse all toggle
  const prevForceCollapsedRef = useRef(forceCollapsed)
  if (prevForceCollapsedRef.current !== forceCollapsed) {
    prevForceCollapsedRef.current = forceCollapsed
    if (forceCollapsed !== undefined) setCollapsed(forceCollapsed)
  }

  const style = {
    transform: CSS.Transform.toString(transform),
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
        "transition-all",
        isOver && "ring-primary border-primary ring-2",
        dragActive && !isOver && "border-dashed",
        isDragging && "opacity-50",
      )}
    >
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
              <CardTitle className="text-sm">{group.name}</CardTitle>
              <div className="mt-1 flex items-center gap-1">
                <Badge variant="secondary" className="text-xs">
                  {MODE_LABELS[group.mode] ?? "Unknown"}
                </Badge>
                {collapsed && group.items.length > 0 && (
                  <span className="text-muted-foreground text-xs">
                    {group.items.length} item{group.items.length !== 1 ? "s" : ""}
                  </span>
                )}
              </div>
            </div>
          </button>
        </div>
        <div className="flex items-center gap-1">
          {group.items.length > 0 && (
            <Button
              variant="ghost"
              size="icon"
              className="h-9 w-9"
              onClick={onClear}
              title="Clear all"
            >
              <X className="h-3 w-3" />
            </Button>
          )}
          <Button variant="ghost" size="icon" className="h-9 w-9" onClick={onEdit}>
            <Pencil className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-9 w-9" onClick={onDelete}>
            <Trash2 className="text-destructive h-3 w-3" />
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
    return (
      <ModelCard
        key={`${item.channelId}-${item.modelName}-${idx}`}
        modelId={item.modelName}
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
          <div className="flex flex-wrap gap-1.5">
            {g.items.map((it, i) => renderModelCard(it, i))}
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
