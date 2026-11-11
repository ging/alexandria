// Generic dense data table built on TanStack Table v9 with client-side sorting, quick filter and row selection callback.
// Markup and styling are owned here so every list in the console shares one compact grid look.
import { useMemo, useState, type ReactNode } from 'react'
import { createSortedRowModel, rowSortingFeature, sortFns, tableFeatures, useTable, type ColumnDef, type RowData, type SortingState } from '@tanstack/react-table'
import { ArrowDown, ArrowUp, Search } from 'lucide-react'
import { cn } from '@/lib/cn'

const features = tableFeatures({ rowSortingFeature, sortedRowModel: createSortedRowModel(), sortFns })

// Column is a column definition bound to the table feature set.
export type Column<T extends RowData> = ColumnDef<typeof features, T> & { meta?: { className?: string } }

// DataTable renders rows with sortable headers; filter is matched against the stringified row.
export function DataTable<T extends object>({
  data,
  columns,
  onRowClick,
  selected,
  rowId,
  filter,
  empty,
  initialSort,
  className,
  minWidth = 640,
}: {
  data: T[]
  columns: Column<T>[]
  onRowClick?: (row: T) => void
  selected?: string | null
  rowId?: (row: T) => string
  filter?: string
  empty?: ReactNode
  initialSort?: SortingState
  className?: string
  minWidth?: number
}) {
  const [sorting, setSorting] = useState<SortingState>(initialSort ?? [])
  const rows = useMemo(() => {
    const q = filter?.trim().toLowerCase()
    return q ? data.filter((r) => JSON.stringify(r).toLowerCase().includes(q)) : data
  }, [data, filter])
  const table = useTable({ features, columns: columns as ColumnDef<typeof features, T>[], data: rows, state: { sorting }, onSortingChange: setSorting })

  if (rows.length === 0 && empty) return <>{empty}</>
  return (
    <div className={cn('overflow-auto', className)}>
      <table className="w-full table-fixed border-separate border-spacing-0 text-sm" style={{ minWidth }}>
        <thead className="sticky top-0 z-10 bg-side">
          {table.getHeaderGroups().map((group) => (
            <tr key={group.id}>
              {group.headers.map((header) => {
                const sort = header.column.getIsSorted()
                const can = header.column.getCanSort()
                return (
                  <th
                    key={header.id}
                    aria-sort={sort === 'asc' ? 'ascending' : sort === 'desc' ? 'descending' : undefined}
                    className={cn('h-7 border-b px-3 text-left align-middle text-xs font-medium whitespace-nowrap text-muted first:pl-4 last:pr-4', (header.column.columnDef as Column<T>).meta?.className)}
                  >
                    {header.isPlaceholder ? null : (
                      <button type="button" disabled={!can} onClick={header.column.getToggleSortingHandler()} className={cn('inline-flex items-center gap-1', can && 'hover:text-fg')}>
                        <table.FlexRender header={header} />
                        {sort === 'asc' ? <ArrowUp className="size-3" /> : sort === 'desc' ? <ArrowDown className="size-3" /> : null}
                      </button>
                    )}
                  </th>
                )
              })}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => {
            const id = rowId?.(row.original)
            const active = selected != null && id === selected
            return (
              <tr
                key={row.id}
                onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                tabIndex={onRowClick ? 0 : undefined}
                onKeyDown={onRowClick ? (e) => e.key === 'Enter' && onRowClick(row.original) : undefined}
                className={cn('group/row', onRowClick && 'cursor-pointer', active ? 'bg-accent-soft' : 'hover:bg-hover')}
              >
                {row.getAllCells().map((cell) => (
                  <td key={cell.id} className={cn('h-8 border-b px-3 align-middle first:pl-4 last:pr-4', (cell.column.columnDef as Column<T>).meta?.className)}>
                    <div className="truncate">
                      <table.FlexRender cell={cell} />
                    </div>
                  </td>
                ))}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// FilterInput renders the quick-filter box placed in page headers.
export function FilterInput({ value, onChange, placeholder = 'Filter', className }: { value: string; onChange: (v: string) => void; placeholder?: string; className?: string }) {
  return (
    <div className={cn('relative w-52', className)}>
      <Search className="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-subtle" />
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        spellCheck={false}
        className="h-7 w-full rounded-md border border-line-strong bg-input pr-2 pl-7 text-sm placeholder:text-subtle focus:border-accent focus:ring-1 focus:ring-accent focus:outline-none"
      />
    </div>
  )
}
