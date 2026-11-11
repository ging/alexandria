// Vocabularies view: hosted and proxy vocabularies of a namespace, their immutable revisions and revision content.
// Stewards can register vocabularies, upload hosted content (new revision per distinct digest) and refresh proxies from upstream.
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { BookOpen, CloudDownload, Plus, Upload } from 'lucide-react'
import { cn } from '@/lib/cn'
import { formatBytes, formatInstant, relativeTo, shortDigest } from '@/lib/format'
import { useAction, useNs } from '@/lib/hooks'
import { hub, hubQueries, type Vocabulary, type VocabularyInput } from '@/lib/hub'
import { CodeEditor, guessLanguage } from '@/components/code-editor'
import { DataTable, FilterInput, type Column } from '@/components/data-table'
import { DetailPanel, Split } from '@/components/detail-panel'
import { ErrorState } from '@/components/problem'
import { Button } from '@/components/ui/button'
import { Field, Input, Select } from '@/components/ui/form'
import { Badge, Empty, KV, Mono, Page, Spinner } from '@/components/ui/misc'
import { Dialog } from '@/components/ui/overlay'

const columns: Column<Vocabulary>[] = [
  { accessorKey: 'name', header: 'Name', cell: (c) => <span className="font-mono text-xs font-medium">{c.row.original.name}</span> },
  { accessorKey: 'kind', header: 'Kind', cell: (c) => <Badge tone={c.row.original.kind === 'proxy' ? 'purple' : 'info'}>{c.row.original.kind}</Badge>, meta: { className: 'w-20' } },
  { accessorKey: 'media_type', header: 'Media type', cell: (c) => <span className="font-mono text-xs text-muted">{c.row.original.media_type}</span> },
  { id: 'revisions', header: 'Rev', accessorFn: (v) => v.revisions.length, cell: (c) => <span className="tnum text-muted">r{c.row.original.revisions.length}</span>, meta: { className: 'w-12' } },
  { id: 'updated', header: 'Last revision', accessorFn: (v) => v.revisions.at(-1)?.created_at ?? '', cell: (c) => <span className="text-muted">{relativeTo(c.row.original.revisions.at(-1)?.created_at)}</span>, meta: { className: 'w-32' } },
]

// VocabulariesPage renders the vocabulary list with its inspector.
export function VocabulariesPage() {
  const slug = useNs()
  const q = useQuery(hubQueries.vocabularies(slug))
  const [filter, setFilter] = useState('')
  const [selected, setSelected] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const current = q.data?.find((v) => v.name === selected)
  return (
    <Page
      flush
      title="Vocabularies"
      subtitle={<>Registry served at <span className="font-mono">/pub/{slug}/vocab/{'{name}'}</span> — hosted content or pinned copies of an upstream.</>}
      actions={
        <>
          <FilterInput value={filter} onChange={setFilter} placeholder="Filter vocabularies" />
          <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setCreating(true)}>Register vocabulary</Button>
        </>
      }
    >
      {q.isPending ? (
        <div className="flex h-40 items-center justify-center"><Spinner /></div>
      ) : q.isError ? (
        <ErrorState error={q.error} />
      ) : (
        <Split detail={current ? <VocabularyDetail key={current.name} slug={slug} vocab={current} onClose={() => setSelected(null)} /> : undefined}>
          <DataTable
            data={q.data}
            minWidth={560}
            columns={columns}
            filter={filter}
            rowId={(v) => v.name}
            selected={selected}
            onRowClick={(v) => setSelected(v.name)}
            empty={<Empty icon={<BookOpen />} title="No vocabularies registered" action={<Button variant="primary" onClick={() => setCreating(true)}>Register vocabulary</Button>}>Register a hosted vocabulary to upload content, or a proxy to pin an upstream document.</Empty>}
          />
        </Split>
      )}
      <CreateVocabulary slug={slug} open={creating} onOpenChange={setCreating} onCreated={setSelected} />
    </Page>
  )
}

// VocabularyDetail shows metadata, the revision list and the bytes of the selected revision.
function VocabularyDetail({ slug, vocab, onClose }: { slug: string; vocab: Vocabulary; onClose: () => void }) {
  const latest = vocab.revisions.at(-1)
  const [rev, setRev] = useState<number | null>(latest?.n ?? null)
  const [uploading, setUploading] = useState(false)
  useEffect(() => setRev(latest?.n ?? null), [latest?.n])
  const revision = vocab.revisions.find((r) => r.n === rev)
  const content = useQuery({ ...hubQueries.revisionContent(revision?.id ?? ''), enabled: !!revision })
  const keys = [['hub', 'ns', slug, 'vocabularies']]
  const refresh = useAction(() => hub.refreshVocabulary(slug, vocab.name), { success: (r) => (r.status === 201 ? `Revision r${r.data.n} pinned from upstream` : 'Upstream unchanged'), invalidate: keys })
  return (
    <DetailPanel
      title={<span className="font-mono text-sm">{vocab.name}</span>}
      onClose={onClose}
      actions={
        vocab.kind === 'proxy' ? (
          <Button size="sm" icon={<CloudDownload className="size-3.5" />} loading={refresh.isPending} onClick={() => refresh.mutate(undefined)}>Refresh</Button>
        ) : (
          <Button size="sm" icon={<Upload className="size-3.5" />} onClick={() => setUploading(true)}>Upload content</Button>
        )
      }
    >
      <div className="space-y-4 p-3">
        <KV
          items={[
            ['Kind', <Badge tone={vocab.kind === 'proxy' ? 'purple' : 'info'}>{vocab.kind}</Badge>],
            ['Media type', <Mono>{vocab.media_type}</Mono>],
            ...(vocab.upstream ? [['Upstream', <Mono copy={vocab.upstream}>{vocab.upstream}</Mono>] as [string, React.ReactNode]] : []),
            ...(vocab.title ? [['Title', vocab.title] as [string, React.ReactNode]] : []),
            ...(vocab.description ? [['Description', <span className="text-muted">{vocab.description}</span>] as [string, React.ReactNode]] : []),
            ['Stable URL', <Mono copy={`/pub/${slug}/vocab/${vocab.name}`}>/pub/{slug}/vocab/{vocab.name}</Mono>],
          ]}
        />
        <div>
          <p className="label-caps mb-1.5">Revisions</p>
          {vocab.revisions.length === 0 ? (
            <p className="text-sm text-muted">No content yet. Upload the first revision.</p>
          ) : (
            <ul className="overflow-hidden rounded-md border">
              {[...vocab.revisions].reverse().map((r) => (
                <li key={r.n}>
                  <button onClick={() => setRev(r.n)} className={cn('flex h-8 w-full items-center gap-3 border-b px-2.5 text-left last:border-b-0', r.n === rev ? 'bg-accent-soft' : 'hover:bg-hover')}>
                    <span className="tnum w-7 font-mono text-xs font-semibold">r{r.n}</span>
                    <span className="font-mono text-xs text-muted">{shortDigest(r.digest, 14)}</span>
                    <span className="tnum text-xs text-subtle">{formatBytes(r.size)}</span>
                    <span className="ml-auto truncate text-xs text-subtle" title={formatInstant(r.created_at)}>{relativeTo(r.created_at)} · {r.created_by.split('#').pop()}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
        {revision ? (
          <div>
            <div className="mb-1.5 flex items-center justify-between">
              <p className="label-caps">Content · r{revision.n}</p>
              <Mono copy={revision.digest} className="text-subtle">{shortDigest(revision.digest, 18)}</Mono>
            </div>
            {content.isPending ? <Spinner /> : content.isError ? <ErrorState error={content.error} /> : <CodeEditor readOnly copy value={content.data} language={guessLanguage(content.data, vocab.media_type)} maxHeight="46vh" />}
          </div>
        ) : null}
      </div>
      <UploadContent slug={slug} vocab={vocab} initial={content.data ?? ''} open={uploading} onOpenChange={setUploading} />
    </DetailPanel>
  )
}

// UploadContent is the dialog that PUTs new bytes of a hosted vocabulary, guarded by If-Match on the latest digest.
function UploadContent({ slug, vocab, initial, open, onOpenChange }: { slug: string; vocab: Vocabulary; initial: string; open: boolean; onOpenChange: (o: boolean) => void }) {
  const [text, setText] = useState(initial)
  useEffect(() => { if (open) setText(initial) }, [open, initial])
  const latest = vocab.revisions.at(-1)
  const etag = latest ? `"${latest.digest.replace(/^ni:\/\/\/sha-256;/, '')}"` : undefined
  const put = useAction(() => hub.putContent(slug, vocab.name, text, vocab.media_type), {
    success: (r) => (r.status === 201 ? `Revision r${r.data.n} created` : 'Content unchanged — no new revision'),
    invalidate: [['hub', 'ns', slug, 'vocabularies']],
    onSuccess: () => onOpenChange(false),
  })
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      width="max-w-3xl"
      title={`Upload content · ${vocab.name}`}
      description={<>Sent as <Mono>{vocab.media_type}</Mono>. Identical bytes do not create a revision.</>}
      footer={
        <>
          {etag ? <span className="mr-auto text-xs text-subtle">replaces r{latest!.n}</span> : null}
          <Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="primary" disabled={!text} loading={put.isPending} onClick={() => put.mutate(undefined)}>Publish revision</Button>
        </>
      }
    >
      <CodeEditor value={text} onChange={setText} language={guessLanguage(text, vocab.media_type)} minHeight="320px" maxHeight="50vh" placeholder="Paste the vocabulary document…" />
    </Dialog>
  )
}

const mediaTypes = ['text/turtle', 'application/ld+json', 'application/rdf+xml', 'application/n-triples', 'application/schema+json']

// CreateVocabulary is the dialog that registers a hosted or proxy vocabulary.
function CreateVocabulary({ slug, open, onOpenChange, onCreated }: { slug: string; open: boolean; onOpenChange: (o: boolean) => void; onCreated: (name: string) => void }) {
  const [form, setForm] = useState<VocabularyInput>({ name: '', kind: 'hosted', media_type: 'text/turtle' })
  const set = (patch: Partial<VocabularyInput>) => setForm((f) => ({ ...f, ...patch }))
  const create = useAction(() => hub.createVocabulary(slug, { ...form, upstream: form.kind === 'proxy' ? form.upstream : undefined, title: form.title || undefined }), {
    success: `Vocabulary ${form.name} registered`,
    invalidate: [['hub', 'ns', slug, 'vocabularies']],
    onSuccess: () => { onOpenChange(false); onCreated(form.name); setForm({ name: '', kind: 'hosted', media_type: 'text/turtle' }) },
  })
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Register vocabulary"
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="primary" disabled={!form.name || (form.kind === 'proxy' && !form.upstream)} loading={create.isPending} onClick={() => create.mutate(undefined)}>Register</Button>
        </>
      }
    >
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-1 rounded-md bg-active p-0.5">
          {(['hosted', 'proxy'] as const).map((k) => (
            <button key={k} onClick={() => set({ kind: k })} className={cn('h-6 rounded-sm text-sm capitalize', form.kind === k ? 'bg-bg font-medium text-fg shadow-sm' : 'text-muted')}>{k}</button>
          ))}
        </div>
        <p className="text-xs text-muted">{form.kind === 'hosted' ? 'The hub stores the content you upload; each distinct upload is a new immutable revision.' : 'The hub downloads the upstream document and pins it as revision 1; refresh creates new revisions when it changes.'}</p>
        <Field label="Name" hint="Lowercase letters, digits, dot, dash and underscore.">
          <Input mono autoFocus value={form.name} onChange={(e) => set({ name: e.target.value })} placeholder="odrl-ext" />
        </Field>
        <Field label="Media type">
          <Select value={form.media_type} onChange={(e) => set({ media_type: e.target.value })}>
            {mediaTypes.map((m) => <option key={m}>{m}</option>)}
          </Select>
        </Field>
        {form.kind === 'proxy' ? (
          <Field label="Upstream URL" hint="https only (http://localhost is accepted in lab mode).">
            <Input mono value={form.upstream ?? ''} onChange={(e) => set({ upstream: e.target.value })} placeholder="https://example.org/context.jsonld" />
          </Field>
        ) : null}
        <Field label="Title">
          <Input value={form.title ?? ''} onChange={(e) => set({ title: e.target.value })} />
        </Field>
      </div>
    </Dialog>
  )
}
