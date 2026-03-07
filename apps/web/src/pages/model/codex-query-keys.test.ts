import { describe, expect, it } from "vitest"
import {
  channelsQueryKey,
  codexAuthFilesQueryKey,
  codexQuotaQueryKey,
  codexUploadRefreshQueryKeys,
} from "./codex-query-keys"

describe("codex query keys", () => {
  it("includes pagination inputs in the auth-files query key", () => {
    expect(
      codexAuthFilesQueryKey(42, { page: 2, pageSize: 25, search: "team", channelType: 34 }),
    ).toEqual(["codex-auth-files", 42, { page: 2, pageSize: 25, search: "team", channelType: 34 }])
  })

  it("includes pagination inputs in the quota query key", () => {
    expect(
      codexQuotaQueryKey(42, { page: 3, pageSize: 12, search: "copilot", channelType: 34 }),
    ).toEqual(["codex-quota", 42, { page: 3, pageSize: 12, search: "copilot", channelType: 34 }])
  })

  it("includes auth-files, quota, and channel queries after upload", () => {
    expect(codexUploadRefreshQueryKeys(42)).toEqual([
      codexAuthFilesQueryKey(42),
      codexQuotaQueryKey(42),
      channelsQueryKey,
    ])
  })
})
