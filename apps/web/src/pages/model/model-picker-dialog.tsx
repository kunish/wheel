import { motion } from "motion/react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useModelMetadataQuery } from "@/hooks/use-model-meta"
import { ModelPickerBase } from "./model-picker-base"

interface ModelPickerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelect: (modelId: string) => void
}

export default function ModelPickerDialog({
  open,
  onOpenChange,
  onSelect,
}: ModelPickerDialogProps) {
  const { t } = useTranslation("model")
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
    <ModelPickerBase
      open={open}
      onOpenChange={onOpenChange}
      title={t("modelPicker.title")}
      searchPlaceholder={t("modelPicker.searchPlaceholder")}
      emptyText={t("modelPicker.noModelsFound")}
      isLoading={isLoading}
      search={search}
      onSearchChange={setSearch}
      providerKeys={providerKeys}
      getProviderCount={(p) => grouped[p].length}
      totalCount={totalCount}
      renderProviderItems={(provider) =>
        grouped[provider].map((m) => (
          <motion.button
            key={m.id}
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.2 }}
            type="button"
            className="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 overflow-hidden rounded-md px-2 py-1.5 text-left text-sm transition-colors"
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
          </motion.button>
        ))
      }
    />
  )
}
