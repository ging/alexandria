// Overlay primitives built on Radix: modal dialog, tooltip and dropdown menu with the console styling.
// Radix supplies focus management and keyboard behavior; this file only adds layout and tokens.
import type { ReactNode } from 'react'
import { Dialog as RDialog, DropdownMenu as RMenu, Tooltip as RTooltip } from 'radix-ui'
import { X } from 'lucide-react'
import { cn } from '@/lib/cn'

// Dialog renders a centered modal with a title bar, scrollable body and optional footer.
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  width = 'max-w-md',
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: ReactNode
  children: ReactNode
  footer?: ReactNode
  width?: string
}) {
  return (
    <RDialog.Root open={open} onOpenChange={onOpenChange}>
      <RDialog.Portal>
        <RDialog.Overlay className="fixed inset-0 z-40 animate-in bg-black/40" />
        <RDialog.Content
          className={cn(
            'fixed top-[12vh] left-1/2 z-50 flex max-h-[76vh] w-[calc(100vw-2rem)] -translate-x-1/2 animate-pop flex-col rounded-lg border border-line-strong bg-raised shadow-pop focus:outline-none',
            width,
          )}
        >
          <header className="flex items-start justify-between gap-4 border-b px-4 py-3">
            <div className="min-w-0">
              <RDialog.Title className="text-lg font-semibold text-fg">{title}</RDialog.Title>
              {description ? <RDialog.Description className="mt-0.5 text-sm text-muted">{description}</RDialog.Description> : <RDialog.Description className="sr-only">{title}</RDialog.Description>}
            </div>
            <RDialog.Close className="-mr-1 rounded-md p-1 text-subtle hover:bg-hover hover:text-fg" aria-label="Close">
              <X className="size-4" />
            </RDialog.Close>
          </header>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">{children}</div>
          {footer ? <footer className="flex items-center justify-end gap-2 border-t px-4 py-2.5">{footer}</footer> : null}
        </RDialog.Content>
      </RDialog.Portal>
    </RDialog.Root>
  )
}

// Tooltip shows a short hint on hover or focus.
export function Tooltip({ content, children, side = 'bottom' }: { content: ReactNode; children: ReactNode; side?: 'top' | 'right' | 'bottom' | 'left' }) {
  return (
    <RTooltip.Root>
      <RTooltip.Trigger asChild>{children}</RTooltip.Trigger>
      <RTooltip.Portal>
        <RTooltip.Content side={side} sideOffset={6} className="z-50 max-w-xs animate-in rounded-md border border-line-strong bg-raised px-2 py-1 text-xs text-fg shadow-pop">
          {content}
        </RTooltip.Content>
      </RTooltip.Portal>
    </RTooltip.Root>
  )
}

// Menu renders a dropdown anchored to its trigger.
export function Menu({ trigger, children, align = 'start' }: { trigger: ReactNode; children: ReactNode; align?: 'start' | 'end' | 'center' }) {
  return (
    <RMenu.Root>
      <RMenu.Trigger asChild>{trigger}</RMenu.Trigger>
      <RMenu.Portal>
        <RMenu.Content align={align} sideOffset={4} className="z-50 min-w-48 animate-pop rounded-md border border-line-strong bg-raised p-1 shadow-pop">
          {children}
        </RMenu.Content>
      </RMenu.Portal>
    </RMenu.Root>
  )
}

// MenuItem renders one selectable entry of a Menu.
export function MenuItem({ children, onSelect, icon, danger, hint }: { children: ReactNode; onSelect?: () => void; icon?: ReactNode; danger?: boolean; hint?: ReactNode }) {
  return (
    <RMenu.Item
      onSelect={onSelect}
      className={cn(
        'flex h-7 cursor-default items-center gap-2 rounded-sm px-2 text-sm outline-none select-none data-[highlighted]:bg-accent data-[highlighted]:text-accent-fg',
        danger ? 'text-danger' : 'text-fg',
      )}
    >
      <span className="flex size-3.5 items-center justify-center">{icon}</span>
      <span className="min-w-0 flex-1 truncate">{children}</span>
      {hint ? <span className="text-xs opacity-70">{hint}</span> : null}
    </RMenu.Item>
  )
}

// MenuLabel renders a section caption inside a Menu.
export function MenuLabel({ children }: { children: ReactNode }) {
  return <RMenu.Label className="label-caps px-2 pt-1.5 pb-1">{children}</RMenu.Label>
}

// MenuSeparator renders a divider between Menu sections.
export function MenuSeparator() {
  return <RMenu.Separator className="my-1 h-px bg-line" />
}
