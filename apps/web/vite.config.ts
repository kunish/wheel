import { resolve } from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

export default defineConfig({
  base: process.env.VITE_BASE_PATH || "/",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": resolve(__dirname, "src"),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: process.env.VITE_API_BASE_URL || "http://localhost:8787",
        changeOrigin: true,
      },
      "/v1": {
        target: process.env.VITE_API_BASE_URL || "http://localhost:8787",
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {},
})
