import type { MCPClientInput } from "@/lib/api"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { createMCPClient, deleteMCPClient, reconnectMCPClient, updateMCPClient } from "@/lib/api"
import { defaultMutationCallbacks } from "@/lib/mutation-utils"

export function useMCPMutations(callbacks?: { onSaveSuccess?: () => void }) {
  const { t } = useTranslation("mcp")
  const queryClient = useQueryClient()

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["mcp-clients"] })

  const createMut = useMutation({
    mutationFn: (d: Omit<MCPClientInput, "id">) => createMCPClient(d),
    ...defaultMutationCallbacks,
    onSuccess: () => {
      invalidate()
      toast.success(t("toast.created"))
      callbacks?.onSaveSuccess?.()
    },
  })

  const updateMut = useMutation({
    mutationFn: (d: Partial<MCPClientInput> & { id: number }) => updateMCPClient(d),
    ...defaultMutationCallbacks,
    onSuccess: () => {
      invalidate()
      toast.success(t("toast.updated"))
      callbacks?.onSaveSuccess?.()
    },
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteMCPClient(id),
    ...defaultMutationCallbacks,
    onSuccess: () => {
      invalidate()
      toast.success(t("toast.deleted"))
    },
  })

  const reconnectMut = useMutation({
    mutationFn: (id: number) => reconnectMCPClient(id),
    ...defaultMutationCallbacks,
    onSuccess: () => {
      invalidate()
      toast.success(t("toast.reconnected"))
    },
  })

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      updateMCPClient({ id, enabled }),
    onMutate: async ({ id, enabled }) => {
      await queryClient.cancelQueries({ queryKey: ["mcp-clients"] })
      const prev = queryClient.getQueryData(["mcp-clients"])
      queryClient.setQueryData(["mcp-clients"], (old: any) => {
        if (!old?.data?.clients) return old
        return {
          ...old,
          data: {
            ...old.data,
            clients: old.data.clients.map((c: any) => (c.id === id ? { ...c, enabled } : c)),
          },
        }
      })
      return { prev }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(["mcp-clients"], context.prev)
      toast.error(t("toast.toggleFailed"))
    },
    onSettled: invalidate,
  })

  return { createMut, updateMut, deleteMut, reconnectMut, toggleMut }
}
