// OAuth 2.0 / OpenID Connect public client: discovery, Authorization Code with PKCE (S256), refresh tokens and RP-initiated logout.
// Back-channel calls go through the /api/idp proxy in development and straight to the issuer when embedded; the resulting access token is what the hub validates (A1–A5).
import { decodeJwt } from '@/lib/jwt'
import { appPath, runtime } from '@/lib/runtime'
import { activeIdentity, getSettings, removeIdentity, saveIdentity, updateIdentity, type Identity, type OidcConfig } from '@/lib/settings'

// Discovery is the subset of OIDC provider metadata the console uses.
export interface Discovery {
  issuer: string
  authorization_endpoint?: string
  token_endpoint?: string
  userinfo_endpoint?: string
  end_session_endpoint?: string
  jwks_uri?: string
  code_challenge_methods_supported?: string[]
  grant_types_supported?: string[]
  scopes_supported?: string[]
}

// TokenResponse is the token endpoint success body (RFC 6749 §5.1).
interface TokenResponse {
  access_token: string
  token_type?: string
  expires_in?: number
  refresh_token?: string
  id_token?: string
}

// Pending is the login transaction kept in sessionStorage across the redirect.
interface Pending {
  state: string
  verifier: string
  nonce: string
  returnTo: string
  config: OidcConfig
}

const PENDING = 'alexandria.gui.oidc'

// trim removes trailing slashes from an issuer URL.
const trim = (u: string) => u.replace(/\/+$/, '')

// redirectUri is the registered callback of this single-page application.
export const redirectUri = () => `${location.origin}${runtime.base}auth/callback`

// viaProxy routes URLs of the dev-server issuer through /api/idp; an embedded console calls the issuer directly (CORS required).
function viaProxy(url: string): string {
  const base = trim(runtime.proxiedIssuer)
  return base && url.startsWith(base) ? `/api/idp${url.slice(new URL(base).origin.length)}` : url
}

// random returns n random bytes encoded as base64url.
function random(n = 32): string {
  return b64url(crypto.getRandomValues(new Uint8Array(n)))
}

// b64url encodes bytes without padding (RFC 7636 Appendix A).
function b64url(bytes: Uint8Array): string {
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

// challenge derives the S256 code challenge of a verifier.
async function challenge(verifier: string): Promise<string> {
  return b64url(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))))
}

// discover fetches and sanity-checks the provider metadata of an issuer.
export async function discover(issuer: string): Promise<Discovery> {
  const url = `${trim(issuer)}/.well-known/openid-configuration`
  let res: Response
  try {
    res = await fetch(viaProxy(url), { headers: { Accept: 'application/json' } })
  } catch {
    throw new Error(`Discovery document not reachable at ${url} (network or CORS).`)
  }
  if (!res.ok) throw new Error(`Discovery failed: ${url} answered ${res.status}.`)
  const doc = (await res.json().catch(() => null)) as Discovery | null
  if (!doc?.issuer) throw new Error('Discovery document is not valid OIDC provider metadata.')
  if (trim(doc.issuer) !== trim(issuer)) throw new Error(`Issuer mismatch: metadata declares ${doc.issuer}.`)
  return doc
}

// beginLogin starts Authorization Code + PKCE and redirects the browser to the provider.
export async function beginLogin(config: OidcConfig, returnTo: string): Promise<void> {
  const meta = await discover(config.issuer)
  if (!meta.authorization_endpoint || !meta.token_endpoint) {
    throw new Error('This issuer does not advertise authorization_endpoint/token_endpoint, so interactive sign-in is not available.')
  }
  const pending: Pending = { state: random(16), verifier: random(48), nonce: random(16), returnTo: appPath(returnTo), config }
  sessionStorage.setItem(PENDING, JSON.stringify(pending))
  const q = new URLSearchParams({
    response_type: 'code',
    client_id: config.clientId,
    redirect_uri: redirectUri(),
    scope: config.scope,
    state: pending.state,
    nonce: pending.nonce,
    code_challenge: await challenge(pending.verifier),
    code_challenge_method: 'S256',
  })
  if (config.audience) q.set('audience', config.audience)
  location.assign(`${meta.authorization_endpoint}${meta.authorization_endpoint.includes('?') ? '&' : '?'}${q}`)
}

// token posts a form-encoded grant to the token endpoint and returns the parsed response.
async function token(endpoint: string, form: Record<string, string>): Promise<TokenResponse> {
  const res = await fetch(viaProxy(endpoint), {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json' },
    body: new URLSearchParams(form),
  })
  const body = (await res.json().catch(() => ({}))) as TokenResponse & { error?: string; error_description?: string }
  if (!res.ok || !body.access_token) throw new Error(body.error_description || body.error || `Token endpoint answered ${res.status}.`)
  return body
}

// expiry computes the absolute expiry of an access token from expires_in or its exp claim.
function expiry(t: TokenResponse): number | undefined {
  if (t.expires_in) return Date.now() + t.expires_in * 1000
  const exp = decodeJwt(t.access_token)?.exp
  return exp ? exp * 1000 : undefined
}

// completeLogin validates the callback, redeems the code and stores the new identity; it returns where to navigate next.
export async function completeLogin(search: string): Promise<string> {
  const q = new URLSearchParams(search)
  const raw = sessionStorage.getItem(PENDING)
  sessionStorage.removeItem(PENDING)
  if (q.get('error')) throw new Error(q.get('error_description') || q.get('error') || 'Authorization failed.')
  const pending = raw ? (JSON.parse(raw) as Pending) : null
  const code = q.get('code')
  if (!pending || !code) throw new Error('No sign-in is in progress for this callback.')
  if (q.get('state') !== pending.state) throw new Error('State mismatch: the callback does not belong to this sign-in.')
  const { config } = pending
  const meta = await discover(config.issuer)
  const t = await token(meta.token_endpoint!, {
    grant_type: 'authorization_code',
    code,
    redirect_uri: redirectUri(),
    client_id: config.clientId,
    code_verifier: pending.verifier,
  })
  const id = t.id_token ? decodeJwt(t.id_token) : null
  if (id && (id.nonce !== pending.nonce || trim(String(id.iss)) !== trim(config.issuer))) throw new Error('ID token nonce or issuer mismatch.')
  const claims = id ?? decodeJwt(t.access_token)
  const label = String(claims?.preferred_username ?? claims?.email ?? claims?.sub ?? 'oidc user')
  saveIdentity({
    id: crypto.randomUUID(),
    kind: 'oidc',
    label,
    token: t.access_token,
    idToken: t.id_token,
    refreshToken: t.refresh_token,
    expiresAt: expiry(t),
    issuer: trim(config.issuer),
    clientId: config.clientId,
  })
  return pending.returnTo || '/'
}

let refreshing: Promise<string> | null = null

// refresh redeems the refresh token of an identity, single-flighted across concurrent requests.
function refresh(i: Identity): Promise<string> {
  refreshing ??= (async () => {
    try {
      const meta = await discover(i.issuer!)
      const t = await token(meta.token_endpoint!, { grant_type: 'refresh_token', refresh_token: i.refreshToken!, client_id: i.clientId! })
      updateIdentity(i.id, { token: t.access_token, refreshToken: t.refresh_token ?? i.refreshToken, idToken: t.id_token ?? i.idToken, expiresAt: expiry(t) })
      return t.access_token
    } catch {
      // A rejected refresh token ends the session; the stale access token then yields a visible 401.
      updateIdentity(i.id, { refreshToken: undefined })
      return i.token
    } finally {
      refreshing = null
    }
  })()
  return refreshing
}

// accessToken returns the bearer token of the active identity, refreshing it when it is about to expire.
export async function accessToken(): Promise<string | undefined> {
  const i = activeIdentity()
  if (!i) return undefined
  if (i.kind === 'oidc' && i.refreshToken && i.expiresAt && i.expiresAt - Date.now() < 30_000) return refresh(i)
  return i.token
}

// logout forgets an identity and, for OIDC sessions, performs RP-initiated logout when the provider supports it.
export async function logout(i: Identity): Promise<void> {
  removeIdentity(i.id)
  if (i.kind !== 'oidc' || !i.issuer) return
  const meta = await discover(i.issuer).catch(() => null)
  if (!meta?.end_session_endpoint) return
  const q = new URLSearchParams({ post_logout_redirect_uri: location.origin + runtime.base, client_id: i.clientId ?? '' })
  if (i.idToken) q.set('id_token_hint', i.idToken)
  location.assign(`${meta.end_session_endpoint}?${q}`)
}

// oidcDefaults exposes the current OIDC client configuration.
export const oidcConfig = (): OidcConfig => getSettings().oidc
