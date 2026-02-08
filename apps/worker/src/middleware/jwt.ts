import type { MiddlewareHandler } from "hono"

import type { AppEnv } from "../runtime/types"

// Minimal JWT implementation for Edge (no external deps)
interface JWTPayload {
  iat: number
  exp: number
  iss: string
}

async function createHmacKey(secret: string): Promise<CryptoKey> {
  const enc = new TextEncoder()
  return crypto.subtle.importKey(
    "raw",
    enc.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign", "verify"],
  )
}

function base64UrlEncode(data: ArrayBuffer | Uint8Array): string {
  const bytes = data instanceof Uint8Array ? data : new Uint8Array(data)
  let binary = ""
  for (const b of bytes) binary += String.fromCharCode(b)
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
}

function base64UrlDecode(str: string): Uint8Array {
  str = str.replace(/-/g, "+").replace(/_/g, "/")
  while (str.length % 4) str += "="
  const binary = atob(str)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

export async function signJWT(payload: JWTPayload, secret: string): Promise<string> {
  const enc = new TextEncoder()
  const header = base64UrlEncode(enc.encode(JSON.stringify({ alg: "HS256", typ: "JWT" })))
  const body = base64UrlEncode(enc.encode(JSON.stringify(payload)))
  const data = `${header}.${body}`
  const key = await createHmacKey(secret)
  const sig = await crypto.subtle.sign("HMAC", key, enc.encode(data))
  return `${data}.${base64UrlEncode(sig)}`
}

export async function verifyJWT(token: string, secret: string): Promise<JWTPayload | null> {
  const parts = token.split(".")
  if (parts.length !== 3) return null

  const [header, body, sig] = parts
  const key = await createHmacKey(secret)
  const enc = new TextEncoder()
  const data = `${header}.${body}`
  const valid = await crypto.subtle.verify(
    "HMAC",
    key,
    base64UrlDecode(sig).buffer as ArrayBuffer,
    enc.encode(data),
  )
  if (!valid) return null

  const payload: JWTPayload = JSON.parse(new TextDecoder().decode(base64UrlDecode(body)))
  if (payload.exp && payload.exp < Math.floor(Date.now() / 1000)) return null

  return payload
}

export function generateToken(expiresMinutes: number) {
  const now = Math.floor(Date.now() / 1000)
  let exp: number
  if (expiresMinutes === 0) {
    exp = now + 15 * 60 // 15 minutes
  } else if (expiresMinutes === -1) {
    exp = now + 30 * 24 * 60 * 60 // 30 days
  } else {
    exp = now + expiresMinutes * 60
  }

  const payload: JWTPayload = { iat: now, exp, iss: "wheel" }
  const expireAt = new Date(exp * 1000).toISOString()
  return { payload, expireAt }
}

export const jwtAuth = (): MiddlewareHandler<AppEnv> => {
  return async (c, next) => {
    const authHeader = c.req.header("Authorization")
    if (!authHeader?.startsWith("Bearer ")) {
      return c.json({ success: false, error: "Unauthorized" }, 401)
    }

    const token = authHeader.slice(7)
    const secret = c.env.JWT_SECRET
    const payload = await verifyJWT(token, secret)
    if (!payload) {
      return c.json({ success: false, error: "Invalid or expired token" }, 401)
    }

    await next()
  }
}
