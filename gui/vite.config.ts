// Vite configuration for the Alexandria console: React, Tailwind v4 and same-origin proxies to the hub, the client admin listener and the OIDC issuer.
// The proxies avoid CORS and the lab TLS certificate, and rewrite absolute hub Location headers so /pub redirects stay same-origin.
import { fileURLToPath, URL } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv, type ProxyOptions } from 'vite'

// proxyTo builds a prefix-stripping proxy entry that keeps redirects inside the prefix.
function proxyTo(prefix: string, target: string): ProxyOptions {
  const origin = new URL(target).origin
  return {
    target,
    changeOrigin: true,
    secure: false,
    rewrite: (path) => path.slice(prefix.length) || '/',
    configure: (proxy) => {
      proxy.on('proxyRes', (res) => {
        const loc = res.headers.location
        if (loc?.startsWith(origin)) res.headers.location = prefix + loc.slice(origin.length)
        else if (loc?.startsWith('/')) res.headers.location = prefix + loc
      })
    },
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'ALEXANDRIA_')
  const hub = env.ALEXANDRIA_HUB_URL || 'https://127.0.0.1:8443'
  const client = env.ALEXANDRIA_CLIENT_ADMIN_URL || 'http://127.0.0.1:9090'
  const idp = env.ALEXANDRIA_OIDC_ISSUER || 'http://127.0.0.1:9400'
  const proxy = { '/api/hub': proxyTo('/api/hub', hub), '/api/client': proxyTo('/api/client', client), '/api/idp': proxyTo('/api/idp', idp) }
  // The embed mode builds the console served by the Go binaries under /ui/ (internal/webui, -tags gui).
  const embed = mode === 'embed'
  return {
    base: embed ? '/ui/' : '/',
    build: embed ? { outDir: '../internal/webui/dist', emptyOutDir: true } : {},
    plugins: [react(), tailwindcss()],
    resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
    define: { __HUB_TARGET__: JSON.stringify(hub), __CLIENT_TARGET__: JSON.stringify(client), __IDP_TARGET__: JSON.stringify(idp) },
    server: { port: 5273, proxy },
    preview: { port: 5273, proxy },
  }
})
