import type { ChannelFormData } from "@/pages/model/channel-dialog"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import {
  createChannel,
  deleteChannel,
  enableChannel,
  reorderChannels,
  updateChannel,
} from "@/lib/api-client"
import { defaultMutationCallbacks } from "@/lib/mutation-utils"

export function useChannelMutations(callbacks?: {
  onSaveSuccess?: () => void
  channelForm?: ChannelFormData
}) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()

  const invalidateChannels = () => queryClient.invalidateQueries({ queryKey: ["channels"] })

  const channelDeleteMut = useMutation({
    mutationFn: deleteChannel,
    onSuccess: () => {
      invalidateChannels()
      toast.success(t("toast.channelDeleted"))
    },
    ...defaultMutationCallbacks,
  })

  const channelEnableMut = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => enableChannel(id, enabled),
    onMutate: async ({ id, enabled }) => {
      await queryClient.cancelQueries({ queryKey: ["channels"] })
      const prev = queryClient.getQueryData(["channels"])
      queryClient.setQueryData(["channels"], (old: any) => {
        if (!old?.data?.channels) return old
        return {
          ...old,
          data: {
            ...old.data,
            channels: old.data.channels.map((ch: any) => (ch.id === id ? { ...ch, enabled } : ch)),
          },
        }
      })
      return { prev }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(["channels"], context.prev)
      toast.error(_err.message)
    },
    onSettled: invalidateChannels,
  })

  const channelSaveMut = useMutation({
    mutationFn: (data: ChannelFormData) => (data.id ? updateChannel(data) : createChannel(data)),
    onSuccess: () => {
      invalidateChannels()
      callbacks?.onSaveSuccess?.()
      toast.success(
        callbacks?.channelForm?.id ? t("toast.channelUpdated") : t("toast.channelCreated"),
      )
    },
    ...defaultMutationCallbacks,
  })

  const channelReorderMut = useMutation({
    mutationFn: reorderChannels,
    onSuccess: invalidateChannels,
    ...defaultMutationCallbacks,
  })

  return {
    channelDeleteMut,
    channelEnableMut,
    channelSaveMut,
    channelReorderMut,
  }
}
