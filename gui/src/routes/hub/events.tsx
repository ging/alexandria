// Events view: the append-only governance log, either for one namespace or hub-wide (admin only).
// Rows are derived from /v1/events and can be filtered by any field; newest entries come first.
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { RefreshCw, ScrollText } from 'lucide-react'
import { formatInstant, iriTail, relativeTo } from '@/lib/format'
import { useNs } from '@/lib/hooks'
import { hubQueries, type HubEvent } from '@/lib/hub'
import { DataTable, FilterInput, type Column } from '@/components/data-table'
import { ErrorState } from '@/components/problem'
import { IconButton } from '@/components/ui/button'
import { Badge, Empty, Mono, Page, Spinner, type Tone } from '@/components/ui/misc'

// eventTone colors an event type by its family and outcome.
function eventTone(type: string): Tone {
  if (/revoked|rejected|aborted|removed|unpublished/.test(type)) return 'danger'
  if (/published|approved/.test(type)) return 'success'
  if (/proposed|listing/.test(type)) return 'info'
  if (/draft|vocabulary/.test(type)) return 'purple'
  return 'neutral'
}

const columns: Column<HubEvent>[] = [
  { id: 'n', header: '#', accessorFn: (e) => Number(iriTail(e.id)), cell: (c) => <span className="tnum font-mono text-xs text-subtle">{iriTail(c.row.original.id)}</span>, meta: { className: 'w-14' } },
  { accessorKey: 'ts', header: 'Time', cell: (c) => <span className="tnum text-muted" title={relativeTo(c.row.original.ts)}>{formatInstant(c.row.original.ts)}</span>, meta: { className: 'w-52' } },
  { accessorKey: 'type', header: 'Type', cell: (c) => <Badge tone={eventTone(c.row.original.type)} className="font-mono">{c.row.original.type}</Badge>, meta: { className: 'w-44' } },
  { accessorKey: 'namespace', header: 'Namespace', cell: (c) => (c.row.original.namespace ? <span className="font-mono text-xs">{iriTail(c.row.original.namespace)}</span> : <span className="text-subtle">—</span>), meta: { className: 'w-28' } },
  { accessorKey: 'principal', header: 'Principal', cell: (c) => <span className="font-mono text-xs text-muted" title={c.row.original.principal}>{c.row.original.principal.split('#').pop() || 'system'}</span>, meta: { className: 'w-28' } },
  { accessorKey: 'subject', header: 'Subject', enableSorting: false, cell: (c) => <Mono copy={c.row.original.subject} className="text-muted">{c.row.original.subject}</Mono> },
]

// EventsView renders an event log for a namespace or, without one, for the whole hub.
function EventsView({ slug }: { slug?: string }) {
  const q = useQuery({ ...hubQueries.events(slug), refetchInterval: 10_000 })
  const [filter, setFilter] = useState('')
  return (
    <Page
      flush
      title={slug ? 'Events' : 'Hub events'}
      subtitle={slug ? <>Audit trail of namespace <span className="font-mono">{slug}</span> (stewards and admin).</> : 'Global audit trail across every namespace (hub admin only).'}
      actions={
        <>
          {q.data ? <span className="tnum text-xs text-subtle">{q.data.length} events</span> : null}
          <FilterInput value={filter} onChange={setFilter} placeholder="Filter by type, principal, subject" className="w-64" />
          <IconButton label="Refresh" onClick={() => void q.refetch()}><RefreshCw className={q.isFetching ? 'size-3.5 animate-spin' : 'size-3.5'} /></IconButton>
        </>
      }
    >
      {q.isPending ? (
        <div className="flex h-40 items-center justify-center"><Spinner /></div>
      ) : q.isError ? (
        <ErrorState error={q.error} />
      ) : (
        <DataTable minWidth={940} data={q.data} columns={columns} filter={filter} initialSort={[{ id: 'n', desc: true }]} empty={<Empty icon={<ScrollText />} title={filter ? 'No event matches the filter' : 'No events recorded'} />} />
      )}
    </Page>
  )
}

// NamespaceEventsPage renders the log of the namespace in the route.
export function NamespaceEventsPage() {
  return <EventsView slug={useNs()} />
}

// HubEventsPage renders the global log.
export function HubEventsPage() {
  return <EventsView />
}
