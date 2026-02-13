import type { Page } from "@playwright/test"
import type { ChildProcess } from "node:child_process"
import { spawn } from "node:child_process"
import { mkdirSync } from "node:fs"
import { resolve } from "node:path"
import { chromium } from "@playwright/test"

const ROOT = resolve(import.meta.dirname, "../../..")
const WORKER_DIR = resolve(ROOT, "apps/worker")
const SCREENSHOTS_DIR = resolve(ROOT, "docs/screenshots")
const WORKER_PORT = 18787
const WEB_PORT = 15173
const WORKER_URL = `http://localhost:${WORKER_PORT}`
const WEB_URL = `http://localhost:${WEB_PORT}`

const PAGES = [
  { name: "dashboard", path: "/dashboard" },
  { name: "model", path: "/model" },
  { name: "groups", path: "/groups" },
  { name: "logs", path: "/logs" },
  { name: "settings", path: "/settings" },
]

const processes: ChildProcess[] = []

function cleanup() {
  for (const p of processes) {
    p.kill("SIGTERM")
  }
}

process.on("SIGINT", () => {
  cleanup()
  process.exit(1)
})
process.on("SIGTERM", () => {
  cleanup()
  process.exit(1)
})

async function waitForReady(url: string, label: string, timeoutMs = 30_000) {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(url)
      if (res.ok) {
        console.log(`✓ ${label} ready`)
        return
      }
    } catch {}
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error(`${label} did not become ready within ${timeoutMs}ms`)
}

function startWorker(): Promise<ChildProcess> {
  // Build worker first
  console.log("Building worker...")
  const build = spawn("go", ["build", "-o", "tmp/worker", "./cmd/worker/"], {
    cwd: WORKER_DIR,
    stdio: "inherit",
  })

  const seedDataPath = resolve(WORKER_DIR, "seed-data")

  return new Promise<ChildProcess>((res, rej) => {
    build.on("close", (code) => {
      if (code !== 0) {
        rej(new Error(`Worker build failed with code ${code}`))
        return
      }

      console.log("Starting worker with seed data...")
      const worker = spawn("./tmp/worker", ["seed"], {
        cwd: WORKER_DIR,
        stdio: "inherit",
        env: {
          ...process.env,
          PORT: String(WORKER_PORT),
          DATA_PATH: seedDataPath,
        },
      })
      processes.push(worker)

      worker.on("close", (seedCode) => {
        // After seed completes, start the actual server
        const idx = processes.indexOf(worker)
        if (idx >= 0) processes.splice(idx, 1)

        if (seedCode !== 0) {
          rej(new Error(`Worker seed failed with code ${seedCode}`))
          return
        }

        console.log("Starting worker server...")
        const server = spawn("./tmp/worker", [], {
          cwd: WORKER_DIR,
          stdio: "inherit",
          env: {
            ...process.env,
            PORT: String(WORKER_PORT),
            DATA_PATH: seedDataPath,
          },
        })
        processes.push(server)
        server.on("error", rej)
        res(server)
      })
    })
    build.on("error", rej)
  })
}

function startWebDev(): ChildProcess {
  console.log("Starting web dev server...")
  const web = spawn("pnpm", ["dev", "--port", String(WEB_PORT), "--strictPort"], {
    cwd: resolve(ROOT, "apps/web"),
    stdio: "inherit",
    env: {
      ...process.env,
      VITE_API_BASE_URL: WORKER_URL,
    },
  })
  processes.push(web)
  web.on("error", (err) => {
    console.error("Web dev server error:", err)
  })
  return web
}

async function login(page: Page) {
  await page.goto(`${WEB_URL}/login`)
  await page.waitForLoadState("networkidle")
  await page.fill("#username", "admin")
  await page.fill("#password", "admin")
  await page.click('button[type="submit"]')
  await page.waitForURL("**/dashboard")
  // Wait for dashboard data to load
  await page.waitForTimeout(2000)
}

async function setTheme(page: Page, theme: "light" | "dark") {
  await page.evaluate((t) => {
    document.documentElement.classList.remove("light", "dark")
    document.documentElement.classList.add(t)
    document.documentElement.style.colorScheme = t
    localStorage.setItem("theme", t)
  }, theme)
  // Wait for theme transition
  await page.waitForTimeout(500)
}

async function captureScreenshots() {
  mkdirSync(SCREENSHOTS_DIR, { recursive: true })

  const browser = await chromium.launch()
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  })
  const page = await context.newPage()

  await login(page)

  for (const theme of ["light", "dark"] as const) {
    await setTheme(page, theme)

    for (const { name, path } of PAGES) {
      await page.goto(`${WEB_URL}${path}`)
      await page.waitForLoadState("networkidle")
      await page.waitForTimeout(1500) // Wait for animations

      const filename = `${name}-${theme}.png`
      await page.screenshot({
        path: resolve(SCREENSHOTS_DIR, filename),
        fullPage: false,
      })
      console.log(`✓ Captured ${filename}`)
    }
  }

  await browser.close()
}

async function main() {
  try {
    await startWorker()
    await waitForReady(WORKER_URL, "Worker")

    startWebDev()
    await waitForReady(WEB_URL, "Web dev server")

    await captureScreenshots()
    console.log("\n✓ All screenshots captured successfully!")
  } catch (err) {
    console.error("Screenshot capture failed:", err)
    process.exitCode = 1
  } finally {
    cleanup()
  }
}

main()
