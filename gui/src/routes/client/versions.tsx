// Client versions view: every lifecycle record the client knows, its reference state and whether the package is held on disk.
// Mirrors GET /versions and reuses the hub timeline so both sides of the synchronization can be compared.
import { useQuery } from '@tanstack/react-query'
import { ListTree, RefreshCw } from 'lucide-react'
import { client, clientQueries, type ClientVersion } from '@/lib/client'
import { formatInstant, shortDigest } from '@/lib/format'
import { useAction, useNow } from '@/lib/hooks'
import { hasOpenDialog, isEditableTarget, useWindowKeydown } from '@/lib/hotkeys'
import { DataTable, type Column } from '@/components/data-table'
import { ClassBadge, StateBadge, Timeline } from '@/components/lifecycle'
import { ErrorState } from '@/components/problem'
import { Button } from '@/components/ui/button'
import { Badge, Empty, Mono, Page, Spinner } from '@/components/ui/misc'

const instant = (key: 'published_at' | 'new_content_from' | 'enforced_from', header: string): Column<ClientVersion> => ({
  accessorKey: key,
  header,
  cell: (c) => <span className="tnum text-muted">{formatInstant(c.row.original[key])}</span>,
})

const columns: Column<ClientVersion>[] = [
  { accessorKey: 'version', header: 'Version', cell: (c) => <span className="font-mono text-xs font-semibold">{c.row.original.version}</span>, meta: { className: 'w-20' } },
  { accessorKey: 'state', header: 'State', cell: (c) => <StateBadge state={c.row.original.state} />, meta: { className: 'w-28' } },
  { accessorKey: 'class', header: 'Class', cell: (c) => <ClassBadge value={c.row.original.class} />, meta: { className: 'w-24' } },
  instant('published_at', 'Published'),
  instant('new_content_from', 'New content from'),
  instant('enforced_from', 'Enforced from'),
  { accessorKey: 'revoked_from', header: 'Revoked from', cell: (c) => (c.row.original.revoked_from ? <span className="tnum text-danger">{formatInstant(c.row.original.revoked_from)}</span> : <span className="text-subtle">—</span>) },
  { accessorKey: 'retained', header: 'Package', cell: (c) => (c.row.original.retained ? <Badge tone="success" dot>on disk</Badge> : <Badge>not held</Badge>), meta: { className: 'w-24' } },
  { accessorKey: 'manifest', header: 'Manifest', enableSorting: false, cell: (c) => <Mono copy={c.row.original.manifest} className="text-muted">{shortDigest(c.row.original.manifest)}</Mono>, meta: { className: 'w-36' } },
]

// ClientVersionsPage renders the synchronized lifecycle records.
export function ClientVersionsPage() {
  const q = useQuery({ ...clientQueries.versions(), refetchInterval: 10_000 })
  const now = useNow(30_000)
  const sync = useAction(() => client.sync(), {
    success: (r) => `Synced · listing sequence ${r.data.last_accepted_listing?.sequence ?? '—'}`,
    invalidate: [['client', 'versions'], ['client', 'status'], ['client', 'health'], ['client', 'audit']],
  })
  useWindowKeydown((e) => {
    if (e.key === 'r' && !e.metaKey && !e.ctrlKey && !e.altKey && !isEditableTarget(e.target) && !hasOpenDialog() && !sync.isPending) {
      e.preventDefault()
      sync.mutate(undefined)
    }
  })
  return (
    <Page
      flush
      title="Versions"
      subtitle="Lifecycle records accepted from the hub listing, with their reference state at the client's local instant."
      actions={
        <Button size="sm" icon={<RefreshCw className={sync.isPending ? 'size-3.5 animate-spin' : 'size-3.5'} />} loading={sync.isPending} onClick={() => sync.mutate(undefined)}>
          Sync now
        </Button>
      }
    >
      {q.isPending ? (
        <div className="flex h-40 items-center justify-center"><Spinner /></div>
      ) : q.isError ? (
        <ErrorState error={q.error} />
      ) : q.data.length === 0 ? (
        <Empty icon={<ListTree />} title="No versions synchronized">The client has not accepted a listing yet.</Empty>
      ) : (
        <>
          <div className="border-b px-6 py-3.5"><Timeline records={q.data} now={now} /></div>
          <DataTable minWidth={1180} data={q.data} columns={columns} />
        </>
      )}
    </Page>
  )
}
