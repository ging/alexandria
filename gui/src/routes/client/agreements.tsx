// Agreements view: looks up a captured agreement binding by id and reports whether it is still usable for transfers.
// The admin API has no listing, so recently inspected ids are remembered locally for quick access.
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Handshake, Search, X } from 'lucide-react'
import { client, type Agreement } from '@/lib/client'
import { formatInstant, shortDigest } from '@/lib/format'
import { asApiError } from '@/lib/http'
import { setSettings, useSettings } from '@/lib/settings'
import { ProblemBanner } from '@/components/problem'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/form'
import { Badge, Empty, KV, Mono, Page, Section, Spinner } from '@/components/ui/misc'

// AgreementsPage renders the lookup form, recent ids and the binding details.
export function AgreementsPage() {
  const { agreements } = useSettings()
  const [draft, setDraft] = useState('')
  const [id, setId] = useState<string | null>(null)
  const q = useQuery({
    queryKey: ['client', 'agreement', id],
    enabled: !!id,
    retry: false,
    queryFn: async () => {
      const r = (await client.agreement(id!)).data
      setSettings((s) => ({ agreements: [id!, ...s.agreements.filter((a) => a !== id)].slice(0, 12) }))
      return r
    },
  })
  const lookup = (v: string) => { const t = v.trim(); if (t) { setDraft(t); setId(t) } }
  return (
    <Page title="Agreements" subtitle="Agreement bindings captured from ContractAgreement messages; transfers are admitted against the bound version.">
      <div className="mx-auto max-w-3xl space-y-4">
        <form className="flex gap-2" onSubmit={(e) => { e.preventDefault(); lookup(draft) }}>
          <Input mono className="h-8" value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="urn:uuid:agreement-id" aria-label="Agreement id" />
          <Button type="submit" variant="primary" className="h-8" icon={<Search className="size-3.5" />} loading={q.isFetching}>Look up</Button>
        </form>
        {agreements.length ? (
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="label-caps mr-1">Recent</span>
            {agreements.map((a) => (
              <span key={a} className="group inline-flex h-5 items-center rounded-sm border bg-bg text-xs">
                <button className="px-1.5 font-mono text-muted hover:text-fg" onClick={() => lookup(a)}>{a}</button>
                <button aria-label={`Forget ${a}`} className="pr-1 text-subtle hover:text-danger" onClick={() => setSettings((s) => ({ agreements: s.agreements.filter((x) => x !== a) }))}><X className="size-3" /></button>
              </span>
            ))}
          </div>
        ) : null}
        {!id ? (
          <Empty icon={<Handshake />} title="Inspect an agreement">Enter the agreement id carried by the DSP ContractAgreementMessage.</Empty>
        ) : q.isPending ? (
          <Spinner />
        ) : q.isError ? (
          asApiError(q.error).status === 404 ? <Empty icon={<Handshake />} title="Agreement not found">The client has not captured an agreement with id <span className="font-mono">{id}</span>.</Empty> : <ProblemBanner error={q.error} />
        ) : (
          <AgreementCard a={q.data} />
        )}
      </div>
    </Page>
  )
}

// AgreementCard renders one binding.
function AgreementCard({ a }: { a: Agreement }) {
  return (
    <Section title="Agreement binding" actions={a.usable ? <Badge tone="success" dot>usable</Badge> : <Badge tone="danger" dot>not usable · version revoked</Badge>}>
      <KV
        items={[
          ['Agreement id', <Mono copy={a.agreement_id}>{a.agreement_id}</Mono>],
          ['Bound version', <span className="font-mono text-xs font-semibold">{a.version}</span>],
          ['Manifest', <Mono copy={a.manifest}>{shortDigest(a.manifest, 26)}</Mono>],
          ['Role', <Badge>{a.role}</Badge>],
          ['Provider PID', <Mono>{a.provider_pid || '—'}</Mono>],
          ['Consumer PID', <Mono>{a.consumer_pid || '—'}</Mono>],
          ['Declared profile', <Mono>{a.declared_profile ?? '—'}</Mono>],
          ['Recorded', <span className="tnum">{formatInstant(a.recorded_at)}</span>],
          ['Source', <Badge tone="info">{a.source}</Badge>],
        ]}
      />
    </Section>
  )
}
