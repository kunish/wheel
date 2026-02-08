import type { NextRequest } from "next/server"
import { NextResponse } from "next/server"

const API_BASE_URL = process.env.API_BASE_URL || "http://localhost:8787"

/**
 * Minimal proxy — only handles WebSocket upgrades.
 * All other /api/* and /v1/* requests are handled by Route Handlers
 * which have first-class streaming support.
 */
export async function proxy(request: NextRequest) {
  if (request.headers.get("upgrade") === "websocket") {
    const { pathname, search } = request.nextUrl
    const target = new URL(pathname + search, API_BASE_URL)
    return NextResponse.rewrite(target)
  }

  return NextResponse.next()
}

export const config = {
  matcher: ["/api/:path*"],
}
