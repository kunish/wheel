import type { GroupFormData, GroupItemForm } from "@/pages/model/group-dialog"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { createGroup, deleteGroup, reorderGroups, updateGroup } from "@/lib/api-client"
import { defaultMutationCallbacks } from "@/lib/mutation-utils"

interface GroupRecord {
  id: number
  name: string
  mode: number
  firstTokenTimeOut: number
  sessionKeepTime: number
  order: number
  items: GroupItemForm[]
}

export function useGroupMutations(callbacks?: {
  activeProfileId?: number
  onSaveSuccess?: () => void
  groupForm?: GroupFormData
}) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()

  const invalidateGroupQueries = () => {
    queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey[0]
        return key === "groups" || key === "profile-group-preview" || key === "profiles"
      },
    })
  }

  const groupDeleteMut = useMutation({
    mutationFn: deleteGroup,
    onSuccess: () => {
      invalidateGroupQueries()
      toast.success(t("toast.groupDeleted"))
    },
    ...defaultMutationCallbacks,
  })

  const groupSaveMut = useMutation({
    mutationFn: (data: GroupFormData) =>
      data.id ? updateGroup(data) : createGroup({ ...data, profileId: callbacks?.activeProfileId }),
    onSuccess: () => {
      invalidateGroupQueries()
      callbacks?.onSaveSuccess?.()
      toast.success(callbacks?.groupForm?.id ? t("toast.groupUpdated") : t("toast.groupCreated"))
    },
    ...defaultMutationCallbacks,
  })

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
      invalidateGroupQueries()
      toast.success(t("toast.channelAddedToGroup"))
    },
    ...defaultMutationCallbacks,
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
      invalidateGroupQueries()
      toast.success(t("toast.groupCleared"))
    },
    ...defaultMutationCallbacks,
  })

  const groupRemoveItemMut = useMutation({
    mutationFn: (data: { group: GroupRecord; itemIndex: number }) => {
      const items = data.group.items.filter((_, i) => i !== data.itemIndex)
      return updateGroup({
        id: data.group.id,
        name: data.group.name,
        mode: data.group.mode,
        firstTokenTimeOut: data.group.firstTokenTimeOut,
        sessionKeepTime: data.group.sessionKeepTime ?? 0,
        items,
      })
    },
    onSuccess: () => {
      invalidateGroupQueries()
    },
    ...defaultMutationCallbacks,
  })

  const groupReorderMut = useMutation({
    mutationFn: reorderGroups,
    onSuccess: () => invalidateGroupQueries(),
    ...defaultMutationCallbacks,
  })

  return {
    groupDeleteMut,
    groupSaveMut,
    groupAddItemMut,
    groupClearMut,
    groupRemoveItemMut,
    groupReorderMut,
    invalidateGroupQueries,
  }
}
