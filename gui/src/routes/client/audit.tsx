// Audit view: the latest passive catalog audit, predicting how stored catalog responses would classify under the next version.
// Reads GET /audit/reports/latest; a 404 simply means no report has been generated yet.
import { useQuery } from '@tanstack/react-query'
import { ClipboardCheck, RefreshCw } from 'lucide-react'
import { clientQueries, type AuditReport } from '@/lib/client'
import { formatInstant } from '@/lib/format'
import { DataTable, type Column } from '@/components/data-table'
import { StateBadge } from '@/components/lifecycle'
import { ErrorState } from '@/components/problem'
import { IconButton } from '@/components/ui/button'
import { Badge, Empty, Page, Spinner } from '@/components/ui/misc'

type Entry = NonNullable<AuditReport['entries']>[number]

const columns: Column<Entry>[] = [
  { accessorKey: 'response', header: 'Response', cell: (c) => <span className="tnum font-mono text-xs">#{c.row.original.response}</span>, meta: { className: 'w-24' } },
  { accessorKey: 'at', header: 'Observed', cell: (c) => <span className="tnum text-muted">{formatInstant(c.row.original.at)}</span>, meta: { className: 'w-56' } },
  { accessorKey: 'classification', header: 'Classification', cell: (c) => <StateBadge state={c.row.original.classification} />, meta: { className: 'w-32' } },
  { accessorKey: 'code', header: 'Code', cell: (c) => (c.row.original.code ? <Badge tone="danger" className="font-mono">{c.row.original.code}</Badge> : <span className="text-subtle">—</span>) },
]

// AuditPage renders the latest audit report.
export function AuditPage() {
  const q = useQuery(clientQueries.audit())
  const entries = q.data?.entries ?? []
  const bad = entries.filter((e) => e.classification !== 'ok').length
  return (
    <Page
      flush
      title="Audit"
      subtitle={q.data ? <>Against version <span className="font-mono">{q.data.version}</span> · generated {formatInstant(q.data.generated_at)} · {bad} of {entries.length} responses would not pass</> : 'Passive catalog audit of recorded responses'}
      actions={<IconButton label="Refresh" onClick={() => void q.refetch()}><RefreshCw className={q.isFetching ? 'size-3.5 animate-spin' : 'size-3.5'} /></IconButton>}
    >
      {q.isPending ? (
        <div className="flex h-40 items-center justify-center"><Spinner /></div>
      ) : q.isError ? (
        <ErrorState error={q.error} />
      ) : !q.data ? (
        <Empty icon={<ClipboardCheck />} title="No audit report yet">A report is produced when a new version is scheduled and catalog responses have been observed on rules with validate_response.</Empty>
      ) : (
        <DataTable data={entries} columns={columns} empty={<Empty icon={<ClipboardCheck />} title="Report has no entries" />} />
      )}
    </Page>
  )
}
