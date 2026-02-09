/**
 * Custom server for Next.js with WebSocket proxy.
 *
 * Proxies WebSocket upgrade requests for /api/v1/ws to the worker backend,
 * since Next.js route handlers cannot handle WebSocket upgrades.
 */
import { createServer } from "node:http"
import next from "next"
import { WebSocket, WebSocketServer } from "ws"

const port = Number.parseInt(process.env.PORT ?? "3000", 10)
const hostname = process.env.HOSTNAME ?? "0.0.0.0"
const apiBaseUrl = process.env.API_BASE_URL ?? "http://localhost:8787"
const dev = process.env.NODE_ENV !== "production"

const app = next({ dev, hostname, port, dir: __dirname })
const handle = app.getRequestHandler()

app.prepare().then(() => {
  const server = createServer((req, res) => {
    handle(req, res)
  })

  const wss = new WebSocketServer({ noServer: true })

  server.on("upgrade", (req, socket, head) => {
    const url = new URL(req.url ?? "/", `http://${req.headers.host}`)
    if (url.pathname === "/api/v1/ws") {
      wss.handleUpgrade(req, socket, head, (clientWs) => {
        const target = new URL(apiBaseUrl)
        const wsProto = target.protocol === "https:" ? "wss:" : "ws:"
        const upstreamUrl = `${wsProto}//${target.host}/api/v1/ws`

        const upstream = new WebSocket(upstreamUrl)

        upstream.on("open", () => {
          clientWs.on("message", (data) => {
            if (upstream.readyState === WebSocket.OPEN) upstream.send(data)
          })
          upstream.on("message", (data) => {
            if (clientWs.readyState === WebSocket.OPEN) clientWs.send(data)
          })
        })

        upstream.on("close", () => clientWs.close())
        upstream.on("error", () => clientWs.close())
        clientWs.on("close", () => upstream.close())
        clientWs.on("error", () => upstream.close())
      })
    }
  })

  server.listen(port, hostname, () => {
    console.warn(`> Ready on http://${hostname}:${port}`)
  })
})
