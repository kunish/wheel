/**
 * Custom server for Next.js with WebSocket proxy.
 *
 * Proxies WebSocket upgrade requests for /api/v1/ws to the worker backend.
 * Uses http-proxy style raw TCP pipe but with proper WebSocket handshake
 * via the ws library to avoid "Invalid frame header" errors.
 */
import { createServer, request as httpRequest } from "node:http"
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
    if (url.pathname !== "/api/v1/ws") return

    const target = new URL(apiBaseUrl)

    const proxyReq = httpRequest({
      hostname: target.hostname,
      port: target.port || 8787,
      path: "/api/v1/ws",
      method: "GET",
      headers: {
        ...req.headers,
        host: target.host,
      },
    })

    proxyReq.on("upgrade", (proxyRes, proxySocket, proxyHead) => {
      // Write the HTTP 101 response back to the client
      let resHeaders = "HTTP/1.1 101 Switching Protocols\r\n"
      for (const [key, value] of Object.entries(proxyRes.headers)) {
        if (value) resHeaders += `${key}: ${value}\r\n`
      }
      resHeaders += "\r\n"
      socket.write(resHeaders)

      if (proxyHead.length > 0) socket.write(proxyHead)
      if (head.length > 0) proxySocket.write(head)

      proxySocket.pipe(socket)
      socket.pipe(proxySocket)

      proxySocket.on("error", () => socket.destroy())
      socket.on("error", () => proxySocket.destroy())
      proxySocket.on("close", () => socket.destroy())
      socket.on("close", () => proxySocket.destroy())
    })

    proxyReq.on("error", () => socket.destroy())
    proxyReq.end()
  })

  server.listen(port, hostname, () => {
    console.warn(`> Ready on http://${hostname}:${port}`)
  })
})
