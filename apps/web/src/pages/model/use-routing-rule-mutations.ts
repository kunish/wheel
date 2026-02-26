import type { RoutingRuleInput } from "@/lib/api-client"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { createRoutingRule, deleteRoutingRule, updateRoutingRule } from "@/lib/api-client"
import { defaultMutationCallbacks } from "@/lib/mutation-utils"

export function useRoutingRuleMutations(callbacks?: { onSaveSuccess?: () => void }) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["routing-rules"] })

  const createMut = useMutation({
    mutationFn: (d: Omit<RoutingRuleInput, "id">) => createRoutingRule(d),
    ...defaultMutationCallbacks,
    onSuccess: () => {
      invalidate()
      toast.success(t("routingRules.ruleCreated"))
      callbacks?.onSaveSuccess?.()
    },
  })

  const updateMut = useMutation({
    mutationFn: (d: Partial<RoutingRuleInput> & { id: number }) => updateRoutingRule(d),
    ...defaultMutationCallbacks,
    onSuccess: () => {
      invalidate()
      toast.success(t("routingRules.ruleUpdated"))
      callbacks?.onSaveSuccess?.()
    },
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteRoutingRule(id),
    ...defaultMutationCallbacks,
    onSuccess: () => {
      invalidate()
      toast.success(t("routingRules.ruleDeleted"))
    },
  })

  // Optimistic toggle for inline Switch
  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      updateRoutingRule({ id, enabled }),
    onMutate: async ({ id, enabled }) => {
      await queryClient.cancelQueries({ queryKey: ["routing-rules"] })
      const prev = queryClient.getQueryData(["routing-rules"])
      queryClient.setQueryData(["routing-rules"], (old: any) => {
        if (!old?.data?.rules) return old
        return {
          ...old,
          data: {
            ...old.data,
            rules: old.data.rules.map((r: any) => (r.id === id ? { ...r, enabled } : r)),
          },
        }
      })
      return { prev }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(["routing-rules"], context.prev)
      toast.error(t("routingRules.toggleFailed"))
    },
    onSettled: invalidate,
  })

  return { createMut, updateMut, deleteMut, toggleMut }
}
