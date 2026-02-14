import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

// ───────────── Types ─────────────

export interface PriceFormData {
  name: string
  inputPrice: string
  outputPrice: string
}

export const EMPTY_PRICE_FORM: PriceFormData = {
  name: "",
  inputPrice: "",
  outputPrice: "",
}

// ───────────── Price Form ─────────────

function PriceForm({
  form,
  onChange,
  onSubmit,
  isPending,
  submitLabel,
  nameReadonly,
}: {
  form: PriceFormData
  onChange: (f: PriceFormData) => void
  onSubmit: () => void
  isPending: boolean
  submitLabel: string
  nameReadonly?: boolean
}) {
  const { t } = useTranslation("model")
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-4"
    >
      <div className="flex flex-col gap-2">
        <Label>{t("price.form.modelName")}</Label>
        <Input
          value={form.name}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          placeholder="gpt-4o"
          required
          readOnly={nameReadonly}
          className={nameReadonly ? "bg-muted" : ""}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("price.form.inputPrice")}</Label>
        <Input
          type="number"
          step="0.000001"
          min="0"
          value={form.inputPrice}
          onChange={(e) => onChange({ ...form, inputPrice: e.target.value })}
          placeholder="0.000000"
          required
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("price.form.outputPrice")}</Label>
        <Input
          type="number"
          step="0.000001"
          min="0"
          value={form.outputPrice}
          onChange={(e) => onChange({ ...form, outputPrice: e.target.value })}
          placeholder="0.000000"
          required
        />
      </div>
      <Button type="submit" disabled={isPending}>
        {submitLabel}
      </Button>
    </form>
  )
}

// ───────────── Price Dialog ─────────────

interface PriceDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  form: PriceFormData
  onChange: (f: PriceFormData) => void
  onSubmit: () => void
  isPending: boolean
  title: string
  submitLabel: string
  nameReadonly?: boolean
}

export default function PriceDialog({
  open,
  onOpenChange,
  form,
  onChange,
  onSubmit,
  isPending,
  title,
  submitLabel,
  nameReadonly,
}: PriceDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <PriceForm
          form={form}
          onChange={onChange}
          onSubmit={onSubmit}
          isPending={isPending}
          submitLabel={submitLabel}
          nameReadonly={nameReadonly}
        />
      </DialogContent>
    </Dialog>
  )
}
