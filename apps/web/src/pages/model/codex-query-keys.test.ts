import { describe, expect, it } from "vitest"
import {
  channelsQueryKey,
  codexAuthFilesQueryKey,
  codexQuotaQueryKey,
  codexUploadRefreshQueryKeys,
} from "./codex-query-keys"

describe("codex query keys", () => {
  it("includes auth-files, quota, and channel queries after upload", () => {
    expect(codexUploadRefreshQueryKeys(42)).toEqual([
      codexAuthFilesQueryKey(42),
      codexQuotaQueryKey(42),
      channelsQueryKey,
    ])
  })
})
