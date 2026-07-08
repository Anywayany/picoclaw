import path from "path"

import tailwindcss from "@tailwindcss/vite"
import { tanstackRouter } from "@tanstack/router-plugin/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

function normalizeBasePath(raw: string | undefined): string {
  const trimmed = (raw ?? "").trim()
  if (trimmed === "" || trimmed === "/") {
    return ""
  }
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`
  return withSlash.replace(/\/+$/, "")
}

const publicBasePath = normalizeBasePath(process.env.VITE_PUBLIC_BASE_PATH)
const proxyPath = (path: string) =>
  publicBasePath === "" ? path : `${publicBasePath}${path}`
const stripProxyBase = (path: string) =>
  publicBasePath === "" ? path : path.slice(publicBasePath.length) || "/"

// https://vite.dev/config/
export default defineConfig({
  base: publicBasePath === "" ? "/" : `${publicBasePath}/`,
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
    }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    chunkSizeWarningLimit: 2048,
  },
  server: {
    proxy: {
      [proxyPath("/api")]: {
        target: "http://localhost:18800",
        changeOrigin: true,
        rewrite: stripProxyBase,
      },
      [proxyPath("/pico/media")]: {
        target: "http://localhost:18800",
        changeOrigin: true,
        rewrite: stripProxyBase,
      },
      [proxyPath("/pico/ws")]: {
        target: "ws://localhost:18800",
        ws: true,
        rewrite: stripProxyBase,
      },
    },
  },
})
