import react, { reactCompilerPreset } from '@vitejs/plugin-react'
import babel from '@rolldown/plugin-babel'
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

const apiTarget = process.env.API_URL || `http://localhost:${process.env.API_PORT || '8090'}`

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), babel({ presets: [reactCompilerPreset()] }), tailwindcss()],
  server: {
    // The API serves the built dashboard from the same origin in production, so the
    // dev server has to impersonate it. The API's route prefixes are proxied without a
    // rewrite, which is what lets the frontend call `/meters` in development and in
    // production alike — one set of relative paths, no environment branch, and no CORS.
    proxy: Object.fromEntries(
      ['/meters', '/anomalies', '/ai', '/dashboard'].map((route) => [
        route,
        {
          target: apiTarget,
          changeOrigin: true,
        },
      ]),
    ),
  },
})
