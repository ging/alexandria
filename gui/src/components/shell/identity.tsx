// Identity controls: the title-bar menu to switch principals and the dialog to sign in with OIDC or paste an access token.
// Interactive sign-in uses Authorization Code + PKCE against the configured issuer; pasted tokens cover lab IdPs without an authorize endpoint.
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ChevronDown, KeyRound, LogIn, LogOut, ShieldCheck, UserRound } from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/cn'
import { relativeTo } from '@/lib/format'
import { decodeJwt, isAdmin } from '@/lib/jwt'
import { beginLogin, discover, logout, redirectUri } from '@/lib/oidc'
import { addIdentity, describeIdentity, setSettings, useActiveIdentity, useSettings, type OidcConfig } from '@/lib/settings'
import { Button } from '@/components/ui/button'
import { Field, Input, Textarea } from '@/components/ui/form'
import { Badge, KV, Mono, Spinner, Tabs } from '@/components/ui/misc'
import { Dialog, Menu, MenuItem, MenuLabel, MenuSeparator } from '@/components/ui/overlay'

// IdentityMenu shows the active principal and lets the user switch, add or sign out identities.
export function IdentityMenu({ onAdd }: { onAdd: () => void }) {
  const { identities, activeIdentity } = useSettings()
  const active = useActiveIdentity()
  const qc = useQueryClient()
  const select = (id: string | null) => {
    setSettings({ activeIdentity: id })
    void qc.invalidateQueries({ queryKey: ['hub'] })
  }
  return (
    <Menu
      align="end"
      trigger={
        <button className="flex h-6 items-center gap-1.5 rounded-md border border-line-strong bg-bg px-2 text-xs hover:bg-hover">
          <UserRound className={cn('size-3.5', active ? 'text-accent dark:text-info' : 'text-subtle')} />
          <span className="max-w-32 truncate font-medium">{active ? active.label : 'Anonymous'}</span>
          {active?.admin ? <Badge tone="purple">admin</Badge> : null}
          {active?.expired ? <Badge tone="danger">expired</Badge> : null}
          <ChevronDown className="size-3 text-subtle" />
        </button>
      }
    >
      <MenuLabel>Identities</MenuLabel>
      <MenuItem icon={activeIdentity === null ? <Check className="size-3.5" /> : null} onSelect={() => select(null)}>
        Anonymous
      </MenuItem>
      {identities.map((i) => {
        const d = describeIdentity(i)
        return (
          <MenuItem key={i.id} icon={activeIdentity === i.id ? <Check className="size-3.5" /> : null} onSelect={() => select(i.id)} hint={d.expired ? 'expired' : d.kind === 'oidc' ? 'oidc' : d.admin ? 'admin' : undefined}>
            {i.label}
          </MenuItem>
        )
      })}
      <MenuSeparator />
      <MenuItem icon={<LogIn className="size-3.5" />} onSelect={onAdd}>
        Sign in or add token…
      </MenuItem>
      {active ? (
        <MenuItem
          danger
          icon={<LogOut className="size-3.5" />}
          onSelect={() => {
            void logout(active).then(() => qc.invalidateQueries({ queryKey: ['hub'] }))
          }}
        >
          Sign out {active.label}
        </MenuItem>
      ) : null}
    </Menu>
  )
}

// IdentityDialog hosts the OIDC sign-in form, the manual token form and the details of the active session.
export function IdentityDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [tab, setTab] = useState<'oidc' | 'token' | 'session'>('oidc')
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Identity" description="Authenticate to the hub management API (OAuth 2.0 bearer, RFC 9068)." width="max-w-xl">
      <Tabs
        className="-mx-4 -mt-3 mb-3"
        value={tab}
        onChange={setTab}
        tabs={[
          { id: 'oidc', label: 'Sign in with OIDC' },
          { id: 'token', label: 'Access token' },
          { id: 'session', label: 'Current session' },
        ]}
      />
      {tab === 'oidc' ? <OidcForm /> : tab === 'token' ? <TokenForm onDone={() => onOpenChange(false)} /> : <SessionView />}
    </Dialog>
  )
}

// OidcForm edits the client registration, previews discovery and starts Authorization Code + PKCE.
function OidcForm() {
  const { oidc } = useSettings()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const set = (patch: Partial<OidcConfig>) => setSettings((s) => ({ oidc: { ...s.oidc, ...patch } }))
  const meta = useQuery({ queryKey: ['oidc', 'discovery', oidc.issuer], queryFn: () => discover(oidc.issuer), enabled: !!oidc.issuer, retry: false, staleTime: 60_000 })
  const interactive = !!meta.data?.authorization_endpoint && !!meta.data?.token_endpoint
  const pkce = meta.data?.code_challenge_methods_supported
  const start = async () => {
    setBusy(true)
    setError(null)
    try {
      await beginLogin(oidc, location.pathname + location.search)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setBusy(false)
    }
  }
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-3">
        <Field label="Issuer" className="col-span-2" hint="Must match an issuer accepted by the hub (auth_config.issuer).">
          <Input mono value={oidc.issuer} onChange={(e) => set({ issuer: e.target.value.trim() })} placeholder="https://idp.example.com/realms/dataspace" />
        </Field>
        <Field label="Client ID" hint="Public client, no secret.">
          <Input mono value={oidc.clientId} onChange={(e) => set({ clientId: e.target.value.trim() })} />
        </Field>
        <Field label="Audience" hint="Optional audience parameter.">
          <Input mono value={oidc.audience} onChange={(e) => set({ audience: e.target.value.trim() })} />
        </Field>
        <Field label="Scopes" className="col-span-2">
          <Input mono value={oidc.scope} onChange={(e) => set({ scope: e.target.value })} />
        </Field>
      </div>

      <div className="rounded-md border bg-side p-3">
        <div className="mb-2 flex items-center gap-2">
          <span className="label-caps">Discovery</span>
          {meta.isFetching ? <Spinner className="size-3" /> : meta.isError ? <Badge tone="danger">failed</Badge> : meta.data ? <Badge tone={interactive ? 'success' : 'warning'} dot>{interactive ? 'ready' : 'no authorize endpoint'}</Badge> : null}
        </div>
        {meta.isError ? (
          <p className="text-sm text-danger">{(meta.error as Error).message}</p>
        ) : meta.data ? (
          <KV
            items={[
              ['Authorization', <Mono>{meta.data.authorization_endpoint ?? '—'}</Mono>],
              ['Token', <Mono>{meta.data.token_endpoint ?? '—'}</Mono>],
              ['JWKS', <Mono>{meta.data.jwks_uri ?? '—'}</Mono>],
              ['End session', <Mono>{meta.data.end_session_endpoint ?? '—'}</Mono>],
              ['PKCE', <span>{pkce ? pkce.join(', ') : 'not advertised (S256 is sent anyway)'}</span>],
              ['Redirect URI', <Mono copy={redirectUri()}>{redirectUri()}</Mono>],
            ]}
          />
        ) : null}
        {meta.data && !interactive ? (
          <p className="mt-2 text-xs text-muted">
            This issuer only publishes keys (as the lab <span className="font-mono">fake-idp</span> does). Mint a token with its CLI and use the <b>Access token</b> tab.
          </p>
        ) : null}
      </div>

      {error ? <p className="text-sm text-danger">{error}</p> : null}
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs text-subtle">Authorization Code flow with PKCE (S256), state and nonce. Tokens are refreshed silently when a refresh token is issued.</p>
        <Button variant="primary" icon={<LogIn className="size-3.5" />} disabled={!interactive || !oidc.clientId} loading={busy} onClick={start}>
          Sign in
        </Button>
      </div>
    </div>
  )
}

// TokenForm stores a pasted access token after previewing its claims.
function TokenForm({ onDone }: { onDone: () => void }) {
  const [token, setToken] = useState('')
  const [label, setLabel] = useState('')
  const qc = useQueryClient()
  const claims = token.trim() ? decodeJwt(token) : null
  return (
    <div className="space-y-3">
      <Field label="Access token (at+jwt)" error={token.trim() && !claims ? 'This does not look like a JWT.' : undefined}>
        <Textarea mono rows={5} value={token} onChange={(e) => setToken(e.target.value)} placeholder="eyJhbGciOiJFUzI1NiIsInR5cCI6ImF0K2p3dCIs…" className="break-all" />
      </Field>
      <Field label="Label" hint="Defaults to the token subject.">
        <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder={claims?.sub ?? 'ana'} />
      </Field>
      {claims ? <ClaimsView claims={claims} /> : null}
      <div className="rounded-md border bg-side p-2.5">
        <p className="label-caps mb-1">Lab tokens</p>
        <Mono copy="./bin/fake-idp token --sub ana --admin --ttl 24h" className="text-muted">./bin/fake-idp token --sub ana --admin --ttl 24h</Mono>
      </div>
      <div className="flex justify-end">
        <Button
          variant="primary"
          icon={<KeyRound className="size-3.5" />}
          disabled={!claims}
          onClick={() => {
            const i = addIdentity(token, label)
            void qc.invalidateQueries({ queryKey: ['hub'] })
            toast.success(`Identity “${i.label}” is now active`)
            onDone()
          }}
        >
          Use token
        </Button>
      </div>
    </div>
  )
}

// ClaimsView summarizes the claims of a token.
function ClaimsView({ claims }: { claims: NonNullable<ReturnType<typeof decodeJwt>> }) {
  const exp = claims.exp ? new Date(claims.exp * 1000).toISOString() : null
  return (
    <KV
      className="rounded-md border p-2.5"
      items={[
        ['Subject', <Mono>{claims.sub ?? '—'}</Mono>],
        ['Issuer', <Mono>{claims.iss ?? '—'}</Mono>],
        ['Audience', <Mono>{[claims.aud].flat().join(', ') || '—'}</Mono>],
        ['Roles', isAdmin(claims) ? <Badge tone="purple"><ShieldCheck className="size-3" /> vhub-admin</Badge> : <span className="text-muted">{claims.roles?.join(', ') || 'none'}</span>],
        ['Expires', exp ? <span className={claims.exp! * 1000 < Date.now() ? 'text-danger' : ''}>{relativeTo(exp)}</span> : '—'],
      ]}
    />
  )
}

// SessionView shows the claims and token material of the active identity.
function SessionView() {
  const active = useActiveIdentity()
  if (!active) return <p className="py-6 text-center text-sm text-muted">No identity is active. Requests are sent anonymously.</p>
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span className="text-base font-medium">{active.label}</span>
        <Badge tone={active.kind === 'oidc' ? 'info' : 'neutral'}>{active.kind === 'oidc' ? 'OIDC session' : 'pasted token'}</Badge>
        {active.refreshToken ? <Badge tone="success" dot>auto-refresh</Badge> : null}
        {active.expired ? <Badge tone="danger">expired</Badge> : null}
      </div>
      {active.claims ? <ClaimsView claims={active.claims} /> : <p className="text-sm text-muted">The access token is opaque; no claims can be shown.</p>}
      <Field label="Access token">
        <Textarea mono rows={4} readOnly value={active.token} className="break-all text-muted" />
      </Field>
    </div>
  )
}
