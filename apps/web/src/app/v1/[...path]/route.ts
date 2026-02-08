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

  // Return response immediately with streaming body.
  // Must use plain Response (not NextResponse) to avoid buffering.
  return new Response(resp.body, {
    status: resp.status,
    statusText: resp.statusText,
    headers: resp.headers,
  })
}

export const GET = handler
export const POST = handler
export const PUT = handler
export const DELETE = handler
export const PATCH = handler
export const OPTIONS = handler
