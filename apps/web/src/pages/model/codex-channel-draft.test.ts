import { describe, expect, it, vi } from "vitest"
import {
  adaptChannelDraftForType,
  ensureCodexChannelId,
  getRuntimeProviderKey,
  isRuntimeChannelType,
  mergeRuntimeChannelModels,
  RUNTIME_MANAGED_CHANNEL_KEY,
  shouldShowGenericModelFetch,
} from "./codex-channel-draft"

const baseForm = {
  name: "Codex Channel",
  type: 33,
  enabled: true,
  baseUrls: [{ url: "https://api.openai.com", delay: 0 }],
  keys: [{ channelKey: RUNTIME_MANAGED_CHANNEL_KEY, remark: "" }],
  model: [],
  fetchedModel: [],
  customModel: "",
  paramOverride: "",
}

describe("ensureCodexChannelId", () => {
  it("returns the existing channel id without saving again", async () => {
    const saveChannel = vi.fn()

    await expect(
      ensureCodexChannelId({
        form: { ...baseForm, id: 12 },
        saveChannel,
      }),
    ).resolves.toBe(12)

    expect(saveChannel).not.toHaveBeenCalled()
  })

  it("creates the channel first and returns the new id for Codex imports", async () => {
    const saveChannel = vi.fn().mockResolvedValue({
      data: {
        id: 27,
      },
    })

    await expect(
      ensureCodexChannelId({
        form: baseForm,
        saveChannel,
      }),
    ).resolves.toBe(27)

    expect(saveChannel).toHaveBeenCalledWith(baseForm)
  })

  it("throws when the created channel id is missing from the save response", async () => {
    const saveChannel = vi.fn().mockResolvedValue({ data: {} })

    await expect(
      ensureCodexChannelId({
        form: baseForm,
        saveChannel,
      }),
    ).rejects.toThrow("Failed to save channel")
  })
})

describe("mergeRuntimeChannelModels", () => {
  it("hydrates the draft with synced runtime models from the saved channel", () => {
    expect(
      mergeRuntimeChannelModels(
        { ...baseForm, id: 27, name: "Codex Channel (edited)" },
        {
          id: 27,
          model: ["gpt-5", "o3"],
          fetchedModel: ["gpt-5", "o3"],
        },
      ),
    ).toEqual({
      ...baseForm,
      id: 27,
      name: "Codex Channel (edited)",
      model: ["gpt-5", "o3"],
      fetchedModel: ["gpt-5", "o3"],
    })
  })

  it("keeps the current draft when the refreshed channel does not match", () => {
    const form = { ...baseForm, id: 27, model: ["manual-model"], fetchedModel: [] }

    expect(
      mergeRuntimeChannelModels(form, {
        id: 99,
        model: ["gpt-5"],
        fetchedModel: ["gpt-5"],
      }),
    ).toBe(form)
  })
})

describe("runtime channel helpers", () => {
  it("detects Codex and Copilot as runtime-managed providers", () => {
    expect(isRuntimeChannelType(33)).toBe(true)
    expect(isRuntimeChannelType(34)).toBe(true)
    expect(isRuntimeChannelType(1)).toBe(false)
    expect(getRuntimeProviderKey(33)).toBe("codex")
    expect(getRuntimeProviderKey(34)).toBe("copilot")
    expect(getRuntimeProviderKey(1)).toBeNull()
  })

  it("adapts runtime drafts to managed credentials and blank base url", () => {
    expect(
      adaptChannelDraftForType(
        {
          ...baseForm,
          type: 1,
          baseUrls: [{ url: "https://api.openai.com", delay: 50 }],
          keys: [{ channelKey: "sk-live", remark: "primary" }],
        },
        34,
      ),
    ).toEqual({
      ...baseForm,
      type: 34,
      baseUrls: [{ url: "", delay: 50 }],
      keys: [{ channelKey: RUNTIME_MANAGED_CHANNEL_KEY, remark: "primary" }],
    })
  })

  it("clears the managed placeholder when switching back to a generic provider", () => {
    expect(adaptChannelDraftForType(baseForm, 1)).toEqual({
      ...baseForm,
      type: 1,
      keys: [{ channelKey: "", remark: "" }],
    })
  })

  it("hides generic model fetch in runtime mode", () => {
    expect(shouldShowGenericModelFetch(33)).toBe(false)
    expect(shouldShowGenericModelFetch(34)).toBe(false)
    expect(shouldShowGenericModelFetch(1)).toBe(true)
  })
})
