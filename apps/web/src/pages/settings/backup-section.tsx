import type { ImportDataResult } from "@/lib/api"
import { Download, FileUp, Upload } from "lucide-react"
import { useCallback, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { exportData, importData } from "@/lib/api"

type ImportResult = NonNullable<ImportDataResult["data"]>

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export default function BackupSection() {
  const { t } = useTranslation("settings")
  const [includeLogs, setIncludeLogs] = useState(false)
  const [includeStats, setIncludeStats] = useState(false)
  const [exporting, setExporting] = useState(false)

  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [importing, setImporting] = useState(false)
  const [importResult, setImportResult] = useState<ImportResult | null>(null)
  const [isDragOver, setIsDragOver] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleExport = async () => {
    setExporting(true)
    try {
      const resp = await exportData(includeLogs)
      if (!resp.ok) {
        throw new Error("Export failed")
      }
      const blob = await resp.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = `wheel-export-${new Date().toISOString().split("T")[0]}.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      toast.success(t("backup.exportDownloaded"))
    } catch {
      toast.error(t("backup.exportFailed"))
    } finally {
      setExporting(false)
    }
  }

  const handleImport = async () => {
    if (!selectedFile) return
    setImporting(true)
    setImportResult(null)
    try {
      const result = await importData(selectedFile)
      if (result.success && result.data) {
        setImportResult(result.data)
        toast.success(t("backup.importCompleted"))
      } else {
        toast.error(result.error ?? t("backup.importFailed"))
      }
    } catch {
      toast.error(t("backup.importDataFailed"))
    } finally {
      setImporting(false)
    }
  }

  const handleFileSelect = useCallback(
    (file: File) => {
      if (!file.name.endsWith(".json")) {
        toast.error(t("backup.onlyJsonSupported"))
        return
      }
      setSelectedFile(file)
      setImportResult(null)
    },
    [t],
  )

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setIsDragOver(true)
  }, [])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setIsDragOver(false)
  }, [])

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      setIsDragOver(false)
      const file = e.dataTransfer.files[0]
      if (file) handleFileSelect(file)
    },
    [handleFileSelect],
  )

  return (
    <div className="flex flex-col gap-6">
      {/* Export */}
      <Card>
        <CardHeader>
          <CardTitle>{t("backup.exportTitle")}</CardTitle>
          <CardDescription>{t("backup.exportDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <Switch id="include-logs" checked={includeLogs} onCheckedChange={setIncludeLogs} />
              <Label htmlFor="include-logs">{t("backup.includeRequestLogs")}</Label>
            </div>
            <div className="flex items-center gap-2">
              <Switch id="include-stats" checked={includeStats} onCheckedChange={setIncludeStats} />
              <Label htmlFor="include-stats">{t("backup.includeStatisticsData")}</Label>
            </div>
          </div>

          <Button onClick={handleExport} disabled={exporting} className="w-fit">
            <Download className="mr-2 h-4 w-4" />
            {exporting ? t("backup.exporting") : t("backup.downloadExport")}
          </Button>
        </CardContent>
      </Card>

      {/* Import */}
      <Card>
        <CardHeader>
          <CardTitle>{t("backup.importTitle")}</CardTitle>
          <CardDescription>{t("backup.importDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div
            className={`relative flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-8 transition-colors ${
              isDragOver
                ? "border-primary bg-primary/5"
                : "border-muted-foreground/25 hover:border-muted-foreground/50"
            }`}
            onClick={() => fileInputRef.current?.click()}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault()
                fileInputRef.current?.click()
              }
            }}
            role="button"
            tabIndex={0}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
          >
            <FileUp className="text-muted-foreground h-8 w-8" />
            <p className="text-muted-foreground text-sm">{t("backup.dropFileHere")}</p>
            <p className="text-muted-foreground text-xs">{t("backup.supportsJson")}</p>
            <input
              ref={fileInputRef}
              type="file"
              accept=".json"
              className="hidden"
              onChange={(e) => {
                const file = e.target.files?.[0]
                if (file) handleFileSelect(file)
              }}
            />
          </div>

          {selectedFile && (
            <p className="text-muted-foreground text-sm">
              {t("backup.selected")}{" "}
              <span className="text-foreground font-medium">{selectedFile.name}</span> (
              {formatFileSize(selectedFile.size)})
            </p>
          )}

          <Button onClick={handleImport} disabled={!selectedFile || importing} className="w-fit">
            <Upload className="mr-2 h-4 w-4" />
            {importing ? t("backup.importing") : t("backup.importData")}
          </Button>

          {importResult && (
            <div className="rounded-md border p-4">
              <p className="mb-2 text-sm font-medium">{t("backup.importResult")}</p>
              <ul className="text-muted-foreground flex flex-col gap-1 text-sm">
                {Object.entries(importResult).map(([key, value]) => (
                  <li key={key}>
                    {key}: {t("backup.added", { count: value.added })},{" "}
                    {t("backup.skipped", { count: value.skipped })}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
