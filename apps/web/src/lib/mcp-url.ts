interface ResolveMcpServerUrlParams {
  backendServerUrl?: string
  apiBaseUrl?: string
  windowOrigin: string
}

function normalizeBaseUrl(base: string): string {
  return base.replace(/\/+$/, "")
}

function fromBase(base: string): string {
  return `${normalizeBaseUrl(base)}/mcp/sse`
}

export function resolveMcpServerUrl({
  backendServerUrl,
  apiBaseUrl,
  windowOrigin,
}: ResolveMcpServerUrlParams): string {
  if (backendServerUrl && backendServerUrl.trim()) {
    const normalized = normalizeBaseUrl(backendServerUrl)
    if (/\/mcp$/.test(normalized)) {
      return `${normalized}/sse`
    }
    return normalized
  }
  if (apiBaseUrl && apiBaseUrl.trim()) {
    return fromBase(apiBaseUrl)
  }
  return fromBase(windowOrigin)
}

export function resolveMcpToolExecuteUrl(mcpServerUrl: string): string {
  const gatewayBase = normalizeBaseUrl(mcpServerUrl).replace(/\/mcp(?:\/(sse|message))?$/, "")
  return `${gatewayBase}/v1/mcp/tool/execute`
}
