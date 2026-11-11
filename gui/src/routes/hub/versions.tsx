// Versions view: the signed VersionListing credential of a namespace, its lifecycle timeline and per-version records.
// Stewards can revoke a published version; every document shown is the exact credential signed by the hub.
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Ban, FileBadge, GitBranch, RefreshCw } from 'lucide-react'
import { formatInstant, prettyJSON, relativeTo, shortDigest } from '@/lib/format'
import { useAction, useNow, useNs } from '@/lib/hooks'
import { asApiError } from '@/lib/http'
import { hub, hubQueries, type Credential, type VersionRecord } from '@/lib/hub'
import { CodeEditor } from '@/components/code-editor'
import { DataTable, type Column } from '@/components/data-table'
import { DetailPanel, Split } from '@/components/detail-panel'
import { ClassBadge, Timeline } from '@/components/lifecycle'
import { ErrorState } from '@/components/problem'
import { Button, IconButton } from '@/components/ui/button'
import { Field, Input, Textarea } from '@/components/ui/form'
import { Badge, Empty, KV, Mono, Page, Spinner, Tabs } from '@/components/ui/misc'
import { Dialog } from '@/components/ui/overlay'

const instant = (key: keyof VersionRecord, header: string): Column<VersionRecord> => ({
  accessorKey: key,
  header,
  cell: (c) => <span className="tnum text-muted" title={relativeTo(c.row.original[key] as string)}>{formatInstant(c.row.original[key] as string | null)}</span>,
})

const columns: Column<VersionRecord>[] = [
  { accessorKey: 'version', header: 'Version', cell: (c) => <span className="font-mono text-xs font-semibold">{c.row.original.version}</span>, meta: { className: 'w-20' } },
  { accessorKey: 'class', header: 'Class', cell: (c) => <ClassBadge value={c.row.original.class} />, meta: { className: 'w-24' } },
  instant('published_at', 'Published'),
  instant('new_content_from', 'New content from'),
  instant('enforced_from', 'Enforced from'),
  { accessorKey: 'revoked_from', header: 'Revoked from', cell: (c) => (c.row.original.revoked_from ? <span className="tnum text-danger">{formatInstant(c.row.original.revoked_from)}</span> : <span className="text-subtle">—</span>) },
  { accessorKey: 'manifest', header: 'Manifest', enableSorting: false, cell: (c) => <Mono className="text-muted">{shortDigest(c.row.original.manifest)}</Mono>, meta: { className: 'w-36' } },
]

// VersionsPage renders the listing credential and its versions.
export function VersionsPage() {
  const slug = useNs()
  const q = useQuery(hubQueries.listing(slug))
  const now = useNow(30_000)
  const [selected, setSelected] = useState<string | null>(null)
  const [raw, setRaw] = useState(false)
  const none = q.isError && asApiError(q.error).status === 404
  const listing = q.data
  const versions = listing?.credentialSubject.versions ?? []
  return (
    <Page
      flush
      title="Versions"
      subtitle="Signed VersionListing (W3C VC 2.0, eddsa-jcs-2022). Clients synchronize from this document."
      actions={
        <>
          {listing ? <Button icon={<FileBadge className="size-3.5" />} onClick={() => setRaw(true)}>Listing credential</Button> : null}
          <IconButton label="Refresh" onClick={() => void q.refetch()}><RefreshCw className={q.isFetching ? 'size-3.5 animate-spin' : 'size-3.5'} /></IconButton>
        </>
      }
    >
      {q.isPending ? (
        <div className="flex h-40 items-center justify-center"><Spinner /></div>
      ) : none ? (
        <Empty icon={<GitBranch />} title="No version published yet">Approve a candidate in Authoring to issue the first signed listing.</Empty>
      ) : q.isError ? (
        <ErrorState error={q.error} />
      ) : listing ? (
        <Split detail={selected ? <RecordDetail key={selected} slug={slug} version={selected} onClose={() => setSelected(null)} /> : undefined}>
          <div className="grid grid-cols-4 gap-px border-b bg-line">
            {[
              ['Sequence', <span className="tnum font-mono">{listing.credentialSubject.sequence}</span>],
              ['Valid from', <span className="tnum">{formatInstant(listing.validFrom)}</span>],
              ['Valid until', <span className="tnum">{formatInstant(listing.validUntil)} <span className="text-subtle">({relativeTo(listing.validUntil, now)})</span></span>],
              ['Issuer', <Mono copy={listing.issuer}>{shortDigest(listing.issuer, 22)}</Mono>],
            ].map(([k, v]) => (
              <div key={k as string} className="min-w-0 bg-bg px-4 py-2">
                <p className="text-xs text-muted">{k}</p>
                <div className="mt-0.5 truncate font-medium">{v}</div>
              </div>
            ))}
          </div>
          <div className="border-b px-4 py-3">
            <Timeline records={versions} now={now} selected={selected} onSelect={setSelected} />
          </div>
          <DataTable minWidth={980} data={versions} columns={columns} rowId={(v) => v.version} selected={selected} onRowClick={(v) => setSelected(v.version)} />
        </Split>
      ) : null}
      {listing ? (
        <Dialog open={raw} onOpenChange={setRaw} width="max-w-3xl" title="VersionListing credential" description="Exactly as signed and stored by the hub.">
          <CodeEditor readOnly copy value={prettyJSON(listing)} maxHeight="60vh" />
        </Dialog>
      ) : null}
    </Page>
  )
}

// RecordDetail shows one signed VersionRecord with its proof and the revoke action.
function RecordDetail({ slug, version, onClose }: { slug: string; version: string; onClose: () => void }) {
  const q = useQuery(hubQueries.record(slug, version))
  const [tab, setTab] = useState<'summary' | 'json'>('summary')
  const [revoking, setRevoking] = useState(false)
  return (
    <DetailPanel
      title={<span className="font-mono text-sm">{version}</span>}
      onClose={onClose}
      actions={q.data && !q.data.credentialSubject.revoked_from ? <Button size="sm" variant="danger" icon={<Ban className="size-3.5" />} onClick={() => setRevoking(true)}>Revoke</Button> : null}
    >
      {q.isPending ? (
        <div className="p-4"><Spinner /></div>
      ) : q.isError ? (
        <ErrorState error={q.error} />
      ) : (
        <>
          <Tabs className="px-3" value={tab} onChange={setTab} tabs={[{ id: 'summary', label: 'Record' }, { id: 'json', label: 'Credential JSON' }]} />
          {tab === 'summary' ? <RecordSummary vc={q.data} /> : <div className="p-3"><CodeEditor readOnly copy value={prettyJSON(q.data)} maxHeight="70vh" /></div>}
        </>
      )}
      <Revoke slug={slug} version={version} open={revoking} onOpenChange={setRevoking} />
    </DetailPanel>
  )
}

// RecordSummary renders the subject and proof of a VersionRecord credential.
function RecordSummary({ vc }: { vc: Credential<VersionRecord> }) {
  const r = vc.credentialSubject
  return (
    <div className="space-y-4 p-3">
      <KV
        items={[
          ['Class', <ClassBadge value={r.class} />],
          ['Status', r.revoked_from ? <Badge tone="danger" dot>revoked</Badge> : <Badge tone="success" dot>published</Badge>],
          ['Published', <span className="tnum">{formatInstant(r.published_at)}</span>],
          ['New content from', <span className="tnum">{formatInstant(r.new_content_from)} <span className="text-subtle">· {relativeTo(r.new_content_from)}</span></span>],
          ['Enforced from', <span className="tnum">{formatInstant(r.enforced_from)} <span className="text-subtle">· {relativeTo(r.enforced_from)}</span></span>],
          ['Revoked from', r.revoked_from ? <span className="tnum text-danger">{formatInstant(r.revoked_from)}</span> : '—'],
          ['Manifest', <Mono copy={r.manifest}>{shortDigest(r.manifest, 26)}</Mono>],
          ['Record IRI', <Mono copy={r.id}>{r.id}</Mono>],
        ]}
      />
      <div>
        <p className="label-caps mb-1.5">Proof</p>
        <KV
          className="rounded-md border bg-side p-2.5"
          items={[
            ['Cryptosuite', <Mono>{vc.proof.cryptosuite}</Mono>],
            ['Created', <span className="tnum">{formatInstant(vc.proof.created)}</span>],
            ['Verification method', <Mono copy={vc.proof.verificationMethod}>{shortDigest(vc.proof.verificationMethod, 30)}</Mono>],
            ['Proof value', <Mono copy={vc.proof.proofValue}>{shortDigest(vc.proof.proofValue, 26)}</Mono>],
          ]}
        />
      </div>
    </div>
  )
}

// Revoke is the dialog that schedules the revocation of a published version.
function Revoke({ slug, version, open, onOpenChange }: { slug: string; version: string; open: boolean; onOpenChange: (o: boolean) => void }) {
  const [justification, setJustification] = useState('')
  const [from, setFrom] = useState('')
  const revoke = useAction(() => hub.revokeVersion(slug, version, { justification, revoked_from: from || undefined }), {
    success: `Version ${version} revoked`,
    invalidate: [['hub', 'ns', slug]],
    onSuccess: () => onOpenChange(false),
  })
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Revoke version ${version}`}
      description="Revocation is permanent: the record is re-signed with revoked_from and clients will reject agreements bound to it."
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="danger" disabled={!justification.trim()} loading={revoke.isPending} onClick={() => revoke.mutate(undefined)}>Revoke version</Button>
        </>
      }
    >
      <div className="space-y-3">
        <Field label="Justification" hint="Required (REV-2).">
          <Textarea rows={3} autoFocus value={justification} onChange={(e) => setJustification(e.target.value)} />
        </Field>
        <Field label="Revoked from" hint="Optional RFC 3339 instant; defaults to now + propagation window (REV-3).">
          <Input mono value={from} onChange={(e) => setFrom(e.target.value)} placeholder="2026-12-01T10:00:00Z" />
        </Field>
      </div>
    </Dialog>
  )
}
