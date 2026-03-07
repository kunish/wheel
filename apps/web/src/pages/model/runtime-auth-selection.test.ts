import { describe, expect, it } from "vitest"
import {
  buildAuthFileBatchScope,
  clearRuntimeAuthSelection,
  createRuntimeAuthSelection,
  demoteSelectionFromAllMatching,
  getCurrentPageSelectionState,
  getSelectedCount,
  isAuthFileSelected,
  promoteSelectionToAllMatching,
  setCurrentPageSelection,
  toggleAuthFileSelection,
} from "./runtime-auth-selection"

describe("runtime auth selection", () => {
  it("selects and deselects explicit items", () => {
    let selection = createRuntimeAuthSelection()
    selection = toggleAuthFileSelection(selection, "a.json")
    selection = toggleAuthFileSelection(selection, "c.json")

    expect(getSelectedCount(selection, 10)).toBe(2)
    expect(isAuthFileSelected(selection, "a.json")).toBe(true)
    expect(isAuthFileSelected(selection, "b.json")).toBe(false)

    selection = toggleAuthFileSelection(selection, "a.json")
    expect(getSelectedCount(selection, 10)).toBe(1)
    expect(isAuthFileSelected(selection, "a.json")).toBe(false)
  })

  it("tracks current-page checkbox state for explicit selection", () => {
    let selection = createRuntimeAuthSelection()
    selection = setCurrentPageSelection(selection, ["a.json", "b.json", "c.json"], true)

    expect(getCurrentPageSelectionState(selection, ["a.json", "b.json", "c.json"])).toBe(true)

    selection = toggleAuthFileSelection(selection, "b.json")
    expect(getCurrentPageSelectionState(selection, ["a.json", "b.json", "c.json"])).toBe(
      "indeterminate",
    )
  })

  it("supports all-matching selection with exclusions", () => {
    let selection = createRuntimeAuthSelection()
    selection = setCurrentPageSelection(selection, ["a.json", "b.json"], true)
    selection = promoteSelectionToAllMatching(selection)

    expect(getSelectedCount(selection, 12)).toBe(12)
    expect(isAuthFileSelected(selection, "z.json")).toBe(true)

    selection = toggleAuthFileSelection(selection, "b.json")
    expect(getSelectedCount(selection, 12)).toBe(11)
    expect(isAuthFileSelected(selection, "b.json")).toBe(false)
    expect(getCurrentPageSelectionState(selection, ["a.json", "b.json"])).toBe("indeterminate")

    selection = setCurrentPageSelection(selection, ["a.json", "b.json"], false)
    expect(getSelectedCount(selection, 12)).toBe(10)
    expect(isAuthFileSelected(selection, "a.json")).toBe(false)
  })

  it("can cancel all-matching mode and fall back to selected items on the current page", () => {
    let selection = createRuntimeAuthSelection()
    selection = setCurrentPageSelection(selection, ["a.json", "b.json"], true)
    selection = promoteSelectionToAllMatching(selection)
    selection = toggleAuthFileSelection(selection, "b.json")

    const demoted = demoteSelectionFromAllMatching(selection, ["a.json", "b.json", "c.json"])

    expect(demoted).toEqual({ mode: "explicit", names: ["a.json", "c.json"] })
  })

  it("builds explicit-name and all-matching batch scopes", () => {
    const explicit = setCurrentPageSelection(
      createRuntimeAuthSelection(),
      ["a.json", "b.json"],
      true,
    )
    expect(buildAuthFileBatchScope(explicit, { provider: "copilot", search: "team" })).toEqual({
      names: ["a.json", "b.json"],
    })

    let allMatching = promoteSelectionToAllMatching(explicit)
    allMatching = toggleAuthFileSelection(allMatching, "b.json")
    expect(buildAuthFileBatchScope(allMatching, { provider: "copilot", search: "team" })).toEqual({
      allMatching: true,
      provider: "copilot",
      search: "team",
      excludeNames: ["b.json"],
    })
  })

  it("clears selection back to empty explicit mode", () => {
    const cleared = clearRuntimeAuthSelection()
    expect(getSelectedCount(cleared, 5)).toBe(0)
    expect(getCurrentPageSelectionState(cleared, ["a.json"])).toBe(false)
  })
})
