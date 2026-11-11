// Client status dashboard: operational state machine, listing freshness, hub trust anchor, probes and local version roles.
// Data comes from GET /status, /healthz and /readyz on the admin listener and refreshes every few seconds.
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'
import { clientQueries, type ClientState } from '@/lib/client'
import { formatInstant, formatSpan, iriTail, relativeTo, shortDigest } from '@/lib/format'
import { useNow } from '@/lib/hooks'
import { runtime } from '@/lib/runtime'
import { StateBadge, stateTone } from '@/components/lifecycle'
import { ErrorState } from '@/components/problem'
import { Badge, Dot, KV, Mono, Page, Section, Spinner } from '@/components/ui/misc'

const states: { id: ClientState; hint: string }[] = [
  { id: 'Onboarding', hint: 'No listing accepted yet' },
  { id: 'Active', hint: 'Fresh listing, single head' },
  { id: 'Transition', hint: 'Grace window in progress' },
  { id: 'Degraded', hint: 'Listing expired, within staleness' },
  { id: 'FailClosed', hint: 'Enforce routes reject' },
]

const toneText: Record<string, string> = { success: 'text-success', warning: 'text-warning', danger: 'text-danger', info: 'text-info', purple: 'text-purple', neutral: 'text-muted', accent: 'text-accent' }

// StatusPage renders the client dashboard.
export function StatusPage() {
  const q = useQuery(clientQueries.status())
  const health = useQuery(clientQueries.health())
  const now = useNow(1000)
  if (q.isPending) return <div className="flex h-40 items-center justify-center"><Spinner /></div>
  if (q.isError) return <Page title="Status"><ErrorState error={q.error} /></Page>
  const s = q.data
  const l = s.last_accepted_listing
  const from = l ? new Date(l.valid_from).getTime() : 0
  const until = l ? new Date(l.valid_until).getTime() : 0
  const elapsed = l ? Math.min(1, Math.max(0, (now - from) / (until - from))) : 0
  const expired = l ? now >= until : false
  const skew = Math.abs(s.clock_skew_estimate_ms)
  return (
    <Page title="Status" subtitle={<>Edge proxy for namespace <span className="font-mono">{iriTail(s.namespace)}</span> · refreshed every 5 s</>} actions={<StateBadge state={s.state} />}>
      <div className="mx-auto max-w-5xl space-y-4">
        <Section title="Operational state">
          <ol className="flex items-stretch gap-1">
            {states.map((st, i) => {
              const active = st.id === s.state
              return (
                <li key={st.id} className="flex min-w-0 flex-1 items-center gap-1">
                  <div className={cn('min-w-0 flex-1 rounded-md border px-2.5 py-2 transition-colors', active ? 'border-current bg-side ' + toneText[stateTone(st.id)] : 'border-line opacity-60')}>
                    <div className="flex items-center gap-1.5">
                      <Dot tone={active ? stateTone(st.id) : 'neutral'} pulse={active} />
                      <span className={cn('truncate font-medium', active ? 'text-fg' : 'text-muted')}>{st.id}</span>
                    </div>
                    <p className="mt-0.5 truncate text-xs text-subtle">{st.hint}</p>
                  </div>
                  {i < states.length - 1 ? <ChevronRight className="size-3 shrink-0 text-subtle" /> : null}
                </li>
              )
            })}
          </ol>
        </Section>

        <div className="grid grid-cols-4 gap-3">
          <Tile label="Liveness" value={<Probe ok={health.data?.live} />} hint="GET /healthz" />
          <Tile label="Readiness" value={<Probe ok={health.data?.ready} />} hint="GET /readyz" />
          <Tile label="Clock skew" value={<span className={skew > 5000 ? 'text-warning' : ''}>{s.clock_skew_estimate_ms} ms</span>} hint="estimated against hub Date" />
          <Tile label="Rule set" value={<Link to="/client/rules" className="hover:text-accent">{s.rules.count} rules</Link>} hint={`revision ${s.rules.revision}`} />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <Section title="Last accepted listing">
            {l ? (
              <div className="space-y-3">
                <KV
                  items={[
                    ['Sequence', <span className="tnum font-mono">{l.sequence}</span>],
                    ['Valid from', <span className="tnum">{formatInstant(l.valid_from)}</span>],
                    ['Valid until', <span className="tnum">{formatInstant(l.valid_until)}</span>],
                  ]}
                />
                <div>
                  <div className="mb-1 flex justify-between text-xs">
                    <span className="text-muted">Freshness</span>
                    <span className={expired ? 'font-medium text-danger' : 'text-muted'}>{expired ? `expired ${formatSpan(now - until)} ago` : `expires ${relativeTo(l.valid_until, now)}`}</span>
                  </div>
                  <div className="h-1.5 overflow-hidden rounded-full bg-active">
                    <div className={cn('h-full rounded-full transition-all', expired ? 'bg-danger' : elapsed > 0.8 ? 'bg-warning' : 'bg-success')} style={{ width: `${elapsed * 100}%` }} />
                  </div>
                </div>
              </div>
            ) : (
              <p className="text-sm text-muted">No listing has passed synchronization (S1–S13) yet. The client stays in Onboarding until the hub publishes a version and the DID resolves.</p>
            )}
          </Section>
          <Section title="Hub trust anchor">
            <KV
              items={[
                ['Hub DID', <Mono copy={s.hub_did.did}>{shortDigest(s.hub_did.did, 30)}</Mono>],
                ['DID document', s.hub_did.resolved_at ? <span>resolved {relativeTo(s.hub_did.resolved_at, now)}</span> : <Badge tone="warning">not resolved</Badge>],
                ['Namespace', <Mono copy={s.namespace}>{s.namespace}</Mono>],
                ['Admin listener', <Mono>{runtime.clientTarget}</Mono>],
              ]}
            />
          </Section>
        </div>

        <Section title="Local versions" actions={<Link to="/client/versions" className="text-xs text-accent hover:underline dark:text-info">All lifecycle records</Link>}>
          {s.versions.length === 0 ? (
            <p className="text-sm text-muted">No version packages held.</p>
          ) : (
            <ul className="-my-1 divide-y">
              {s.versions.map((v) => (
                <li key={v.manifest} className="flex items-center gap-3 py-1.5">
                  <span className="w-16 font-mono text-xs font-semibold">{v.version}</span>
                  <StateBadge state={v.local_role} />
                  <Mono copy={v.manifest} className="ml-auto text-muted">{shortDigest(v.manifest, 20)}</Mono>
                </li>
              ))}
            </ul>
          )}
        </Section>
      </div>
    </Page>
  )
}

// Tile renders one small headline figure.
function Tile({ label, value, hint }: { label: string; value: ReactNode; hint?: string }) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-xs text-muted">{label}</p>
      <div className="tnum mt-1 text-xl font-semibold tracking-tight">{value}</div>
      <p className="mt-0.5 font-mono text-2xs text-subtle">{hint}</p>
    </div>
  )
}

// Probe renders the outcome of a health probe.
function Probe({ ok }: { ok?: boolean }) {
  if (ok === undefined) return <span className="text-subtle">—</span>
  return <span className={cn('inline-flex items-center gap-1.5', ok ? 'text-success' : 'text-danger')}><Dot tone={ok ? 'success' : 'danger'} />{ok ? 'OK' : 'Failing'}</span>
}
