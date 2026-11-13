// Authoring view: the draft → candidate → release workflow of a namespace (validate, propose, approve, reject).
// The hub has no list endpoints for drafts or candidates, so the index is rebuilt from the namespace event log.
import { useEffect, useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, CircleSlash, FilePlus2, FileText, PackageCheck, Save, Send, ShieldCheck, XCircle } from 'lucide-react'
import { cn } from '@/lib/cn'
import { formatInstant, iriTail, prettyJSON, relativeTo, shortDigest } from '@/lib/format'
import { useAction, useNs } from '@/lib/hooks'
import { hub, hubQueries, type HubEvent, type ValidationReport } from '@/lib/hub'
import { CodeEditor, guessLanguage } from '@/components/code-editor'
import { ClassBadge, StateBadge } from '@/components/lifecycle'
import { ErrorState } from '@/components/problem'
import { Button } from '@/components/ui/button'
import { Field, Input, Textarea } from '@/components/ui/form'
import { Badge, Empty, KV, Mono, Page, Spinner, Tabs } from '@/components/ui/misc'
import { Dialog } from '@/components/ui/overlay'

const template = (base: string, slug: string) =>
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

type Selection = { kind: 'draft' | 'candidate'; id: string } | null

// AuthoringPage renders the draft and candidate index with the editor or the review pane.
export function AuthoringPage() {
  const slug = useNs()
  const events = useQuery(hubQueries.events(slug))
  const nsq = useQuery(hubQueries.namespace(slug))
  const [tab, setTab] = useState<'draft' | 'candidate'>('draft')
  const [sel, setSel] = useState<Selection>(null)
  const [creating, setCreating] = useState(false)
  const { drafts, candidates } = useMemo(() => index(events.data ?? []), [events.data])
  const list = tab === 'draft' ? drafts : candidates
  const open = (s: NonNullable<Selection>) => { setTab(s.kind); setSel(s) }
  const base = nsq.data ? new URL(nsq.data.id).origin : ''

  return (
    <Page
      flush
      title="Authoring"
      subtitle="Draft a release, validate it (N1–N5), freeze it as a candidate and approve it into a signed version."
      actions={<Button variant="primary" icon={<FilePlus2 className="size-3.5" />} onClick={() => setCreating(true)}>New draft</Button>}
    >
      {events.isError ? (
        <ErrorState error={events.error} />
      ) : (
        <div className="flex h-full min-h-0">
          <div className="flex w-64 shrink-0 flex-col border-r">
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
              {list.map((e) => (
                <li key={e.id}>
                  <button
                    onClick={() => setSel({ kind: tab, id: e.id })}
                    className={cn('flex w-full flex-col gap-0.5 rounded-md px-2 py-1.5 text-left', sel?.id === e.id ? 'bg-active' : 'hover:bg-hover')}
                  >
                    <span className="flex items-center gap-1.5">
                      {tab === 'draft' ? <FileText className="size-3.5 text-subtle" /> : <PackageCheck className="size-3.5 text-subtle" />}
                      <span className="truncate font-mono text-xs font-medium">{shortDigest(e.id, 12)}</span>
                      {e.state ? <StateBadge state={e.state} dot={false} /> : null}
                    </span>
                    <span className="pl-5 text-xs text-subtle">{relativeTo(e.at)} · {e.by}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div className="min-w-0 flex-1">
            {sel?.kind === 'draft' ? (
              <DraftEditor key={sel.id} slug={slug} id={sel.id} onProposed={(id) => open({ kind: 'candidate', id })} />
            ) : sel?.kind === 'candidate' ? (
              <CandidateReview key={sel.id} slug={slug} id={sel.id} />
            ) : (
              <Empty icon={<FileText />} title="Select a draft or candidate" action={<Button variant="primary" onClick={() => setCreating(true)}>New draft</Button>}>
                Drafts stay editable under optimistic concurrency; proposing one compiles it into an immutable candidate.
              </Empty>
            )}
          </div>
        </div>
      )}
      <NewDraft slug={slug} initial={template(base || 'https://hub.example.com', slug)} open={creating} onOpenChange={setCreating} onCreated={(id) => open({ kind: 'draft', id })} />
    </Page>
  )
}

// NewDraft is the dialog that creates a draft from an editable template.
function NewDraft({ slug, initial, open, onOpenChange, onCreated }: { slug: string; initial: string; open: boolean; onOpenChange: (o: boolean) => void; onCreated: (id: string) => void }) {
  const [text, setText] = useState(initial)
  useEffect(() => { if (open) setText(initial) }, [open, initial])
  const create = useAction(() => hub.createDraft(slug, text), {
    success: 'Draft created',
    invalidate: [['hub', 'ns', slug, 'events']],
    onSuccess: (r) => { onOpenChange(false); onCreated(iriTail(r.data.id)) },
  })
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      width="max-w-3xl"
      title="New draft"
      description="JSON for the PoC compiler or a YAML profile document (alexandria.profiles/v1) for the DSL compiler."
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="primary" loading={create.isPending} onClick={() => create.mutate(undefined)}>Create draft</Button>
        </>
      }
    >
      <CodeEditor value={text} onChange={setText} language={guessLanguage(text)} minHeight="340px" maxHeight="52vh" />
    </Dialog>
  )
}

// DraftEditor edits one draft with save (If-Match), validate and propose actions.
function DraftEditor({ slug, id, onProposed }: { slug: string; id: string; onProposed: (candidateId: string) => void }) {
  const qc = useQueryClient()
  const q = useQuery(hubQueries.draft(slug, id))
  const [text, setText] = useState<string | null>(null)
  const [report, setReport] = useState<ValidationReport | null>(null)
  const loaded = q.data?.draft.document !== undefined ? prettyJSON(q.data.draft.document) : ''
  useEffect(() => { if (q.data) setText(loaded) }, [q.data, loaded])
  const dirty = text !== null && text !== loaded
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
  if (q.isPending) return <div className="flex h-40 items-center justify-center"><Spinner /></div>
  if (q.isError) return <ErrorState error={q.error} />
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-10 shrink-0 items-center gap-2 border-b px-3">
        <FileText className="size-3.5 text-subtle" />
        <Mono copy={id} className="font-medium">{shortDigest(id, 16)}</Mono>
        <Badge>rev {q.data.draft.rev}</Badge>
        {dirty ? <Badge tone="warning" dot>unsaved</Badge> : null}
        <div className="ml-auto flex items-center gap-1.5">
          <Button size="sm" icon={<ShieldCheck className="size-3.5" />} loading={validate.isPending} disabled={dirty} onClick={() => validate.mutate(undefined)}>Validate</Button>
          <Button size="sm" icon={<Save className="size-3.5" />} loading={save.isPending} disabled={!dirty} onClick={() => save.mutate(undefined)}>Save</Button>
          <Button size="sm" variant="primary" icon={<Send className="size-3.5" />} loading={propose.isPending} disabled={dirty} onClick={() => propose.mutate(undefined)}>Propose</Button>
        </div>
      </div>
      <div className="min-h-0 flex-1 p-3">
        <CodeEditor value={text ?? ''} onChange={setText} language={guessLanguage(text ?? '')} className="h-full" />
      </div>
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

// CandidateReview shows a frozen candidate with its planned dates and the approve and reject decisions.
function CandidateReview({ slug, id }: { slug: string; id: string }) {
  const q = useQuery(hubQueries.candidate(slug, id))
  const [approving, setApproving] = useState(false)
  const [rejecting, setRejecting] = useState(false)
  const [grace, setGrace] = useState('')
  const [comment, setComment] = useState('')
  const invalidate = [['hub', 'ns', slug]]
  const approve = useAction(() => hub.approveCandidate(slug, id, grace || undefined), {
    success: (r) => `Version ${r.data.credentialSubject.version} published and signed`,
    invalidate,
    onSuccess: () => setApproving(false),
  })
  const reject = useAction(() => hub.rejectCandidate(slug, id, comment), { success: 'Candidate rejected', invalidate, onSuccess: () => setRejecting(false) })
  if (q.isPending) return <div className="flex h-40 items-center justify-center"><Spinner /></div>
  if (q.isError) return <ErrorState error={q.error} />
  const c = q.data
  const p = c.planned
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-10 shrink-0 items-center gap-2 border-b px-3">
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
          <p className="label-caps mb-1.5">Diff against the current head</p>
          {c.diff?.length ? (
            <ul className="rounded-md border bg-side p-2 font-mono text-xs leading-5">
              {c.diff.map((d, i) => (
                <li key={i} className={cn(d.startsWith('+') ? 'text-success' : d.startsWith('-') ? 'text-danger' : 'text-muted')}>{d}</li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-muted">No differences reported (initial release or identical content).</p>
          )}
        </div>
        <div>
          <p className="label-caps mb-1.5">Lifecycle parameters</p>
          <CodeEditor readOnly copy value={prettyJSON(c.lifecycle)} maxHeight="260px" />
        </div>
      </div>

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
