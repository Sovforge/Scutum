import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: process.env.API_BASE ?? 'http://localhost:8080',
        changeOrigin: true,
        // Terminal exec sessions (and any future WS endpoint) go over
        // /api/... too — without this, Vite silently drops the WebSocket
        // upgrade instead of proxying it, and the connection just hangs.
        ws: true,
      },
    },
  },
})
