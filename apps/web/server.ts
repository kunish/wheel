/**
 * Custom server for Next.js with WebSocket proxy.
 *
 * Proxies WebSocket upgrade requests for /api/v1/ws to the worker backend,
 * since Next.js route handlers cannot handle WebSocket upgrades.
 */
import { createServer } from "node:http"
import net from "node:net"
import next from "next"

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

  server.on("upgrade", (req, socket, head) => {
    const url = new URL(req.url ?? "/", `http://${req.headers.host}`)
    if (url.pathname === "/api/v1/ws") {
      const target = new URL(apiBaseUrl)
      const targetPort = Number.parseInt(target.port) || 8787
      const wsTarget = `${target.hostname}:${targetPort}`

      const upstream = net.connect({ host: target.hostname, port: targetPort }, () => {
        const reqLine = `GET /api/v1/ws HTTP/1.1\r\n`
        const headers = Object.entries(req.headers)
          .filter(([k]) => k !== "host")
          .map(([k, v]) => `${k}: ${v}`)
          .join("\r\n")
        upstream.write(`${reqLine}Host: ${wsTarget}\r\n${headers}\r\n\r\n`)
        if (head.length > 0) upstream.write(head)

        upstream.pipe(socket)
        socket.pipe(upstream)
      })
      upstream.on("error", () => socket.destroy())
      socket.on("error", () => upstream.destroy())
    }
  })

  server.listen(port, hostname, () => {
    console.warn(`> Ready on http://${hostname}:${port}`)
  })
})
