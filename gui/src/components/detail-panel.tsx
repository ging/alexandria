// Master–detail layout helper: a list region with an optional right-hand inspector panel.
// Used by views where selecting a row reveals its details without leaving the list.
import type { ReactNode } from 'react'
import { X } from 'lucide-react'
import { IconButton } from '@/components/ui/button'

// Split lays out the master content and, when present, the detail panel beside it.
export function Split({ children, detail }: { children: ReactNode; detail?: ReactNode }) {
  return (
    <div className="flex h-full min-h-0">
      <div className="min-w-0 flex-1 overflow-auto">{children}</div>
      {detail}
    </div>
  )
}

// DetailPanel renders the inspector with a title bar, actions and a scrollable body.
export function DetailPanel({ title, subtitle, actions, onClose, children, width = 'w-[46%]' }: { title: ReactNode; subtitle?: ReactNode; actions?: ReactNode; onClose: () => void; children: ReactNode; width?: string }) {
  return (
    <aside className={`flex min-w-[380px] shrink-0 animate-in flex-col border-l bg-bg ${width}`}>
      <header className="flex h-10 shrink-0 items-center gap-2 border-b px-3">
        <div className="min-w-0 flex-1">
          <div className="truncate text-base font-semibold">{title}</div>
        </div>
        {subtitle}
        {actions}
        <IconButton label="Close panel" size="sm" onClick={onClose}>
          <X className="size-3.5" />
        </IconButton>
      </header>
      <div className="min-h-0 flex-1 overflow-auto">{children}</div>
    </aside>
  )
}
