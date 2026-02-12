import { create } from "zustand"
import { persist } from "zustand/middleware"

interface AuthState {
  token: string | null
  expireAt: string | null
  apiBaseUrl: string
  setAuth: (token: string, expireAt: string) => void
  logout: () => void
  isAuthenticated: () => boolean
  setApiBaseUrl: (url: string) => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      expireAt: null,
      apiBaseUrl: "",
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
      setApiBaseUrl: (url) => set({ apiBaseUrl: url.replace(/\/+$/, "") }),
    }),
    { name: "wheel-auth" },
  ),
)
