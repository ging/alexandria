// Metrics view: the OpenMetrics exposition of the client parsed into headline figures and a classification breakdown.
// Covers the three families of §4.11: classifications, listing age and request latency.
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Gauge } from 'lucide-react'
import { clientQueries } from '@/lib/client'
import { formatSpan } from '@/lib/format'
import { parseMetrics } from '@/lib/metrics'
import { CodeEditor } from '@/components/code-editor'
import { DataTable, type Column } from '@/components/data-table'
import { StateBadge } from '@/components/lifecycle'
import { ErrorState } from '@/components/problem'
import { Badge, Empty, Page, Spinner, Tabs } from '@/components/ui/misc'

// Row is one classification counter.
interface Row {
  rule: string
  gate: string
  version: string
  result: string
  count: number
}

// MetricsPage renders the parsed metrics and the raw exposition.
export function MetricsPage() {
  const q = useQuery(clientQueries.metrics())
  const [tab, setTab] = useState<'table' | 'raw'>('table')
  const samples = useMemo(() => parseMetrics(q.data ?? ''), [q.data])
  const value = (name: string) => samples.find((s) => s.name === name)?.value
  const rows: Row[] = useMemo(
    () => samples.filter((s) => s.name === 'vhub_classifications_total').map((s) => ({ rule: s.labels.rule, gate: s.labels.gate, version: s.labels.version, result: s.labels.result, count: s.value })),
    [samples],
  )
  const total = rows.reduce((n, r) => n + r.count, 0)
  const columns: Column<Row>[] = useMemo(
    () => [
      { accessorKey: 'rule', header: 'Rule', cell: (c) => <span className="font-medium">{c.row.original.rule}</span> },
      { accessorKey: 'gate', header: 'Gate', cell: (c) => <Badge tone="accent" className="font-mono">{c.row.original.gate}</Badge> },
      { accessorKey: 'version', header: 'Version', cell: (c) => <span className="font-mono text-xs">{c.row.original.version || '—'}</span>, meta: { className: 'w-24' } },
      { accessorKey: 'result', header: 'Result', cell: (c) => <StateBadge state={c.row.original.result} />, meta: { className: 'w-28' } },
      {
        accessorKey: 'count',
        header: 'Count',
        cell: (c) => (
          <span className="flex items-center gap-2">
            <span className="tnum w-10 text-right font-mono text-xs">{c.row.original.count}</span>
            <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-active"><span className="block h-full rounded-full bg-accent" style={{ width: `${total ? (c.row.original.count / total) * 100 : 0}%` }} /></span>
          </span>
        ),
        meta: { className: 'w-56' },
      },
    ],
    [total],
  )
  const count = value('vhub_request_latency_seconds_count') ?? 0
  const sum = value('vhub_request_latency_seconds_sum') ?? 0
  const age = value('vhub_listing_age_seconds')
  const tiles: [string, string, string][] = [
    ['Classifications', String(total), 'vhub_classifications_total'],
    ['Requests', String(count), 'vhub_request_latency_seconds_count'],
    ['Mean latency', count ? `${((sum / count) * 1000).toFixed(2)} ms` : '—', 'sum / count'],
    ['Listing age', age != null ? formatSpan(age * 1000) : '—', 'vhub_listing_age_seconds'],
  ]
  return (
    <Page flush title="Metrics" subtitle="OpenMetrics exposition of the admin listener (GET /metrics), refreshed every 5 s.">
      {q.isPending ? (
        <div className="flex h-40 items-center justify-center"><Spinner /></div>
      ) : q.isError ? (
        <ErrorState error={q.error} />
      ) : (
        <div className="flex h-full flex-col">
          <div className="grid shrink-0 grid-cols-4 gap-px border-b bg-line">
            {tiles.map(([label, v, hint]) => (
              <div key={label} className="bg-bg px-4 py-3">
                <p className="text-xs text-muted">{label}</p>
                <p className="tnum mt-0.5 text-xl font-semibold tracking-tight">{v}</p>
                <p className="font-mono text-2xs text-subtle">{hint}</p>
              </div>
            ))}
          </div>
          <Tabs value={tab} onChange={setTab} tabs={[{ id: 'table', label: 'Classifications', count: rows.length }, { id: 'raw', label: 'Raw exposition' }]} />
          <div className="min-h-0 flex-1 overflow-auto">
            {tab === 'table' ? (
              <DataTable data={rows} columns={columns} initialSort={[{ id: 'count', desc: true }]} empty={<Empty icon={<Gauge />} title="No classifications yet">Counters appear once DSP traffic crosses a rule with gates.</Empty>} />
            ) : (
              <div className="h-full p-3"><CodeEditor readOnly copy language="text" value={q.data} className="h-full" /></div>
            )}
          </div>
        </div>
      )}
    </Page>
  )
}
