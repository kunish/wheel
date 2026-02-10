"use client"

import { Download, FileUp, Upload } from "lucide-react"
import { useCallback, useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { exportData, importData } from "@/lib/api"

interface ImportResult {
  channels?: { added: number; skipped: number }
  groups?: { added: number; skipped: number }
  groupItems?: { added: number; skipped: number }
  apiKeys?: { added: number; skipped: number }
  settings?: { added: number; skipped: number }
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export default function BackupSection() {
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
      toast.success("Export downloaded")
    } catch {
      toast.error("Failed to export data")
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
      if (result.success) {
        setImportResult(result.data as ImportResult)
        toast.success("Import completed")
      } else {
        toast.error(result.error ?? "Import failed")
      }
    } catch {
      toast.error("Failed to import data")
    } finally {
      setImporting(false)
    }
  }

  const handleFileSelect = useCallback((file: File) => {
    if (!file.name.endsWith(".json")) {
      toast.error("Only .json files are supported")
      return
    }
    setSelectedFile(file)
    setImportResult(null)
  }, [])

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
          <CardTitle>Export Data</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-muted-foreground text-sm">
            Export all configuration data as a JSON file.
          </p>

          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <Switch id="include-logs" checked={includeLogs} onCheckedChange={setIncludeLogs} />
              <Label htmlFor="include-logs">Include request logs</Label>
            </div>
            <div className="flex items-center gap-2">
              <Switch id="include-stats" checked={includeStats} onCheckedChange={setIncludeStats} />
              <Label htmlFor="include-stats">Include statistics data</Label>
            </div>
          </div>

          <Button onClick={handleExport} disabled={exporting} className="w-fit">
            <Download className="mr-2 h-4 w-4" />
            {exporting ? "Exporting..." : "Download Export"}
          </Button>
        </CardContent>
      </Card>

      {/* Import */}
      <Card>
        <CardHeader>
          <CardTitle>Import Data</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-muted-foreground text-sm">
            Import configuration from a JSON file. Existing data will not be overwritten.
          </p>

          <div
            className={`relative flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-8 transition-colors ${
              isDragOver
                ? "border-primary bg-primary/5"
                : "border-muted-foreground/25 hover:border-muted-foreground/50"
            }`}
            onClick={() => fileInputRef.current?.click()}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
          >
            <FileUp className="text-muted-foreground h-8 w-8" />
            <p className="text-muted-foreground text-sm">Drop file here or click to select</p>
            <p className="text-muted-foreground text-xs">Supports .json files</p>
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
              Selected: <span className="text-foreground font-medium">{selectedFile.name}</span> (
              {formatFileSize(selectedFile.size)})
            </p>
          )}

          <Button onClick={handleImport} disabled={!selectedFile || importing} className="w-fit">
            <Upload className="mr-2 h-4 w-4" />
            {importing ? "Importing..." : "Import Data"}
          </Button>

          {importResult && (
            <div className="rounded-md border p-4">
              <p className="mb-2 text-sm font-medium">Import Result:</p>
              <ul className="text-muted-foreground flex flex-col gap-1 text-sm">
                {Object.entries(importResult).map(([key, value]) => (
                  <li key={key}>
                    {key}: {value.added} added, {value.skipped} skipped
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
