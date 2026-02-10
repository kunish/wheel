import type { NextConfig } from "next"

const apiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8787"

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${apiBaseUrl}/api/:path*` },
      { source: "/v1/:path*", destination: `${apiBaseUrl}/v1/:path*` },
    ]
  },
}

export default nextConfig
