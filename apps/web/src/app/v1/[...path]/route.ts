export const runtime = "nodejs"
export const dynamic = "force-dynamic"

const API_BASE_URL = process.env.API_BASE_URL || "http://localhost:8787"

async function handler(request: Request) {
  const url = new URL(request.url)
  const target = new URL(url.pathname + url.search, API_BASE_URL)

  const headers = new Headers(request.headers)
  headers.delete("host")

  const resp = await fetch(target.toString(), {
    method: request.method,
    headers,
    body: request.body,
    // @ts-expect-error -- Node.js fetch supports duplex for streaming request bodies
    duplex: "half",
  })

  // Next.js 16 route handlers add Content-Encoding: gzip to proxied responses
  // without actually compressing the body, causing ZlibError in clients (e.g.
  // OpenCode/Bun) that auto-decompress based on the header. Force identity
  // encoding on all proxied responses to prevent this mismatch.
  const resHeaders = new Headers(resp.headers)
  resHeaders.set("Content-Encoding", "identity")

  return new Response(resp.body, {
    status: resp.status,
    statusText: resp.statusText,
    headers: resHeaders,
  })
}

export const GET = handler
export const POST = handler
export const PUT = handler
export const DELETE = handler
export const PATCH = handler
export const OPTIONS = handler
