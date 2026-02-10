import type { NextRequest } from "next/server"
import { NextResponse } from "next/server"

export function middleware(request: NextRequest) {
  // Block WebSocket upgrade requests from being proxied through Next.js rewrites.
  // The frontend connects directly to the worker for WebSocket (port 8787 in dev).
  if (request.nextUrl.pathname === "/api/v1/ws") {
    return new NextResponse("WebSocket not supported via proxy", { status: 426 })
  }
  return NextResponse.next()
}

export const config = {
  matcher: "/api/v1/ws",
}
