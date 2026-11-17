// Versions view: the signed VersionListing credential of a namespace, its lifecycle timeline and per-version records.
// Stewards can revoke a published version; every document shown is the exact credential signed by the hub.
import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Ban, ExternalLink, FileBadge, FileCode2, GitBranch, RefreshCw } from 'lucide-react'
import { formatInstant, prettyJSON, relativeTo, shortDigest } from '@/lib/format'
import { useAction, useNow, useNs } from '@/lib/hooks'
import { asApiError } from '@/lib/http'
import { hub, hubQueries, type Credential, type ManifestFile, type VersionRecord } from '@/lib/hub'
import { CodeEditor, guessLanguage } from '@/components/code-editor'
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
              <div key={k as string} className="min-w-0 bg-bg px-6 py-2.5">
                <p className="text-xs text-muted">{k}</p>
                <div className="mt-0.5 truncate font-medium">{v}</div>
              </div>
            ))}
          </div>
          <div className="border-b px-6 py-3.5">
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

// RecordDetail shows one signed VersionRecord with its package files, proof and the revoke action.
function RecordDetail({ slug, version, onClose }: { slug: string; version: string; onClose: () => void }) {
  const q = useQuery(hubQueries.record(slug, version))
  const manifestDigest = q.data?.credentialSubject.manifest ?? ''
  const mq = useQuery({ ...hubQueries.manifest(manifestDigest), enabled: !!manifestDigest })
  const [tab, setTab] = useState<'summary' | 'files' | 'json'>('summary')
  const [revoking, setRevoking] = useState(false)
  const files = mq.data?.credentialSubject.files ?? []
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
          <Tabs
            className="px-3"
            value={tab}
            onChange={setTab}
            tabs={[
              { id: 'summary', label: 'Record' },
              { id: 'files', label: 'Files', count: mq.data ? files.length : undefined },
              { id: 'json', label: 'Credential JSON' },
            ]}
          />
          {tab === 'summary' ? (
            <RecordSummary vc={q.data} />
          ) : tab === 'files' ? (
            <PackageFiles slug={slug} version={version} manifestDigest={manifestDigest} files={files} isPending={mq.isPending} error={mq.isError ? mq.error : null} />
          ) : (
            <div className="p-3"><CodeEditor readOnly copy value={prettyJSON(q.data)} maxHeight="70vh" /></div>
          )}
        </>
      )}
      <Revoke slug={slug} version={version} open={revoking} onOpenChange={setRevoking} />
    </DetailPanel>
  )
}

// PackageFiles lists the immutable artefacts compiled into a version package and previews them from /pub.
function PackageFiles({
  slug,
  version,
  manifestDigest,
  files,
  isPending,
  error,
}: {
  slug: string
  version: string
  manifestDigest: string
  files: ManifestFile[]
  isPending: boolean
  error: unknown
}) {
  const navigate = useNavigate()
  const [preview, setPreview] = useState<ManifestFile | null>(null)
  const previewPath = preview ? `/pub/${slug}/${version}/${preview.path}` : ''
  const pq = useQuery({ ...hubQueries.pub(previewPath), enabled: !!preview })
  const manifestPath = `/pub/packages/${manifestDigest.replace(/^ni:\/\/\/sha-256;/, '')}/manifest.json`
  const openInPub = (p: string) => void navigate({ to: `/hub/pub?path=${encodeURIComponent(p)}` })
  if (isPending) return <div className="p-4"><Spinner /></div>
  if (error) return <ErrorState error={error} />
  const body = pq.data ? (typeof pq.data.data === 'string' ? pq.data.text : prettyJSON(pq.data.data)) : ''
  return (
    <div className="space-y-3 p-3">
      <div className="flex items-center justify-between gap-2 rounded-md border bg-side px-2.5 py-1.5 text-xs">
        <span className="truncate text-muted">Package manifest</span>
        <button onClick={() => openInPub(manifestPath)} className="inline-flex shrink-0 items-center gap-1 font-mono text-accent hover:underline">
          manifest.json <ExternalLink className="size-3" />
        </button>
      </div>
      <ul className="divide-y rounded-md border">
        {files.map((f) => {
          const pubUrl = `/pub/${slug}/${version}/${f.path}`
          return (
            <li key={f.path} className="flex items-center gap-2 px-2.5 py-2 hover:bg-hover">
              <button onClick={() => setPreview(f)} className="min-w-0 flex-1 text-left">
                <div className="flex items-center gap-1.5">
                  <FileCode2 className="size-3.5 shrink-0 text-subtle" />
                  <span className="truncate font-mono text-xs font-medium">{f.path}</span>
                </div>
                <div className="mt-0.5 flex flex-wrap items-center gap-1.5 pl-5 text-2xs text-subtle">
                  <Badge>{f.role}</Badge>
                  {f.slot ? <span>slot: {f.slot}</span> : null}
                  <span className="font-mono">{f.media_type}</span>
                </div>
              </button>
              <button
                title="Open in Publication"
                onClick={() => openInPub(pubUrl)}
                className="rounded p-1 text-subtle hover:bg-active hover:text-fg"
              >
                <ExternalLink className="size-3.5" />
              </button>
            </li>
          )
        })}
      </ul>
      {preview ? (
        <Dialog
          open={!!preview}
          onOpenChange={(o) => { if (!o) setPreview(null) }}
          width="max-w-3xl"
          title={preview.path}
          description={`${previewPath} · ${preview.media_type}`}
          footer={
            <>
              <Button variant="ghost" icon={<ExternalLink className="size-3.5" />} onClick={() => { const u = previewPath; setPreview(null); openInPub(u) }}>
                Open in Publication
              </Button>
              <Button variant="primary" onClick={() => setPreview(null)}>Close</Button>
            </>
          }
        >
          {pq.isPending ? (
            <div className="flex h-40 items-center justify-center"><Spinner /></div>
          ) : pq.isError ? (
            <ErrorState error={pq.error} />
          ) : (
            <CodeEditor readOnly copy value={body} language={guessLanguage(body, preview.media_type)} maxHeight="60vh" />
          )}
        </Dialog>
      ) : null}
    </div>
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
