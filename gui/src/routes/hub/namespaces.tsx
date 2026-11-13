// Namespaces view of the server workspace: lists every namespace of the hub and lets an admin create new ones.
// Selecting a row opens the namespace overview, which scopes the rest of the hub navigation.
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Boxes, Plus, RefreshCw } from 'lucide-react'
import { formatInstant } from '@/lib/format'
import { useAction } from '@/lib/hooks'
import { hub, hubQueries, type Namespace } from '@/lib/hub'
import { DataTable, FilterInput, type Column } from '@/components/data-table'
import { ErrorState } from '@/components/problem'
import { Button, IconButton } from '@/components/ui/button'
import { Field, Input, Textarea } from '@/components/ui/form'
import { Empty, Mono, Page, Spinner } from '@/components/ui/misc'
import { Dialog } from '@/components/ui/overlay'

const columns: Column<Namespace>[] = [
  { accessorKey: 'slug', header: 'Slug', cell: (c) => <span className="font-mono text-xs font-medium text-accent">{c.row.original.slug}</span>, meta: { className: 'w-48' } },
  { accessorKey: 'title', header: 'Title', cell: (c) => c.row.original.title ?? <span className="text-subtle">—</span> },
  { accessorKey: 'description', header: 'Description', enableSorting: false, cell: (c) => <span className="text-muted">{c.row.original.description ?? '—'}</span> },
  { accessorKey: 'created_at', header: 'Created', cell: (c) => <span className="tnum text-muted">{formatInstant(c.row.original.created_at)}</span>, meta: { className: 'w-56' } },
]

// NamespacesPage renders the namespace list.
export function NamespacesPage() {
  const q = useQuery(hubQueries.namespaces())
  const navigate = useNavigate()
  const [filter, setFilter] = useState('')
  const [creating, setCreating] = useState(false)
  return (
    <Page
      flush
      title="Namespaces"
      subtitle="Units of logical separation of the hub (§3.2). Each namespace has its own stewards, vocabularies and release line."
      actions={
        <>
          <FilterInput value={filter} onChange={setFilter} placeholder="Filter namespaces" />
          <IconButton label="Refresh" onClick={() => void q.refetch()}>
            <RefreshCw className={q.isFetching ? 'size-3.5 animate-spin' : 'size-3.5'} />
          </IconButton>
          <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setCreating(true)}>
            New namespace
          </Button>
        </>
      }
    >
      {q.isPending ? (
        <div className="flex h-40 items-center justify-center"><Spinner /></div>
      ) : q.isError ? (
        <ErrorState error={q.error} action={<Button onClick={() => void q.refetch()}>Retry</Button>} />
      ) : (
        <DataTable
          data={q.data}
          columns={columns}
          filter={filter}
          initialSort={[{ id: 'slug', desc: false }]}
          onRowClick={(n) => void navigate({ to: `/hub/n/${n.slug}` })}
          empty={
            <Empty icon={<Boxes />} title={filter ? 'No namespace matches the filter' : 'No namespaces yet'} action={filter ? undefined : <Button variant="primary" onClick={() => setCreating(true)}>Create the first namespace</Button>}>
              {filter ? undefined : 'Creating a namespace requires an identity with the hub admin role.'}
            </Empty>
          }
        />
      )}
      <CreateNamespace open={creating} onOpenChange={setCreating} />
    </Page>
  )
}

// CreateNamespace is the dialog that registers a namespace.
function CreateNamespace({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const navigate = useNavigate()
  const [slug, setSlug] = useState('')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const create = useAction(() => hub.createNamespace({ slug, title: title || undefined, description: description || undefined }), {
    success: `Namespace ${slug} created`,
    invalidate: [['hub', 'namespaces']],
    onSuccess: () => {
      onOpenChange(false)
      void navigate({ to: `/hub/n/${slug}` })
      setSlug(''); setTitle(''); setDescription('')
    },
  })
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="New namespace"
      description="Requires the hub admin role."
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="primary" disabled={!slug} loading={create.isPending} onClick={() => create.mutate(undefined)}>Create namespace</Button>
        </>
      }
    >
      <form className="space-y-3" onSubmit={(e) => { e.preventDefault(); if (slug) create.mutate(undefined) }}>
        <Field label="Slug" hint={<>Becomes part of every URL: <Mono>/v1/namespaces/{slug || '{slug}'}</Mono></>}>
          <Input mono autoFocus value={slug} onChange={(e) => setSlug(e.target.value.toLowerCase())} placeholder="demo" />
        </Field>
        <Field label="Title">
          <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Demo data space" />
        </Field>
        <Field label="Description">
          <Textarea rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>
      </form>
    </Dialog>
  )
}
