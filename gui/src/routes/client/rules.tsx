// Rules view: the dynamic rule set of the client (Appendix A.6) with CRUD under optimistic concurrency on the revision ETag.
// Each rule maps a listener, method and path template to gates and a mode (observe, shadow, enforce).
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { FileSignature, Plus, Save, Trash2 } from 'lucide-react'
import { cn } from '@/lib/cn'
import { client, clientQueries, type Mode, type Rule, type Ruleset } from '@/lib/client'
import { prettyJSON } from '@/lib/format'
import { useAction } from '@/lib/hooks'
import { CodeEditor } from '@/components/code-editor'
import { DataTable, FilterInput, type Column } from '@/components/data-table'
import { DetailPanel, Split } from '@/components/detail-panel'
import { ModeBadge } from '@/components/lifecycle'
import { ErrorState } from '@/components/problem'
import { Button } from '@/components/ui/button'
import { Badge, Empty, KV, Method, Mono, Page, Spinner } from '@/components/ui/misc'
import { Dialog } from '@/components/ui/overlay'

const KEY = [['client', 'rules'], ['client', 'status']]

const columns: Column<Rule>[] = [
  { accessorKey: 'priority', header: 'Prio', cell: (c) => <span className="tnum font-mono text-xs text-subtle">{c.row.original.priority}</span>, meta: { className: 'w-14' } },
  { accessorKey: 'id', header: 'Rule', cell: (c) => <span className="font-medium">{c.row.original.id}</span>, meta: { className: 'w-48' } },
  { accessorKey: 'listener', header: 'Listener', cell: (c) => <Badge tone={c.row.original.listener === 'dsp-egress' ? 'purple' : 'neutral'}>{c.row.original.listener}</Badge>, meta: { className: 'w-28' } },
  {
    id: 'match',
    header: 'Match',
    accessorFn: (r) => r.match.path,
    cell: (c) => (
      <span className="flex items-center gap-2">
        <Method value={c.row.original.match.method} className="w-9" />
        <span className="truncate font-mono text-xs">{c.row.original.match.path}</span>
        {c.row.original.match.body?.length ? <Badge>+body</Badge> : null}
      </span>
    ),
  },
  { id: 'gates', header: 'Gates', enableSorting: false, cell: (c) => <span className="flex gap-1">{c.row.original.verify.gates.map((g) => <Badge key={g} tone="accent" className="font-mono">{g}</Badge>)}</span>, meta: { className: 'w-60' } },
  { id: 'mode', header: 'Mode', accessorFn: (r) => r.mode ?? 'observe', cell: (c) => <ModeBadge mode={c.row.original.mode ?? 'observe'} />, meta: { className: 'w-20' } },
]

const newRule = (rs: Ruleset): Rule => ({
  id: 'custom-rule',
  priority: Math.max(0, ...rs.rules.map((r) => r.priority)) + 10,
  listener: 'dsp-ingress',
  match: { method: 'POST', path: '/diagnostics/{id}' },
  message: 'ContractRequestMessage',
  verify: { gates: ['dsp_message'] },
  mode: 'observe',
})

// RulesPage renders the rule table with its inspector.
export function RulesPage() {
  const q = useQuery(clientQueries.rules())
  const [filter, setFilter] = useState('')
  const [selected, setSelected] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const current = q.data?.rules.find((r) => r.id === selected)
  return (
    <Page
      flush
      title="Rules"
      subtitle={q.data ? <>Revision <span className="tnum font-mono">{q.data.revision}</span> · {q.data.rules.length} rules · changes require <span className="font-mono">If-Match: "{q.data.revision}"</span></> : 'Dynamic rule set'}
      actions={
        <>
          <FilterInput value={filter} onChange={setFilter} placeholder="Filter rules" />
          <Button variant="primary" icon={<Plus className="size-3.5" />} disabled={!q.data} onClick={() => setAdding(true)}>Add rule</Button>
        </>
      }
    >
      {q.isPending ? (
        <div className="flex h-40 items-center justify-center"><Spinner /></div>
      ) : q.isError ? (
        <ErrorState error={q.error} />
      ) : (
        <Split detail={current ? <RuleDetail key={`${current.id}@${q.data.revision}`} rule={current} revision={q.data.revision} onClose={() => setSelected(null)} /> : undefined}>
          <DataTable
            data={q.data.rules}
            minWidth={920}
            columns={columns}
            filter={filter}
            initialSort={[{ id: 'priority', desc: false }]}
            rowId={(r) => r.id}
            selected={selected}
            onRowClick={(r) => setSelected(r.id)}
            empty={<Empty icon={<FileSignature />} title="No rules">Without rules every request takes the default action.</Empty>}
          />
        </Split>
      )}
      {q.data ? <AddRule ruleset={q.data} open={adding} onOpenChange={setAdding} onAdded={setSelected} /> : null}
    </Page>
  )
}

// RuleDetail inspects one rule: quick mode switch, JSON editing and deletion.
function RuleDetail({ rule, revision, onClose }: { rule: Rule; revision: number; onClose: () => void }) {
  const original = prettyJSON(rule)
  const [text, setText] = useState(original)
  const [confirm, setConfirm] = useState(false)
  useEffect(() => setText(original), [original])
  const update = useAction((body: string) => client.updateRule(rule.id, body, revision), { success: (r) => `Rule saved · revision ${r.data.revision}`, invalidate: KEY })
  const remove = useAction(() => client.deleteRule(rule.id, revision), { success: `Rule ${rule.id} deleted`, invalidate: KEY, onSuccess: onClose })
  const mode = rule.mode ?? 'observe'
  return (
    <DetailPanel
      title={rule.id}
      onClose={onClose}
      actions={
        <>
          <Button size="sm" variant="danger" icon={<Trash2 className="size-3.5" />} onClick={() => setConfirm(true)}>Delete</Button>
          <Button size="sm" variant="primary" icon={<Save className="size-3.5" />} disabled={text === original} loading={update.isPending} onClick={() => update.mutate(text)}>Save</Button>
        </>
      }
    >
      <div className="flex h-full flex-col gap-3 p-3">
        <div>
          <p className="label-caps mb-1.5">Mode</p>
          <div className="grid grid-cols-3 gap-1 rounded-md bg-active p-0.5">
            {(['observe', 'shadow', 'enforce'] as Mode[]).map((m) => (
              <button
                key={m}
                disabled={update.isPending}
                onClick={() => m !== mode && update.mutate(JSON.stringify({ ...rule, mode: m }))}
                className={cn('h-6 rounded-sm text-sm capitalize transition-colors', mode === m ? cn('bg-bg font-medium shadow-sm', m === 'enforce' ? 'text-danger' : m === 'shadow' ? 'text-warning' : 'text-fg') : 'text-muted hover:text-fg')}
              >
                {m}
              </button>
            ))}
          </div>
          <p className="mt-1.5 text-xs text-muted">
            {mode === 'observe' ? 'Classifies silently; always forwards.' : mode === 'shadow' ? 'Forwards and emits verdict events for non-ok classifications.' : 'Rejects violations (400 + VHub-Verdict) and adds a notice header on warnings.'}
          </p>
        </div>
        <KV
          items={[
            ['Listener', <Mono>{rule.listener}</Mono>],
            ['Match', <span className="flex items-center gap-2"><Method value={rule.match.method} /><Mono>{rule.match.path}</Mono></span>],
            ['Message', <Mono>{rule.message}</Mono>],
            ['Gates', <span className="flex flex-wrap gap-1">{rule.verify.gates.map((g) => <Badge key={g} tone="accent" className="font-mono">{g}</Badge>)}</span>],
            ...(rule.validate_response ? [['Response', <span className="flex items-center gap-1"><Badge tone="accent" className="font-mono">{rule.validate_response.gate}</Badge><ModeBadge mode={rule.validate_response.mode ?? 'shadow'} /></span>] as [string, React.ReactNode]] : []),
          ]}
        />
        <div className="flex min-h-48 flex-1 flex-col">
          <p className="label-caps mb-1.5">Definition</p>
          <CodeEditor value={text} onChange={setText} className="min-h-0 flex-1" />
        </div>
      </div>
      <Dialog
        open={confirm}
        onOpenChange={setConfirm}
        title={`Delete rule ${rule.id}?`}
        description="Matching requests will fall through to lower-priority rules or the default action."
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirm(false)}>Cancel</Button>
            <Button variant="danger" loading={remove.isPending} onClick={() => remove.mutate(undefined)}>Delete rule</Button>
          </>
        }
      >
        <p className="text-sm text-muted">The rule set revision will advance to {revision + 1}.</p>
      </Dialog>
    </DetailPanel>
  )
}

// AddRule is the dialog that appends a rule from an editable JSON template.
function AddRule({ ruleset, open, onOpenChange, onAdded }: { ruleset: Ruleset; open: boolean; onOpenChange: (o: boolean) => void; onAdded: (id: string) => void }) {
  const [text, setText] = useState('')
  useEffect(() => { if (open) setText(prettyJSON(newRule(ruleset))) }, [open]) // eslint-disable-line react-hooks/exhaustive-deps
  const add = useAction(() => client.addRule(text, ruleset.revision), {
    success: 'Rule added',
    invalidate: KEY,
    onSuccess: () => {
      onOpenChange(false)
      try { onAdded((JSON.parse(text) as Rule).id) } catch { /* YAML input: leave the selection unchanged. */ }
    },
  })
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      width="max-w-2xl"
      title="Add rule"
      description="JSON or YAML. The whole set is re-validated (RULE-1…RULE-5) before it is swapped atomically."
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="primary" loading={add.isPending} onClick={() => add.mutate(undefined)}>Add rule</Button>
        </>
      }
    >
      <CodeEditor value={text} onChange={setText} minHeight="300px" maxHeight="50vh" />
    </Dialog>
  )
}
