/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // Never 0.0.0.0: a dev machine with a public IP would expose the dev server.
    host: '127.0.0.1',
    // 5173 is growth-portal's.
    port: 5174,
    // The Go server serves /api and sign-in (/auth); in dev, Vite forwards to it (`go run ./cmd/server`).
    proxy: { '/api': 'http://127.0.0.1:8002', '/auth': 'http://127.0.0.1:8002' },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
  },
})
