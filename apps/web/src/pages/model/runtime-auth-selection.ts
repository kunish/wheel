export type RuntimeAuthSelection =
  | { mode: "explicit"; names: string[] }
  | { mode: "allMatching"; excludeNames: string[] }

export function createRuntimeAuthSelection(): RuntimeAuthSelection {
  return { mode: "explicit", names: [] }
}

export function clearRuntimeAuthSelection(): RuntimeAuthSelection {
  return createRuntimeAuthSelection()
}

export function getSelectedCount(selection: RuntimeAuthSelection, totalMatching: number): number {
  if (selection.mode === "allMatching") {
    return Math.max(0, totalMatching - selection.excludeNames.length)
  }
  return selection.names.length
}

export function isAuthFileSelected(selection: RuntimeAuthSelection, name: string): boolean {
  if (selection.mode === "allMatching") {
    return !selection.excludeNames.includes(name)
  }
  return selection.names.includes(name)
}

export function toggleAuthFileSelection(
  selection: RuntimeAuthSelection,
  name: string,
): RuntimeAuthSelection {
  if (selection.mode === "allMatching") {
    return selection.excludeNames.includes(name)
      ? {
          mode: "allMatching",
          excludeNames: selection.excludeNames.filter((item) => item !== name),
        }
      : { mode: "allMatching", excludeNames: [...selection.excludeNames, name].sort() }
  }

  return selection.names.includes(name)
    ? { mode: "explicit", names: selection.names.filter((item) => item !== name) }
    : { mode: "explicit", names: [...selection.names, name].sort() }
}

export function setCurrentPageSelection(
  selection: RuntimeAuthSelection,
  pageNames: string[],
  checked: boolean,
): RuntimeAuthSelection {
  const uniquePageNames = [...new Set(pageNames)]
  if (selection.mode === "allMatching") {
    return checked
      ? {
          mode: "allMatching",
          excludeNames: selection.excludeNames.filter((name) => !uniquePageNames.includes(name)),
        }
      : {
          mode: "allMatching",
          excludeNames: [...new Set([...selection.excludeNames, ...uniquePageNames])].sort(),
        }
  }

  return checked
    ? { mode: "explicit", names: [...new Set([...selection.names, ...uniquePageNames])].sort() }
    : { mode: "explicit", names: selection.names.filter((name) => !uniquePageNames.includes(name)) }
}

export function getCurrentPageSelectionState(
  selection: RuntimeAuthSelection,
  pageNames: string[],
): boolean | "indeterminate" {
  if (pageNames.length === 0) {
    return false
  }
  const selectedCount = pageNames.filter((name) => isAuthFileSelected(selection, name)).length
  if (selectedCount === 0) {
    return false
  }
  if (selectedCount === pageNames.length) {
    return true
  }
  return "indeterminate"
}

export function promoteSelectionToAllMatching(
  selection: RuntimeAuthSelection,
): RuntimeAuthSelection {
  if (selection.mode === "allMatching") {
    return selection
  }
  return { mode: "allMatching", excludeNames: [] }
}

export function demoteSelectionFromAllMatching(
  selection: RuntimeAuthSelection,
  pageNames: string[],
): RuntimeAuthSelection {
  if (selection.mode !== "allMatching") {
    return selection
  }

  return {
    mode: "explicit",
    names: [...new Set(pageNames.filter((name) => !selection.excludeNames.includes(name)))].sort(),
  }
}

export function buildAuthFileBatchScope(
  selection: RuntimeAuthSelection,
  scope: { provider?: string; search?: string },
) {
  if (selection.mode === "allMatching") {
    return {
      allMatching: true,
      provider: scope.provider,
      search: scope.search,
      excludeNames: selection.excludeNames,
    }
  }
  return {
    names: selection.names,
  }
}
