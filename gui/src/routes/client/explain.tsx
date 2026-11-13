// Explain view: a Postman-style dry run of a DSP message against the rule set via POST /rules:explain (REQ-CLI-032).
// It shows the selected rule, the per-gate classification, the resulting action and the VHub-Verdict header, without side effects.
import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowRight, FlaskConical, SendHorizontal } from 'lucide-react'
import { client, clientQueries, type Explanation, type GateExplanation, type Rule } from '@/lib/client'
import { prettyJSON } from '@/lib/format'
import { CodeEditor } from '@/components/code-editor'
import { ModeBadge, StateBadge } from '@/components/lifecycle'
import { ProblemBanner } from '@/components/problem'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/form'
import { Badge, Empty, KV, Method, Mono, Page, Tabs } from '@/components/ui/misc'

const listeners = ['dsp-ingress', 'dsp-egress', 'mgmt-ingress']
const methods = ['POST', 'GET', 'PUT', 'DELETE']
const sample = { '@type': 'ContractRequestMessage', providerPid: 'urn:uuid:p-1', consumerPid: 'urn:uuid:c-1', action: 'odrl:use' }

// concretize replaces URI template variables with sample identifiers.
const concretize = (path: string) => path.replace(/\{[^}]+\}/g, 'urn:uuid:p-1')

// ExplainPage renders the request builder and the decision report.
export function ExplainPage() {
  const rules = useQuery(clientQueries.rules())
  const [listener, setListener] = useState('dsp-ingress')
  const [method, setMethod] = useState('POST')
  const [path, setPath] = useState('/negotiations/urn:uuid:p-1/request')
  const [body, setBody] = useState(prettyJSON(sample))
  const [tab, setTab] = useState<'report' | 'json'>('report')
  const [parseError, setParseError] = useState<string | null>(null)
  const run = useMutation({ mutationFn: client.explain })
  const send = () => {
    let parsed: unknown = body
    setParseError(null)
    if (body.trim()) {
      try {
        parsed = JSON.parse(body)
      } catch (e) {
        setParseError(`Body is not valid JSON and is sent as a raw string: ${(e as Error).message}`)
      }
    } else parsed = null
    run.mutate({ listener, method, path, body: parsed })
  }
  const preset = (r: Rule) => {
    setListener(r.listener)
    setMethod(r.match.method)
    setPath(concretize(r.match.path))
    if (r.match.method !== 'GET') setBody(prettyJSON({ ...sample, '@type': r.message }))
  }
  return (
    <Page flush title="Explain" subtitle="What would the hot path decide for this message right now? Nothing is forwarded, recorded or emitted.">
      <div className="flex h-full flex-col">
        <form className="flex shrink-0 items-center gap-2 border-b px-6 py-3" onSubmit={(e) => { e.preventDefault(); send() }}>
          <Select className="w-36 [&>select]:h-8 [&>select]:font-mono [&>select]:text-xs" value={listener} onChange={(e) => setListener(e.target.value)} aria-label="Listener">
            {listeners.map((l) => <option key={l}>{l}</option>)}
          </Select>
          <div className="flex h-8 min-w-0 flex-1 items-center rounded-md border border-line-strong bg-input focus-within:border-accent focus-within:ring-1 focus-within:ring-accent">
            <select value={method} onChange={(e) => setMethod(e.target.value)} aria-label="Method" className="h-full appearance-none border-r bg-transparent px-2.5 font-mono text-xs font-semibold text-warning focus:outline-none">
              {methods.map((m) => <option key={m}>{m}</option>)}
            </select>
            <input value={path} onChange={(e) => setPath(e.target.value)} spellCheck={false} aria-label="Path" className="h-full min-w-0 flex-1 bg-transparent px-2.5 font-mono text-xs focus:outline-none" placeholder="/negotiations/{providerPid}/request" />
          </div>
          <Button type="submit" variant="primary" className="h-8 px-4" icon={<SendHorizontal className="size-3.5" />} loading={run.isPending}>Explain</Button>
        </form>
        <div className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-b bg-side px-6 py-2">
          <span className="label-caps mr-1 shrink-0">From rule</span>
          {rules.data?.rules.map((r) => (
            <button key={r.id} onClick={() => preset(r)} className="h-5 shrink-0 rounded-sm border bg-bg px-1.5 text-xs text-muted hover:border-accent/60 hover:text-fg">{r.id}</button>
          ))}
        </div>
        <div className="grid min-h-0 flex-1 grid-cols-2">
          <div className="flex min-h-0 flex-col border-r">
            <div className="flex h-9 shrink-0 items-center justify-between border-b px-6">
              <span className="label-caps">Message body</span>
              <span className="text-xs text-subtle">application/json</span>
            </div>
            <div className="min-h-0 flex-1 p-3"><CodeEditor value={body} onChange={setBody} className="h-full" /></div>
          </div>
          <div className="flex min-h-0 flex-col">
            {run.data ? (
              <>
                <Tabs value={tab} onChange={setTab} tabs={[{ id: 'report', label: 'Decision' }, { id: 'json', label: 'JSON' }]} trailing={<StateBadge state={run.data.data.action} />} />
                <div className="min-h-0 flex-1 overflow-auto p-3">
                  {parseError ? <p className="mb-2 text-xs text-warning">{parseError}</p> : null}
                  {tab === 'report' ? <Report e={run.data.data} /> : <CodeEditor readOnly copy value={prettyJSON(run.data.data)} className="h-full" />}
                </div>
              </>
            ) : run.isError ? (
              <div className="p-4"><ProblemBanner error={run.error} /></div>
            ) : (
              <Empty icon={<FlaskConical />} title="Run an explanation">Pick a rule preset or compose a request, then press Explain to see rule selection, gate classifications and the action.</Empty>
            )}
          </div>
        </div>
      </div>
    </Page>
  )
}

// Report renders the decision summary and one card per gate.
function Report({ e }: { e: Explanation }) {
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 rounded-md border bg-side px-3 py-2.5">
        <div className="min-w-0 flex-1">
          <p className="text-xs text-muted">Selected rule</p>
          <p className="truncate font-medium">{e.rule ?? <span className="text-muted">none — default action</span>}</p>
        </div>
        <ArrowRight className="size-4 text-subtle" />
        <div className="text-right">
          <p className="text-xs text-muted">Action</p>
          <StateBadge state={e.action} />
        </div>
      </div>
      <KV
        items={[
          ['Client state', <StateBadge state={e.state} />],
          ['Rules revision', <span className="tnum font-mono">{e.rules_revision}</span>],
          ...(e.code ? [['Code', <Badge tone="danger" className="font-mono">{e.code}</Badge>] as [string, React.ReactNode]] : []),
          ...(e.vhub_verdict ? [['VHub-Verdict', <Mono copy={e.vhub_verdict}>{e.vhub_verdict}</Mono>] as [string, React.ReactNode]] : []),
        ]}
      />
      {e.gates?.length ? (
        <div className="space-y-2">
          <p className="label-caps">Gates</p>
          {e.gates.map((g) => <GateCard key={g.gate} g={g} />)}
        </div>
      ) : null}
    </div>
  )
}

// GateCard renders the classification of one gate with its findings.
function GateCard({ g }: { g: GateExplanation }) {
  return (
    <div className="rounded-md border">
      <div className="flex flex-wrap items-center gap-2 border-b px-3 py-1.5">
        <Badge tone="accent" className="font-mono">{g.gate}</Badge>
        <ModeBadge mode={g.mode} />
        {g.evaluated ? <StateBadge state={g.classification ?? 'ok'} /> : <Badge>not evaluated</Badge>}
        <span className="ml-auto flex items-center gap-1.5 text-xs text-muted"><Method value="→" className="text-subtle" /> {g.action}</span>
      </div>
      <div className="space-y-1.5 px-3 py-2 text-xs">
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-muted">
          <span>matched version <span className="font-mono text-fg">{g.matched ?? '—'}</span></span>
          {g.sunset ? <span>sunset <span className="font-mono text-fg">{g.sunset}</span></span> : null}
          {g.codes?.length ? <span className="flex items-center gap-1">codes {g.codes.map((c) => <Badge key={c} tone="danger" className="font-mono">{c}</Badge>)}</span> : null}
        </div>
        {g.findings?.length ? (
          <ul className="space-y-1 border-t pt-1.5">
            {g.findings.map((f, i) => (
              <li key={i} className="flex flex-wrap items-baseline gap-x-2">
                <Badge tone={f.severity === 'violation' || f.severity === 'error' ? 'danger' : 'warning'}>{f.severity}</Badge>
                <span className="text-fg">{f.message}</span>
                {f.path ? <span className="font-mono text-subtle">{f.path}</span> : null}
                <span className="text-subtle">{f.slot} · {f.source}</span>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
    </div>
  )
}
