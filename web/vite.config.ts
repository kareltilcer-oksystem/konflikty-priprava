import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The Go binary embeds internal/spa/dist with `go:embed all:dist`, and an embed
// pattern that matches nothing is a compile error. A tracked .gitkeep keeps a
// fresh clone building; emptyOutDir wipes it on every build, so put it back.
function keepEmbedPlaceholder(): Plugin {
  return {
    name: 'keep-embed-placeholder',
    generateBundle() {
      this.emitFile({ type: 'asset', fileName: '.gitkeep', source: '' })
    },
  }
}

export default defineConfig({
  plugins: [react(), tailwindcss(), keepEmbedPlaceholder()],

  build: {
    // Output straight into the Go package: `go:embed` cannot reach outside its
    // own directory, so the built assets have to live there.
    outDir: '../internal/spa/dist',
    // Required, since outDir is outside the Vite root. Without it Vite only
    // warns and stale hashed bundles accumulate inside the binary for ever.
    emptyOutDir: true,
    sourcemap: false,
  },

  server: {
    port: 9999,
    // Fail loudly rather than silently moving to another port: the Go server's
    // WEB_PORT=0 opt-out exists precisely so 9999 is free for Vite.
    strictPort: true,
    proxy: {
      // The browser always talks to /api on its own origin. In development
      // that is this proxy; in production the SPA listener routes /api to the
      // same in-process handler. Root-relative attachment URLs therefore
      // resolve identically in both environments, and no CORS is needed.
      '/api': {
        // The literal IPv4 address, not "localhost": Node can resolve that to
        // ::1 first, and the proxy then fails against a v4-only listener.
        target: 'http://127.0.0.1:9998',
        changeOrigin: false,
      },
    },
  },
})
