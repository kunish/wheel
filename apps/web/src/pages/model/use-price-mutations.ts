import type { PriceFormData } from "@/pages/model/price-dialog"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import {
  createModelPrice,
  deleteModelPrice,
  syncModelPrices,
  updateModelPrice,
} from "@/lib/api-client"

export function usePriceMutations(callbacks?: {
  onCreateSuccess?: () => void
  onUpdateSuccess?: () => void
}) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()

  const invalidatePrices = () => queryClient.invalidateQueries({ queryKey: ["model-prices"] })

  const syncPriceMut = useMutation({
    mutationFn: syncModelPrices,
    onSuccess: () => {
      invalidatePrices()
      queryClient.invalidateQueries({ queryKey: ["price-update-time"] })
      queryClient.invalidateQueries({ queryKey: ["profiles"] })
      toast.success(t("toast.syncSuccess"))
    },
    onError: () => toast.error(t("toast.syncFailed")),
  })

  const createPriceMut = useMutation({
    mutationFn: (form: PriceFormData) =>
      createModelPrice({
        name: form.name,
        inputPrice: Number.parseFloat(form.inputPrice),
        outputPrice: Number.parseFloat(form.outputPrice),
      }),
    onSuccess: () => {
      invalidatePrices()
      callbacks?.onCreateSuccess?.()
      toast.success(t("toast.priceCreated"))
    },
    onError: () => toast.error(t("toast.createFailed")),
  })

  const updatePriceMut = useMutation({
    mutationFn: ({ id, form }: { id: number; form: PriceFormData }) =>
      updateModelPrice({
        id,
        name: form.name,
        inputPrice: Number.parseFloat(form.inputPrice),
        outputPrice: Number.parseFloat(form.outputPrice),
      }),
    onSuccess: () => {
      invalidatePrices()
      callbacks?.onUpdateSuccess?.()
      toast.success(t("toast.priceUpdated"))
    },
    onError: () => toast.error(t("toast.updateFailed")),
  })

  const deletePriceMut = useMutation({
    mutationFn: (name: string) => deleteModelPrice({ name }),
    onSuccess: () => {
      invalidatePrices()
      toast.success(t("toast.priceDeleted"))
    },
    onError: () => toast.error(t("toast.deleteFailed")),
  })

  return {
    syncPriceMut,
    createPriceMut,
    updatePriceMut,
    deletePriceMut,
  }
}
