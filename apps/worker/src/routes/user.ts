import type { AppEnv } from "../runtime/types"
import { Hono } from "hono"
import { getUser, updatePassword, updateUsername } from "../db/dal/users"
import { generateToken, jwtAuth, signJWT } from "../middleware/jwt"

const userRoutes = new Hono<AppEnv>()

userRoutes.post("/login", async (c) => {
  const body = await c.req.json()
  const { username, password } = body

  if (username !== c.env.ADMIN_USERNAME || password !== c.env.ADMIN_PASSWORD) {
    return c.json({ success: false, error: "Invalid credentials" }, 401)
  }

  const { payload, expireAt } = generateToken(-1) // 30 days
  const token = await signJWT(payload, c.env.JWT_SECRET)
  return c.json({ success: true, data: { token, expireAt } })
})

// Protected routes — require JWT
userRoutes.use("/change-password", jwtAuth())
userRoutes.use("/change-username", jwtAuth())
userRoutes.use("/status", jwtAuth())

userRoutes.post("/change-password", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const user = await getUser(db)
  if (!user) {
    return c.json({ success: false, error: "User not found" }, 404)
  }
  await updatePassword(db, user.id, body.newPassword)
  return c.json({ success: true })
})

userRoutes.post("/change-username", async (c) => {
  const body = await c.req.json()
  const db = c.env.DB
  const user = await getUser(db)
  if (!user) {
    return c.json({ success: false, error: "User not found" }, 404)
  }
  await updateUsername(db, user.id, body.username)
  return c.json({ success: true })
})

userRoutes.get("/status", async (c) => {
  return c.json({ success: true, data: { authenticated: true } })
})

export { userRoutes }
