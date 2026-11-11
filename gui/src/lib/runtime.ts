// Runtime environment of the console: standalone behind the Vite proxies, or embedded in a binary that serves it under /ui/.
// An embedding binary injects window.__ALEXANDRIA__ with the workspace it offers and, for the hub, its OIDC issuer and audience.

// Injected is the configuration written into the page by internal/webui.
interface Injected {
  workspace: 'hub' | 'client'
  issuer?: string
  audience?: string
}

const injected = (window as unknown as { __ALEXANDRIA__?: Injected }).__ALEXANDRIA__

// runtime describes where the APIs live and which workspaces this page offers.
export const runtime = {
  embedded: !!injected,
  hasHub: !injected || injected.workspace === 'hub',
  hasClient: !injected || injected.workspace === 'client',
  base: import.meta.env.BASE_URL,
  hubBase: injected ? '' : '/api/hub',
  clientBase: injected ? '' : '/api/client',
  hubTarget: injected ? location.origin : __HUB_TARGET__,
  clientTarget: injected ? location.origin : __CLIENT_TARGET__,
  proxiedIssuer: injected ? '' : __IDP_TARGET__,
  defaultIssuer: injected ? (injected.issuer ?? '') : __IDP_TARGET__,
  defaultAudience: injected?.audience ?? 'alexandria-vhub',
}

// appPath strips the deployment base from a browser pathname, yielding a router path.
export function appPath(pathname: string): string {
  const base = runtime.base.replace(/\/$/, '')
  return base && pathname.startsWith(base) ? pathname.slice(base.length) || '/' : pathname
}
