// Namespace overview: identity of the namespace, headline figures of its release line and steward management.
// Stewards are the principals (iss + sub) allowed to author, release and revoke inside the namespace.
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { BookOpen, GitBranch, Trash2, UserPlus, Users } from 'lucide-react'
import type { ReactNode } from 'react'
import { formatInstant, iriTail, relativeTo } from '@/lib/format'
import { useAction, useNs } from '@/lib/hooks'
import { hub, hubQueries, type Steward } from '@/lib/hub'
import { useActiveIdentity } from '@/lib/settings'
import { asApiError } from '@/lib/http'
import { ClassBadge, Timeline } from '@/components/lifecycle'
import { ErrorState, ProblemBanner } from '@/components/problem'
import { Button, IconButton } from '@/components/ui/button'
import { Field, Input } from '@/components/ui/form'
import { Empty, KV, Mono, Page, Section, Spinner } from '@/components/ui/misc'
import { Dialog } from '@/components/ui/overlay'

// OverviewPage renders the namespace dashboard.
export function OverviewPage() {
  const slug = useNs()
  const nsq = useQuery(hubQueries.namespace(slug))
  const vocab = useQuery(hubQueries.vocabularies(slug))
  const listing = useQuery(hubQueries.listing(slug))
  if (nsq.isPending) return <div className="flex h-40 items-center justify-center"><Spinner /></div>
  if (nsq.isError) return <ErrorState error={nsq.error} />
  const n = nsq.data
  const versions = listing.data?.credentialSubject.versions ?? []
  const head = versions.filter((v) => !v.revoked_from).at(-1)
  return (
    <Page title={<span className="flex items-center gap-2">{n.title || n.slug} <Mono className="font-normal text-muted">{n.slug}</Mono></span>} subtitle={n.description}>
      <div className="mx-auto max-w-5xl space-y-4">
        <div className="grid grid-cols-3 gap-3">
          <Stat icon={<BookOpen />} label="Vocabularies" value={vocab.data?.length ?? '—'} to={`/hub/n/${slug}/vocabularies`} />
          <Stat icon={<GitBranch />} label="Published versions" value={listing.isError ? 0 : versions.length || '—'} to={`/hub/n/${slug}/versions`} hint={head ? <>latest <Mono>{head.version}</Mono> <ClassBadge value={head.class} /></> : 'no release yet'} />
          <Stat icon={<GitBranch />} label="Listing sequence" value={listing.data?.credentialSubject.sequence ?? '—'} to={`/hub/n/${slug}/versions`} hint={listing.data?.validUntil ? `valid until ${relativeTo(listing.data.validUntil)}` : undefined} />
        </div>

        <Section title="Namespace">
          <KV
            items={[
              ['IRI', <Mono copy={n.id}>{n.id}</Mono>],
              ['Slug', <Mono>{n.slug}</Mono>],
              ['Created', <span className="tnum">{formatInstant(n.created_at)}</span>],
              ['Head redirect', <Mono copy={`/pub/${n.slug}`}>GET /pub/{n.slug}</Mono>],
            ]}
          />
        </Section>

        {versions.length ? (
          <Section title="Release line">
            <Timeline records={versions} />
          </Section>
        ) : null}

        <Stewards slug={slug} />
      </div>
    </Page>
  )
}

// Stat renders one headline figure linking to its detailed view.
function Stat({ icon, label, value, hint, to }: { icon: ReactNode; label: string; value: ReactNode; hint?: ReactNode; to: string }) {
  return (
    <Link to={to} className="group rounded-lg border p-3 transition-colors hover:border-accent/60 hover:bg-side">
      <div className="flex items-center gap-1.5 text-xs text-muted [&>svg]:size-3.5">{icon}{label}</div>
      <div className="tnum mt-1 text-2xl font-semibold tracking-tight">{value}</div>
      <div className="mt-0.5 flex h-[18px] items-center gap-1.5 text-xs text-subtle">{hint}</div>
    </Link>
  )
}

// Stewards lists, assigns and removes the stewards of a namespace.
function Stewards({ slug }: { slug: string }) {
  const q = useQuery({ ...hubQueries.stewards(slug), retry: false })
  const [adding, setAdding] = useState(false)
  const remove = useAction((id: string) => hub.removeSteward(slug, id), { success: 'Steward removed', invalidate: [['hub', 'ns', slug, 'stewards']] })
  const denied = q.isError && [401, 403].includes(asApiError(q.error).status)
  return (
    <Section
      title="Stewards"
      actions={
        <Button size="sm" variant="ghost" icon={<UserPlus className="size-3.5" />} onClick={() => setAdding(true)}>
          Assign steward
        </Button>
      }
    >
      {q.isPending ? (
        <Spinner />
      ) : denied ? (
        <p className="text-sm text-muted">Stewards are visible to the hub admin and to the stewards of this namespace. Sign in with a suitable identity from the menu at the top right.</p>
      ) : q.isError ? (
        <ProblemBanner error={q.error} />
      ) : q.data.length === 0 ? (
        <Empty icon={<Users />} title="No stewards assigned">Namespace operations (vocabularies, drafts, releases) require a steward; the hub admin assigns them.</Empty>
      ) : (
        <ul className="-my-1 divide-y">
          {q.data.map((s: Steward) => (
            <li key={s.id} className="flex items-center gap-3 py-2">
              <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-accent-soft text-xs font-semibold text-accent uppercase">{(s.label || s.sub).slice(0, 2)}</div>
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">{s.label || s.sub}</p>
                <p className="truncate font-mono text-xs text-muted">{s.iss}#{s.sub}</p>
              </div>
              <span className="text-xs text-subtle">assigned {relativeTo(s.assigned_at)}</span>
              <IconButton label="Remove steward" size="sm" loading={remove.isPending && remove.variables === iriTail(s.id)} onClick={() => remove.mutate(iriTail(s.id))}>
                <Trash2 className="size-3.5" />
              </IconButton>
            </li>
          ))}
        </ul>
      )}
      <AssignSteward slug={slug} open={adding} onOpenChange={setAdding} />
    </Section>
  )
}

// AssignSteward is the dialog that binds an issuer and subject to the namespace.
function AssignSteward({ slug, open, onOpenChange }: { slug: string; open: boolean; onOpenChange: (o: boolean) => void }) {
  const me = useActiveIdentity()
  const [iss, setIss] = useState('')
  const [sub, setSub] = useState('')
  const [label, setLabel] = useState('')
  const issuer = iss || me?.claims?.iss || ''
  const assign = useAction(() => hub.assignSteward(slug, { iss: issuer, sub, label: label || undefined }), {
    success: `Steward ${sub} assigned`,
    invalidate: [['hub', 'ns', slug, 'stewards']],
    onSuccess: () => { onOpenChange(false); setSub(''); setLabel('') },
  })
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Assign steward"
      description={<>Grants namespace operations on <Mono>{slug}</Mono> to a token principal.</>}
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="primary" disabled={!issuer || !sub} loading={assign.isPending} onClick={() => assign.mutate(undefined)}>Assign</Button>
        </>
      }
    >
      <div className="space-y-3">
        <Field label="Issuer (iss)" hint="Defaults to the issuer of the active identity.">
          <Input mono value={iss} onChange={(e) => setIss(e.target.value)} placeholder={me?.claims?.iss ?? 'https://idp.example.com'} />
        </Field>
        <Field label="Subject (sub)">
          <Input mono autoFocus value={sub} onChange={(e) => setSub(e.target.value)} placeholder="sergio" />
        </Field>
        <Field label="Label">
          <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Sergio — policy steward" />
        </Field>
      </div>
    </Dialog>
  )
}
