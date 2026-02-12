import { create } from "zustand"
import { persist } from "zustand/middleware"

interface AuthState {
  token: string | null
  expireAt: string | null
  setAuth: (token: string, expireAt: string) => void
  logout: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      expireAt: null,
      setAuth: (token, expireAt) => set({ token, expireAt }),
      logout: () => set({ token: null, expireAt: null }),
      isAuthenticated: () => {
        const { token, expireAt } = get()
        if (!token) return false
        if (expireAt && new Date(expireAt) < new Date()) {
          set({ token: null, expireAt: null })
          return false
        }
        return true
      },
    }),
    { name: "wheel-auth" },
  ),
)
