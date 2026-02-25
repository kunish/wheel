import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useCallback } from "react"
import { toast } from "sonner"
import { activateProfile } from "@/lib/api-client"

export function useProfileMutations(callbacks?: { onActivateSuccess?: () => void }) {
  const queryClient = useQueryClient()

  const activateProfileMut = useMutation({
    mutationFn: (id: number) => activateProfile(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] })
      callbacks?.onActivateSuccess?.()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const setActiveProfileId = useCallback(
    (id: number | undefined) => {
      activateProfileMut.mutate(id ?? 0)
    },
    [activateProfileMut],
  )

  return {
    activateProfileMut,
    setActiveProfileId,
  }
}
