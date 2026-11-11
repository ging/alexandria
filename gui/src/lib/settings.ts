// Persistent console settings: theme, saved identities (bearer tokens), last namespace and locally remembered ids.
// A tiny external store backed by localStorage keeps React and the non-React fetch layer in sync.
import { useSyncExternalStore } from 'react'
import { decodeJwt, isAdmin, type Claims } from '@/lib/jwt'
import { runtime } from '@/lib/runtime'

// Theme is the color scheme preference.
export type Theme = 'light' | 'dark' | 'system'

// Identity is one saved bearer token with a user-facing label.
export interface Identity {
  id: string
  label: string
  token: string
  kind?: 'manual' | 'oidc'
  issuer?: string
  clientId?: string
  refreshToken?: string
  idToken?: string
  expiresAt?: number
}

// OidcConfig is the OAuth 2.0 / OIDC client registration used for interactive sign-in.
export interface OidcConfig {
  issuer: string
  clientId: string
  scope: string
  audience: string
}

// Settings is the persisted state of the console.
export interface Settings {
  theme: Theme
  identities: Identity[]
  activeIdentity: string | null
  namespace: string | null
  agreements: string[]
  oidc: OidcConfig
}

const KEY = 'alexandria.gui'
const defaults: Settings = {
  theme: 'system',
  identities: [],
  activeIdentity: null,
  namespace: null,
  agreements: [],
  oidc: { issuer: runtime.defaultIssuer, clientId: 'alexandria-console', scope: 'openid profile offline_access', audience: runtime.defaultAudience },
}

// load reads the persisted settings, tolerating missing or corrupt storage.
function load(): Settings {
  try {
    return { ...defaults, ...(JSON.parse(localStorage.getItem(KEY) ?? '{}') as Partial<Settings>) }
  } catch {
    return defaults
  }
}

let state = load()
const listeners = new Set<() => void>()

// getSettings returns the current settings snapshot.
export function getSettings(): Settings {
  return state
}

// setSettings merges a patch, persists it and notifies subscribers.
export function setSettings(patch: Partial<Settings> | ((s: Settings) => Partial<Settings>)): void {
  state = { ...state, ...(typeof patch === 'function' ? patch(state) : patch) }
  localStorage.setItem(KEY, JSON.stringify(state))
  listeners.forEach((l) => l())
}

// subscribe registers a store listener.
function subscribe(l: () => void): () => void {
  listeners.add(l)
  return () => listeners.delete(l)
}

// useSettings subscribes a component to the settings store.
export function useSettings(): Settings {
  return useSyncExternalStore(subscribe, getSettings)
}

// activeIdentity returns the active identity record, if any.
export function activeIdentity(): Identity | undefined {
  return state.identities.find((i) => i.id === state.activeIdentity)
}

// updateIdentity patches one saved identity in place (used by token refresh).
export function updateIdentity(id: string, patch: Partial<Identity>): void {
  setSettings((s) => ({ identities: s.identities.map((i) => (i.id === id ? { ...i, ...patch } : i)) }))
}

// saveIdentity stores a full identity record and makes it active, replacing one with the same issuer and subject label.
export function saveIdentity(identity: Identity): void {
  setSettings((s) => ({
    identities: [...s.identities.filter((i) => !(i.kind === 'oidc' && i.issuer === identity.issuer && i.label === identity.label)), identity],
    activeIdentity: identity.id,
  }))
}

// IdentityInfo is an identity enriched with its decoded claims.
export interface IdentityInfo extends Identity {
  claims: Claims | null
  admin: boolean
  expired: boolean
}

// describeIdentity decodes the token of an identity for display.
export function describeIdentity(i: Identity): IdentityInfo {
  const claims = decodeJwt(i.token) ?? (i.idToken ? decodeJwt(i.idToken) : null)
  const exp = i.expiresAt ?? (claims?.exp ? claims.exp * 1000 : undefined)
  return { ...i, claims, admin: isAdmin(decodeJwt(i.token)), expired: !i.refreshToken && !!exp && exp < Date.now() }
}

// useActiveIdentity returns the active identity with decoded claims, or null when anonymous.
export function useActiveIdentity(): IdentityInfo | null {
  const s = useSettings()
  const i = s.identities.find((x) => x.id === s.activeIdentity)
  return i ? describeIdentity(i) : null
}

// addIdentity stores a token, labels it from its subject and makes it active.
export function addIdentity(token: string, label?: string): Identity {
  // Tokens copied from a wrapped terminal line carry newlines or spaces that would invalidate the signature.
  const clean = token.replace(/\s+/g, '')
  const claims = decodeJwt(clean)
  const id = crypto.randomUUID()
  const identity: Identity = { id, kind: 'manual', token: clean, label: label?.trim() || claims?.sub || 'token' }
  setSettings((s) => ({ identities: [...s.identities, identity], activeIdentity: id }))
  return identity
}

// removeIdentity deletes a saved identity and clears the active one when it matches.
export function removeIdentity(id: string): void {
  setSettings((s) => ({
    identities: s.identities.filter((i) => i.id !== id),
    activeIdentity: s.activeIdentity === id ? null : s.activeIdentity,
  }))
}

// applyTheme toggles the .dark class according to the preference and the OS scheme.
export function applyTheme(theme: Theme): void {
  const dark = theme === 'dark' || (theme === 'system' && matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
}

// useIsDark reports whether the dark scheme is currently effective.
export function useIsDark(): boolean {
  const { theme } = useSettings()
  const system = useSyncExternalStore(
    (l) => {
      const mq = matchMedia('(prefers-color-scheme: dark)')
      mq.addEventListener('change', l)
      return () => mq.removeEventListener('change', l)
    },
    () => matchMedia('(prefers-color-scheme: dark)').matches,
  )
  return theme === 'dark' || (theme === 'system' && system)
}
