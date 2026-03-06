import type { CodexAuthUploadBatchResult } from "./codex"
import { describe, expect, it } from "vitest"
import { buildCodexAuthUploadFormData, getCodexAuthUploadToastState } from "./codex"

describe("codex api helpers", () => {
  it("appends every selected file under repeated files fields", () => {
    const files = [
      new File(["a"], "first.json", { type: "application/json" }),
      new File(["b"], "second.json", { type: "application/json" }),
    ]

    const formData = buildCodexAuthUploadFormData(files)
    const appended = formData.getAll("files")

    expect(appended).toHaveLength(2)
    expect(appended[0]).toBe(files[0])
    expect(appended[1]).toBe(files[1])
    expect(formData.getAll("file")).toEqual([])
  })

  it("returns success toast state for fully successful batches", () => {
    const result: CodexAuthUploadBatchResult = {
      total: 2,
      successCount: 2,
      failedCount: 0,
      results: [
        { name: "a.json", status: "ok" },
        { name: "b.json", status: "ok" },
      ],
    }

    expect(getCodexAuthUploadToastState(result)).toEqual({
      level: "success",
      key: "codex.uploadSummarySuccess",
      values: { total: 2, successCount: 2, failedCount: 0 },
    })
  })

  it("returns partial toast state for mixed batch results", () => {
    const result: CodexAuthUploadBatchResult = {
      total: 3,
      successCount: 2,
      failedCount: 1,
      results: [
        { name: "a.json", status: "ok" },
        { name: "b.json", status: "error", error: "invalid auth file json" },
        { name: "c.json", status: "ok" },
      ],
    }

    expect(getCodexAuthUploadToastState(result)).toEqual({
      level: "info",
      key: "codex.uploadSummaryPartial",
      values: { total: 3, successCount: 2, failedCount: 1 },
    })
  })

  it("returns error toast state when the whole batch fails", () => {
    const result: CodexAuthUploadBatchResult = {
      total: 2,
      successCount: 0,
      failedCount: 2,
      results: [
        { name: "a.json", status: "error", error: "invalid auth file json" },
        { name: "b.json", status: "error", error: "duplicate auth file" },
      ],
    }

    expect(getCodexAuthUploadToastState(result)).toEqual({
      level: "error",
      key: "codex.uploadSummaryError",
      values: { total: 2, successCount: 0, failedCount: 2 },
    })
  })
})
