// Authoring view: the draft → candidate → release workflow of a namespace (validate, propose, approve, reject).
// The hub has no list endpoints for drafts or candidates, so the index is rebuilt from the namespace event log.
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, CircleSlash, ExternalLink, Eye, FileCode2, FilePlus2, FileText, PackageCheck, PanelLeftClose, PanelLeftOpen, Save, Send, ShieldCheck, XCircle } from 'lucide-react'
import { cn } from '@/lib/cn'
import { formatInstant, iriTail, prettyJSON, relativeTo, shortDigest } from '@/lib/format'
import { useAction, useNs } from '@/lib/hooks'
import { hasOpenDialog, isEditableTarget, useWindowKeydown } from '@/lib/hotkeys'
import { hub, hubQueries, type HubEvent, type ManifestFile, type ValidationReport } from '@/lib/hub'
import { runtime } from '@/lib/runtime'
import { CodeEditor, guessLanguage } from '@/components/code-editor'
import { DslStudio } from '@/components/dsl-studio'
import { ClassBadge, StateBadge } from '@/components/lifecycle'
import { defaultOdrlProfile, defaultSpaceProfile, serializeDslBundle } from '@/lib/dsl-bundle'
import { ErrorState } from '@/components/problem'
import { Button, IconButton } from '@/components/ui/button'
import { Field, Input, Textarea } from '@/components/ui/form'
import { Badge, Empty, KV, Mono, Page, Spinner, Tabs } from '@/components/ui/misc'
import { Dialog } from '@/components/ui/overlay'

// dslTemplate builds a minimal compilable alexandria.profiles/v1 YAML bundle with ODRL and DSP 2025-1 bindings.
const dslTemplate = (base: string, slug: string) => {
  const odrl = defaultOdrlProfile(base, slug)
  return serializeDslBundle({ odrls: [odrl], space: defaultSpaceProfile(base, slug, odrl.profile.id) })
}

// pocTemplate builds a minimal Part 5.1 JSON draft when the server runs in poc compiler mode.
const pocTemplate = (base: string, slug: string) =>
  prettyJSON({
    version: '1.0.0',
    profiles: [`${base}/pub/${slug}/profiles/odrl/1.0.0`],
    slots: [{ slot: 'policy', obligation: 'mandatory', applies_to: ['policy_intake'] }],
    files: [
      {
        path: 'schemas/policy.schema.json',
        media_type: 'application/schema+json',
        role: 'json-schema',
        slot: 'policy',
        applies_to: ['policy_intake'],
        content: { type: 'object', properties: { action: { enum: ['odrl:use', 'ex:access'] } }, required: ['action'] },
      },
    ],
  })

// Entry is one draft or candidate reconstructed from events.
interface Entry {
  id: string
  at: string
  by: string
  state?: string
}

// index folds the event log into the latest known drafts and candidates.
function index(events: HubEvent[]): { drafts: Entry[]; candidates: Entry[] } {
  const drafts = new Map<string, Entry>()
  const candidates = new Map<string, Entry>()
  for (const e of events) {
    const id = iriTail(e.subject)
    const by = e.principal.split('#').pop() ?? e.principal
    if (e.type === 'draft.created' || e.type === 'draft.updated') drafts.set(id, { id, at: e.ts, by })
    if (e.type === 'candidate.proposed') candidates.set(id, { id, at: e.ts, by, state: 'open' })
    if (e.type === 'candidate.approved' || e.type === 'candidate.rejected') {
      const c = candidates.get(id)
      if (c) candidates.set(id, { ...c, state: e.type.split('.')[1] })
    }
  }
  const sort = (m: Map<string, Entry>) => [...m.values()].sort((a, b) => b.at.localeCompare(a.at))
  return { drafts: sort(drafts), candidates: sort(candidates) }
}

type Selection = { kind: 'draft' | 'candidate'; id: string } | { kind: 'new' } | null

// AuthoringPage renders the draft and candidate index with the editor, inline new-draft studio or review pane.
export function AuthoringPage() {
  const slug = useNs()
  const events = useQuery(hubQueries.events(slug))
  const nsq = useQuery(hubQueries.namespace(slug))
  const [tab, setTab] = useState<'draft' | 'candidate'>('draft')
  const [sel, setSel] = useState<Selection>(null)
  const [prevSel, setPrevSel] = useState<Selection>(null)
  const [listOpen, setListOpen] = useState(true)
  const { drafts, candidates } = useMemo(() => index(events.data ?? []), [events.data])
  useEffect(() => {
    if (!sel && drafts.length > 0) setSel({ kind: 'draft', id: drafts[0].id })
  }, [sel, drafts])
  const list = tab === 'draft' ? drafts : candidates
  const open = (s: { kind: 'draft' | 'candidate'; id: string }) => { setTab(s.kind); setSel(s) }
  const startNewDraft = () => {
    if (sel?.kind !== 'new') setPrevSel(sel)
    setSel({ kind: 'new' })
  }
  const base = nsq.data ? new URL(nsq.data.id).origin : ''

  useWindowKeydown((e) => {
    if (hasOpenDialog() || isEditableTarget(e.target) || e.metaKey || e.ctrlKey || e.altKey) return
    if (e.key === 'n') {
      e.preventDefault()
      startNewDraft()
    } else if (e.key === '[') {
      e.preventDefault()
      setListOpen((o) => !o)
    }
  })

  return (
    <Page
      flush
      title="Authoring"
      subtitle="Draft a release, validate it (N1–N5), freeze it as a candidate and approve it into a signed version."
      actions={
        <Button
          variant="primary"
          icon={<FilePlus2 className="size-3.5" />}
          disabled={sel?.kind === 'new'}
          title="New draft (n)"
          onClick={startNewDraft}
        >
          New draft
        </Button>
      }
    >
      {events.isError ? (
        <ErrorState error={events.error} />
      ) : (
        <div className="flex h-full min-h-0">
          {listOpen ? (
            <div className="flex w-48 shrink-0 flex-col border-r">
              <Tabs
                className="px-3"
                value={tab}
                onChange={setTab}
                tabs={[
                  { id: 'draft', label: 'Drafts', count: drafts.length },
                  { id: 'candidate', label: 'Candidates', count: candidates.length },
                ]}
              />
              <ul className="min-h-0 flex-1 overflow-auto p-1.5">
                {events.isPending ? <li className="p-3"><Spinner /></li> : null}
                {!events.isPending && list.length === 0 ? <li className="px-2 py-4 text-center text-xs text-subtle">No {tab}s yet.</li> : null}
                {list.map((e) => {
                  const active = sel?.kind === tab && sel.id === e.id
                  return (
                    <li key={e.id}>
                      <button
                        onClick={() => setSel({ kind: tab, id: e.id })}
                        className={cn('flex w-full flex-col gap-0.5 rounded-md px-2 py-1.5 text-left', active ? 'bg-active' : 'hover:bg-hover')}
                      >
                        <span className="flex items-center gap-1.5">
                          {tab === 'draft' ? <FileText className="size-3.5 text-subtle" /> : <PackageCheck className="size-3.5 text-subtle" />}
                          <span className="truncate font-mono text-xs font-medium">{shortDigest(e.id, 12)}</span>
                          {e.state ? <StateBadge state={e.state} dot={false} /> : null}
                        </span>
                        <span className="pl-5 text-xs text-subtle">{relativeTo(e.at)} · {e.by}</span>
                      </button>
                    </li>
                  )
                })}
              </ul>
            </div>
          ) : null}
          <div className="min-w-0 flex-1">
            {sel?.kind === 'new' ? (
              <NewDraftEditor
                key={`new:${slug}`}
                slug={slug}
                base={base || 'https://hub.example.com'}
                initial={(runtime.compiler === 'poc' ? pocTemplate : dslTemplate)(base || 'https://hub.example.com', slug)}
                listOpen={listOpen}
                onToggleList={() => setListOpen((o) => !o)}
                onCancel={() => setSel(prevSel ?? (drafts[0] ? { kind: 'draft', id: drafts[0].id } : null))}
                onCreated={(id) => open({ kind: 'draft', id })}
              />
            ) : sel?.kind === 'draft' ? (
              <DraftEditor
                key={sel.id}
                slug={slug}
                base={base}
                id={sel.id}
                listOpen={listOpen}
                onToggleList={() => setListOpen((o) => !o)}
                onProposed={(id) => open({ kind: 'candidate', id })}
              />
            ) : sel?.kind === 'candidate' ? (
              <CandidateReview key={sel.id} slug={slug} id={sel.id} listOpen={listOpen} onToggleList={() => setListOpen((o) => !o)} />
            ) : (
              <Empty icon={<FileText />} title="Select a draft or candidate" action={<Button variant="primary" onClick={startNewDraft}>New draft</Button>}>
                Drafts stay editable under optimistic concurrency; proposing one compiles it into an immutable candidate.
              </Empty>
            )}
          </div>
        </div>
      )}
    </Page>
  )
}

// NewDraftEditor renders a full-height inline studio for creating a new draft without modal cramping.
function NewDraftEditor({
  slug,
  base,
  initial,
  listOpen,
  onToggleList,
  onCancel,
  onCreated,
}: {
  slug: string
  base: string
  initial: string
  listOpen: boolean
  onToggleList: () => void
  onCancel: () => void
  onCreated: (id: string) => void
}) {
  const [text, setText] = useState(initial)
  const isPoc = runtime.compiler === 'poc'
  const create = useAction(() => hub.createDraft(slug, text), {
    success: 'Draft created',
    invalidate: [['hub', 'ns', slug, 'events']],
    onSuccess: (r) => onCreated(iriTail(r.data.id)),
  })

  useWindowKeydown((e) => {
    if (hasOpenDialog()) return
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && !create.isPending) {
      e.preventDefault()
      create.mutate(undefined)
    } else if (e.key === 'Escape' && !isEditableTarget(e.target)) {
      e.preventDefault()
      onCancel()
    }
  })

  return (
    <div className="flex h-full flex-col">
      <div className="flex h-10 shrink-0 items-center gap-2 border-b px-3">
        <IconButton size="sm" label={listOpen ? 'Hide draft list ([)' : 'Show draft list ([)'} onClick={onToggleList}>
          {listOpen ? <PanelLeftClose className="size-3.5" /> : <PanelLeftOpen className="size-3.5" />}
        </IconButton>
        <FilePlus2 className="size-3.5 text-accent" />
        <span className="text-sm font-medium">New draft</span>
        <Badge tone="accent">{isPoc ? 'PoC JSON' : 'alexandria.profiles/v1'}</Badge>
        <div className="ml-auto flex items-center gap-1.5">
          <Button size="sm" variant="ghost" onClick={onCancel}>Cancel</Button>
          <Button size="sm" variant="primary" title="Create draft (⌘↵)" loading={create.isPending} onClick={() => create.mutate(undefined)}>
            Create draft
          </Button>
        </div>
      </div>
      {isPoc ? (
        <div className="min-h-0 flex-1 p-3">
          <CodeEditor value={text} onChange={setText} language={guessLanguage(text)} className="h-full" />
        </div>
      ) : (
        <DslStudio value={text} onChange={setText} base={base} slug={slug} className="min-h-0 flex-1 rounded-none border-0" />
      )}
    </div>
  )
}

// DraftEditor edits one draft with save (If-Match), validate and propose actions.
function DraftEditor({
  slug,
  base,
  id,
  listOpen,
  onToggleList,
  onProposed,
}: {
  slug: string
  base: string
  id: string
  listOpen: boolean
  onToggleList: () => void
  onProposed: (candidateId: string) => void
}) {
  const qc = useQueryClient()
  const q = useQuery(hubQueries.draft(slug, id))
  const [text, setText] = useState<string | null>(null)
  const [report, setReport] = useState<ValidationReport | null>(null)
  const rawDoc = q.data?.draft.document
  const loaded = typeof rawDoc === 'string' ? rawDoc : rawDoc !== undefined ? prettyJSON(rawDoc) : ''
  useEffect(() => { if (q.data) setText(loaded) }, [q.data, loaded])
  const dirty = text !== null && text !== loaded
  const isJsonDraft = (text ?? '').trimStart().startsWith('{') || runtime.compiler === 'poc'
  const key = ['hub', 'ns', slug, 'drafts', id]
  const save = useAction(() => hub.replaceDraft(slug, id, text ?? '', q.data?.etag ?? ''), {
    success: (r) => `Saved as revision ${r.data.rev}`,
    onSuccess: (r) => { qc.setQueryData(key, { draft: r.data, etag: r.etag ?? '' }); void qc.invalidateQueries({ queryKey: ['hub', 'ns', slug, 'events'] }); setReport(null) },
  })
  const validate = useAction(() => hub.validateDraft(slug, id), { onSuccess: (r) => setReport(r.data) })
  const propose = useAction(() => hub.proposeDraft(slug, id), {
    success: (r) => `Candidate ${r.data.version} proposed`,
    invalidate: [['hub', 'ns', slug, 'events']],
    onSuccess: (r) => onProposed(iriTail(r.data.id)),
  })

  useWindowKeydown((e) => {
    if (hasOpenDialog()) return
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
      e.preventDefault()
      if (dirty && !save.isPending) save.mutate(undefined)
    } else if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault()
      if (!dirty && !validate.isPending) validate.mutate(undefined)
    }
  })

  if (q.isPending) return <div className="flex h-40 items-center justify-center"><Spinner /></div>
  if (q.isError) return <ErrorState error={q.error} />
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-10 shrink-0 items-center gap-2 border-b px-3">
        <IconButton size="sm" label={listOpen ? 'Hide draft list ([)' : 'Show draft list ([)'} onClick={onToggleList}>
          {listOpen ? <PanelLeftClose className="size-3.5" /> : <PanelLeftOpen className="size-3.5" />}
        </IconButton>
        <FileText className="size-3.5 text-subtle" />
        <Mono copy={id} className="font-medium">{shortDigest(id, 16)}</Mono>
        <Badge>rev {q.data.draft.rev}</Badge>
        {dirty ? <Badge tone="warning" dot>unsaved</Badge> : null}
        <div className="ml-auto flex items-center gap-1.5">
          <Button size="sm" icon={<ShieldCheck className="size-3.5" />} title="Validate (⌘↵)" loading={validate.isPending} disabled={dirty} onClick={() => validate.mutate(undefined)}>Validate</Button>
          <Button size="sm" icon={<Save className="size-3.5" />} title="Save (⌘S)" loading={save.isPending} disabled={!dirty} onClick={() => save.mutate(undefined)}>Save</Button>
          <Button size="sm" variant="primary" icon={<Send className="size-3.5" />} loading={propose.isPending} disabled={dirty} onClick={() => propose.mutate(undefined)}>Propose</Button>
        </div>
      </div>
      {isJsonDraft ? (
        <div className="min-h-0 flex-1 p-3">
          <CodeEditor value={text ?? ''} onChange={setText} language={guessLanguage(text ?? '')} className="h-full" />
        </div>
      ) : (
        <DslStudio
          value={text ?? ''}
          onChange={setText}
          base={base || 'https://hub.example.com'}
          slug={slug}
          className="min-h-0 flex-1 rounded-none border-0"
        />
      )}
      {report ? (
        <div className="max-h-[35%] shrink-0 overflow-auto border-t bg-side px-3 py-2">
          <div className="flex items-center gap-2">
            {report.valid ? <CheckCircle2 className="size-4 text-success" /> : <XCircle className="size-4 text-danger" />}
            <span className="font-medium">{report.valid ? 'Draft is valid' : `${report.findings?.length ?? 0} finding(s)`}</span>
            {report.class ? <ClassBadge value={report.class} /> : null}
            <button className="ml-auto text-xs text-subtle hover:text-fg" onClick={() => setReport(null)}>Dismiss</button>
          </div>
          {report.findings?.length ? (
            <ul className="mt-2 space-y-1">
              {report.findings.map((f, i) => (
                <li key={i} className="flex flex-wrap items-baseline gap-x-2 text-xs">
                  <span className="font-mono font-medium text-danger">{String(f.code ?? f.Code ?? 'finding')}</span>
                  <span>{String(f.detail ?? f.message ?? f.Message ?? f.Detail ?? JSON.stringify(f))}</span>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

// DiffEntry represents one parsed line from Candidate.diff.
interface DiffEntry {
  kind: 'added' | 'changed' | 'removed' | 'other'
  path: string
  raw: string
}

// parseDiffLine extracts the change verb and artefact path from a candidate diff string.
function parseDiffLine(line: string): DiffEntry {
  const m = /^(added|changed|removed)\s+(.+)$/.exec(line.trim())
  if (m) return { kind: m[1] as DiffEntry['kind'], path: m[2].trim(), raw: line }
  if (line.startsWith('+')) return { kind: 'added', path: line.slice(1).trim(), raw: line }
  if (line.startsWith('-')) return { kind: 'removed', path: line.slice(1).trim(), raw: line }
  return { kind: 'other', path: '', raw: line }
}

// diffTone maps a DiffEntry kind to a semantic badge tone.
function diffTone(kind: DiffEntry['kind']): 'success' | 'warning' | 'danger' | 'neutral' {
  if (kind === 'added') return 'success'
  if (kind === 'changed') return 'warning'
  if (kind === 'removed') return 'danger'
  return 'neutral'
}

// guessMediaType infers a MIME type from a file extension when manifest metadata is unavailable.
function guessMediaType(filePath: string): string {
  if (filePath.endsWith('.jsonld')) return 'application/ld+json'
  if (filePath.endsWith('.json')) return 'application/json'
  if (filePath.endsWith('.ttl')) return 'text/turtle'
  if (filePath.endsWith('.html')) return 'text/html'
  return 'text/plain'
}

// CandidateReview shows a frozen candidate with its planned dates, interactive diff, file browser and release actions.
function CandidateReview({ slug, id, listOpen, onToggleList }: { slug: string; id: string; listOpen: boolean; onToggleList: () => void }) {
  const navigate = useNavigate()
  const q = useQuery(hubQueries.candidate(slug, id))
  const listingQ = useQuery(hubQueries.listing(slug))
  const [tab, setTab] = useState<'summary' | 'files'>('summary')
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [compareSide, setCompareSide] = useState<'candidate' | 'head'>('candidate')
  const [htmlPreview, setHtmlPreview] = useState(true)
  const [approving, setApproving] = useState(false)
  const [rejecting, setRejecting] = useState(false)
  const [grace, setGrace] = useState('')
  const [comment, setComment] = useState('')

  const c = q.data
  const recQ = useQuery({ ...hubQueries.record(slug, c?.version ?? ''), enabled: !!c && !c.files?.length && c.state === 'approved' })
  const manifestDigest = recQ.data?.credentialSubject.manifest ?? ''
  const manQ = useQuery({ ...hubQueries.manifest(manifestDigest), enabled: !!manifestDigest })

  const diffEntries = useMemo(() => (c?.diff ?? []).map(parseDiffLine), [c?.diff])
  const diffByPath = useMemo(() => {
    const map = new Map<string, DiffEntry['kind']>()
    for (const d of diffEntries) {
      if (d.path) map.set(d.path, d.kind)
    }
    return map
  }, [diffEntries])

  const fileList = useMemo<ManifestFile[]>(() => {
    const baseFiles: ManifestFile[] = c?.files?.length
      ? c.files
      : manQ.data?.credentialSubject.files?.length
        ? manQ.data.credentialSubject.files
        : diffEntries
            .filter((d) => d.path && d.kind !== 'removed')
            .map((d) => ({ path: d.path, media_type: guessMediaType(d.path), digest: '', role: 'artefact' }))
    const known = new Set(baseFiles.map((f) => f.path))
    const removed: ManifestFile[] = diffEntries
      .filter((d) => d.kind === 'removed' && d.path && !known.has(d.path))
      .map((d) => ({ path: d.path, media_type: guessMediaType(d.path), digest: '', role: 'removed' }))
    return [...baseFiles, ...removed]
  }, [c?.files, manQ.data?.credentialSubject.files, diffEntries])

  useEffect(() => {
    if (!selectedFile && fileList.length > 0) setSelectedFile(fileList[0].path)
  }, [selectedFile, fileList])

  const headVersion = useMemo(() => {
    const versions = listingQ.data?.credentialSubject.versions ?? []
    const prior = versions.filter((v) => !v.revoked_from && v.version !== c?.version)
    return prior.at(-1)?.version ?? versions.filter((v) => !v.revoked_from).at(-1)?.version ?? null
  }, [listingQ.data, c?.version])

  const activeFile = fileList.find((f) => f.path === selectedFile) ?? null
  const activeDiffKind = activeFile ? diffByPath.get(activeFile.path) : undefined
  const useHeadBytes = activeDiffKind === 'removed' || (activeDiffKind === 'changed' && compareSide === 'head' && !!headVersion)
  const pubPathForSelected = activeFile
    ? useHeadBytes && headVersion
      ? `/pub/${slug}/${headVersion}/${activeFile.path}`
      : c?.state === 'approved'
        ? `/pub/${slug}/${c.version}/${activeFile.path}`
        : null
    : null

  const pubFileQ = useQuery({
    ...hubQueries.pub(pubPathForSelected ?? ''),
    enabled: tab === 'files' && !!activeFile && !!pubPathForSelected,
  })
  const candFileQ = useQuery({
    ...hubQueries.candidateFile(slug, id, activeFile?.path ?? ''),
    enabled: tab === 'files' && !!activeFile && !pubPathForSelected,
  })

  const invalidate = [['hub', 'ns', slug]]
  const approve = useAction(() => hub.approveCandidate(slug, id, grace || undefined), {
    success: (r) => `Version ${r.data.credentialSubject.version} published and signed`,
    invalidate,
    onSuccess: () => setApproving(false),
  })
  const reject = useAction(() => hub.rejectCandidate(slug, id, comment), { success: 'Candidate rejected', invalidate, onSuccess: () => setRejecting(false) })

  if (q.isPending) return <div className="flex h-40 items-center justify-center"><Spinner /></div>
  if (q.isError || !c) return <ErrorState error={q.error} />
  const p = c.planned
  const activeRes = pubPathForSelected ? pubFileQ.data : candFileQ.data
  const activeLoading = pubPathForSelected ? pubFileQ.isPending : candFileQ.isPending
  const activeError = pubPathForSelected ? (pubFileQ.isError ? pubFileQ.error : null) : candFileQ.isError ? candFileQ.error : null
  const activeMedia = activeRes?.headers.get('Content-Type')?.split(';')[0] || activeFile?.media_type || ''
  const activeBody = activeRes ? (typeof activeRes.data === 'string' ? activeRes.text : prettyJSON(activeRes.data)) : ''
  const isHtml = activeMedia.includes('text/html')

  const openFileTab = (filePath: string) => {
    setSelectedFile(filePath)
    setCompareSide('candidate')
    setTab('files')
  }

  const openFileNewWindow = (filePath: string, diffKind?: DiffEntry['kind']) => {
    if (diffKind === 'removed' && headVersion) {
      window.open(`${runtime.hubBase}/pub/${slug}/${headVersion}/${filePath}`, '_blank', 'noopener,noreferrer')
      return
    }
    if (c.state === 'approved') {
      window.open(`${runtime.hubBase}/pub/${slug}/${c.version}/${filePath}`, '_blank', 'noopener,noreferrer')
      return
    }
    if (activeFile?.path === filePath && activeRes) {
      const blob = new Blob([activeRes.text], { type: activeMedia || 'text/plain;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      window.open(url, '_blank', 'noopener,noreferrer')
      setTimeout(() => URL.revokeObjectURL(url), 60_000)
      return
    }
    openFileTab(filePath)
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex h-10 shrink-0 items-center gap-2 border-b px-3">
        <IconButton size="sm" label={listOpen ? 'Hide candidate list' : 'Show candidate list'} onClick={onToggleList}>
          {listOpen ? <PanelLeftClose className="size-3.5" /> : <PanelLeftOpen className="size-3.5" />}
        </IconButton>
        <PackageCheck className="size-3.5 text-subtle" />
        <span className="font-mono text-sm font-semibold">{c.version}</span>
        <ClassBadge value={c.class} />
        <StateBadge state={c.state} />
        {c.state === 'open' ? (
          <div className="ml-auto flex items-center gap-1.5">
            <Button size="sm" variant="danger" icon={<CircleSlash className="size-3.5" />} onClick={() => setRejecting(true)}>Reject</Button>
            <Button size="sm" variant="primary" icon={<CheckCircle2 className="size-3.5" />} onClick={() => setApproving(true)}>Approve & publish</Button>
          </div>
        ) : null}
      </div>

      <Tabs
        className="px-4"
        value={tab}
        onChange={setTab}
        tabs={[
          { id: 'summary', label: 'Summary' },
          { id: 'files', label: 'Files', count: fileList.length },
        ]}
      />

      {tab === 'summary' ? (
        <div className="min-h-0 flex-1 space-y-4 overflow-auto p-4">
          <KV
            items={[
              ['Candidate', <Mono copy={id}>{id}</Mono>],
              ['Proposed', <span className="tnum">{formatInstant(c.proposed_at)}</span>],
              ...(p.manifest ? [['Manifest', <Mono copy={p.manifest}>{shortDigest(p.manifest, 24)}</Mono>] as [string, React.ReactNode]] : []),
            ]}
          />
          <div>
            <p className="label-caps mb-1.5">Planned lifecycle (if approved at proposal time)</p>
            <div className="grid grid-cols-3 gap-2">
              {([['Published', p.published_at], ['New content from', p.new_content_from], ['Enforced from', p.enforced_from]] as const).map(([k, v]) => (
                <div key={k} className="rounded-md border p-2.5">
                  <p className="text-xs text-muted">{k}</p>
                  <p className="tnum mt-0.5 font-medium">{formatInstant(v)}</p>
                </div>
              ))}
            </div>
          </div>
          <div>
            <div className="mb-1.5 flex items-center justify-between gap-2">
              <p className="label-caps">Diff against the current head</p>
              {fileList.length > 0 ? (
                <button type="button" onClick={() => setTab('files')} className="text-xs text-accent hover:underline">
                  Browse all {fileList.length} files →
                </button>
              ) : null}
            </div>
            {diffEntries.length ? (
              <ul className="divide-y rounded-md border bg-raised">
                {diffEntries.map((d, i) => (
                  <li key={i} className="flex items-center gap-2 px-3 py-1.5 hover:bg-hover/60">
                    {d.path ? (
                      <>
                        <button
                          type="button"
                          onClick={() => openFileTab(d.path)}
                          className="flex min-w-0 flex-1 items-center gap-2 text-left"
                        >
                          <Badge tone={diffTone(d.kind)} className="w-16 justify-center">{d.kind}</Badge>
                          <span className="min-w-0 flex-1 truncate font-mono text-xs text-fg hover:underline">{d.path}</span>
                          <span className="inline-flex shrink-0 items-center gap-1 text-xs text-muted">
                            <Eye className="size-3" /> View
                          </span>
                        </button>
                        {(c.state === 'approved' || (d.kind === 'removed' && headVersion)) ? (
                          <IconButton
                            size="sm"
                            label="Open raw file in new browser tab"
                            onClick={() => openFileNewWindow(d.path, d.kind)}
                          >
                            <ExternalLink className="size-3.5" />
                          </IconButton>
                        ) : null}
                      </>
                    ) : (
                      <span className="font-mono text-xs text-muted">{d.raw}</span>
                    )}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted">No differences reported against the current head.</p>
            )}
          </div>
          <div>
            <p className="label-caps mb-1.5">Lifecycle parameters</p>
            <CodeEditor readOnly copy value={prettyJSON(c.lifecycle)} maxHeight="260px" />
          </div>
        </div>
      ) : (
        <div className="flex min-h-0 flex-1">
          <div className="flex w-72 shrink-0 flex-col border-r bg-side/20">
            <ul className="min-h-0 flex-1 divide-y overflow-y-auto">
              {fileList.length === 0 ? (
                <li className="p-4 text-xs text-subtle">No compiled files available for this candidate.</li>
              ) : (
                fileList.map((f) => {
                  const dKind = diffByPath.get(f.path)
                  const active = f.path === activeFile?.path
                  return (
                    <li key={f.path}>
                      <button
                        type="button"
                        onClick={() => {
                          setSelectedFile(f.path)
                          setCompareSide('candidate')
                        }}
                        className={cn(
                          'flex w-full flex-col gap-1 px-3 py-2 text-left transition-colors',
                          active ? 'bg-accent-soft/60' : 'hover:bg-hover/60',
                        )}
                      >
                        <div className="flex items-center gap-1.5">
                          <FileCode2 className={cn('size-3.5 shrink-0', active ? 'text-accent' : 'text-subtle')} />
                          <span className={cn('min-w-0 flex-1 truncate font-mono text-xs', active ? 'font-semibold text-accent' : 'font-medium text-fg')} title={f.path}>
                            {f.path}
                          </span>
                          {dKind ? <Badge tone={diffTone(dKind)}>{dKind}</Badge> : null}
                        </div>
                        <div className="flex flex-wrap items-center gap-1.5 pl-5 text-2xs text-subtle">
                          {f.role ? <Badge>{f.role}</Badge> : null}
                          {f.slot ? <span>slot: {f.slot}</span> : null}
                          <span className="truncate font-mono">{f.media_type}</span>
                        </div>
                      </button>
                    </li>
                  )
                })
              )}
            </ul>
          </div>

          <div className="flex min-w-0 flex-1 flex-col">
            {!activeFile ? (
              <Empty icon={<FileCode2 />} title="Select a file">Choose an artefact on the left to inspect its compiled content.</Empty>
            ) : (
              <>
                <div className="flex h-9 shrink-0 items-center gap-2 border-b px-4">
                  <span className="min-w-0 flex-1 truncate font-mono text-xs font-medium text-fg">{activeFile.path}</span>
                  <span className="shrink-0 font-mono text-2xs text-subtle">{activeMedia}</span>
                  {activeDiffKind === 'changed' && headVersion ? (
                    <div className="inline-flex h-6 shrink-0 items-center rounded-md border border-line-strong bg-bg p-0.5">
                      {(['candidate', 'head'] as const).map((side) => (
                        <button
                          key={side}
                          type="button"
                          onClick={() => setCompareSide(side)}
                          className={cn(
                            'h-5 rounded-sm px-2 text-2xs font-medium transition-colors',
                            compareSide === side ? 'bg-accent-soft text-accent' : 'text-muted hover:text-fg',
                          )}
                        >
                          {side === 'candidate' ? `Candidate (${c.version})` : `Head (${headVersion})`}
                        </button>
                      ))}
                    </div>
                  ) : null}
                  {isHtml ? (
                    <div className="inline-flex h-6 shrink-0 items-center rounded-md border border-line-strong bg-bg p-0.5">
                      {([['preview', 'Preview'], ['source', 'HTML']] as const).map(([idMode, label]) => (
                        <button
                          key={idMode}
                          type="button"
                          onClick={() => setHtmlPreview(idMode === 'preview')}
                          className={cn(
                            'h-5 rounded-sm px-2 text-2xs font-medium transition-colors',
                            (idMode === 'preview') === htmlPreview ? 'bg-accent-soft text-accent' : 'text-muted hover:text-fg',
                          )}
                        >
                          {label}
                        </button>
                      ))}
                    </div>
                  ) : null}
                  {pubPathForSelected ? (
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-6 px-2 text-2xs"
                      onClick={() => void navigate({ to: `/hub/pub?path=${encodeURIComponent(pubPathForSelected)}` })}
                    >
                      Publication
                    </Button>
                  ) : null}
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-6 px-2 text-2xs"
                    icon={<ExternalLink className="size-3" />}
                    disabled={activeLoading || !!activeError}
                    onClick={() => openFileNewWindow(activeFile.path, activeDiffKind)}
                  >
                    New tab
                  </Button>
                </div>

                <div className="min-h-0 flex-1 p-3">
                  {activeLoading ? (
                    <div className="flex h-40 items-center justify-center"><Spinner /></div>
                  ) : activeError ? (
                    <ErrorState error={activeError} />
                  ) : isHtml && htmlPreview ? (
                    <iframe
                      title={activeFile.path}
                      sandbox=""
                      srcDoc={activeBody}
                      className="h-full w-full rounded-md border bg-white"
                    />
                  ) : (
                    <CodeEditor readOnly copy value={activeBody} language={guessLanguage(activeBody, activeMedia)} className="h-full" />
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      )}

      <Dialog
        open={approving}
        onOpenChange={setApproving}
        title={`Approve ${c.version}`}
        description="Publishes the package, signs the VersionRecord (eddsa-jcs-2022) and re-issues the listing. This cannot be undone."
        footer={
          <>
            <Button variant="ghost" onClick={() => setApproving(false)}>Cancel</Button>
            <Button variant="primary" loading={approve.isPending} onClick={() => approve.mutate(undefined)}>Approve & publish</Button>
          </>
        }
      >
        <Field label="Grace window" hint={c.class === 'breaking' ? 'ISO 8601 duration (e.g. P30D). Empty uses the namespace default.' : 'Only breaking releases accept an explicit grace window.'}>
          <Input mono value={grace} onChange={(e) => setGrace(e.target.value)} placeholder="P30D" disabled={c.class !== 'breaking'} />
        </Field>
      </Dialog>
      <Dialog
        open={rejecting}
        onOpenChange={setRejecting}
        title={`Reject ${c.version}`}
        footer={
          <>
            <Button variant="ghost" onClick={() => setRejecting(false)}>Cancel</Button>
            <Button variant="danger" loading={reject.isPending} onClick={() => reject.mutate(undefined)}>Reject candidate</Button>
          </>
        }
      >
        <Field label="Comment">
          <Textarea rows={3} autoFocus value={comment} onChange={(e) => setComment(e.target.value)} placeholder="Why is this candidate rejected?" />
        </Field>
      </Dialog>
    </div>
  )
}
