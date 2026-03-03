import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import { describe, expect, it } from "vitest"

function flattenKeys(input: unknown, prefix = ""): string[] {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    return prefix ? [prefix] : []
  }

  const out: string[] = []
  const record = input as Record<string, unknown>
  for (const key of Object.keys(record).sort()) {
    const current = prefix ? `${prefix}.${key}` : key
    const child = record[key]
    if (child && typeof child === "object" && !Array.isArray(child)) {
      out.push(...flattenKeys(child, current))
    } else {
      out.push(current)
    }
  }
  return out
}

function readLocale(locale: "en" | "zh-CN") {
  const file = resolve(process.cwd(), "src/i18n/locales", locale, "playground.json")
  return JSON.parse(readFileSync(file, "utf8")) as Record<string, unknown>
}

describe("playground i18n keys", () => {
  it("keeps en and zh-CN key sets aligned", () => {
    const en = flattenKeys(readLocale("en"))
    const zh = flattenKeys(readLocale("zh-CN"))
    expect(zh).toEqual(en)
  })
})
