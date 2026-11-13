// Small presentational primitives: badges, tabs, method tags, key/value grids, copy affordances and page scaffolding.
// They carry no data logic and only encode the visual language of the console.
import { useState, type ReactNode } from 'react'
import { Check, Copy, Loader2 } from 'lucide-react'
import { cn } from '@/lib/cn'

// Tone is the semantic color of a badge or indicator.
export type Tone = 'neutral' | 'accent' | 'success' | 'warning' | 'danger' | 'info' | 'purple'

const tones: Record<Tone, string> = {
  neutral: 'bg-active text-muted',
  accent: 'bg-accent-soft text-accent',
  success: 'bg-success-soft text-success',
  warning: 'bg-warning-soft text-warning',
  danger: 'bg-danger-soft text-danger',
  info: 'bg-info-soft text-info',
  purple: 'bg-purple-soft text-purple',
}

const dots: Record<Tone, string> = {
  neutral: 'bg-subtle',
  accent: 'bg-accent',
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  info: 'bg-info',
  purple: 'bg-purple',
}

// Badge renders a compact status pill, optionally with a leading dot.
export function Badge({ tone = 'neutral', dot, children, className }: { tone?: Tone; dot?: boolean; children: ReactNode; className?: string }) {
  return (
    <span className={cn('inline-flex h-5 items-center gap-1 rounded-sm px-1.5 text-xs font-medium whitespace-nowrap', tones[tone], className)}>
      {dot ? <span className={cn('size-1.5 rounded-full', dots[tone])} /> : null}
      {children}
    </span>
  )
}

// Dot renders a bare status indicator.
export function Dot({ tone, pulse }: { tone: Tone; pulse?: boolean }) {
  return <span className={cn('inline-block size-2 shrink-0 rounded-full', dots[tone], pulse && 'animate-pulse')} />
}

const methods: Record<string, string> = {
  GET: 'text-success',
  POST: 'text-warning',
  PUT: 'text-info',
  PATCH: 'text-purple',
  DELETE: 'text-danger',
}

// Method renders an HTTP method with semantic accent coloring.
export function Method({ value, className }: { value: string; className?: string }) {
  return <span className={cn('font-mono text-xs font-semibold tracking-tight', methods[value.toUpperCase()] ?? 'text-muted', className)}>{value.toUpperCase()}</span>
}

// Tabs renders an underlined tab strip controlled by the caller.
export function Tabs<T extends string>({ tabs, value, onChange, className, trailing }: { tabs: { id: T; label: ReactNode; count?: number }[]; value: T; onChange: (id: T) => void; className?: string; trailing?: ReactNode }) {
  return (
    <div role="tablist" className={cn('flex h-9 shrink-0 items-stretch gap-5 border-b px-6', className)}>
      {tabs.map((t) => (
        <button
          key={t.id}
          role="tab"
          aria-selected={value === t.id}
          onClick={() => onChange(t.id)}
          className={cn(
            'relative -mb-px flex items-center gap-1.5 text-sm transition-colors',
            value === t.id
              ? 'font-medium text-fg after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:rounded-full after:bg-accent-gradient'
              : 'text-muted hover:text-fg',
          )}
        >
          {t.label}
          {t.count != null ? <span className="tnum rounded-sm bg-active px-1.5 text-2xs text-muted">{t.count}</span> : null}
        </button>
      ))}
      {trailing ? <div className="ml-auto flex items-center gap-1.5">{trailing}</div> : null}
    </div>
  )
}

// CopyButton copies a value to the clipboard and confirms briefly.
export function CopyButton({ value, className }: { value: string; className?: string }) {
  const [done, setDone] = useState(false)
  return (
    <button
      type="button"
      aria-label="Copy"
      title="Copy"
      onClick={(e) => {
        e.stopPropagation()
        void navigator.clipboard.writeText(value)
        setDone(true)
        setTimeout(() => setDone(false), 1200)
      }}
      className={cn('inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-subtle hover:bg-hover hover:text-fg', className)}
    >
      {done ? <Check className="size-3 text-success" /> : <Copy className="size-3" />}
    </button>
  )
}

// Mono renders a monospace value with an optional hover copy affordance.
export function Mono({ children, copy, className, title }: { children: ReactNode; copy?: string; className?: string; title?: string }) {
  return (
    <span className={cn('group/mono inline-flex max-w-full items-center gap-1 font-mono text-xs', className)} title={title}>
      <span className="truncate">{children}</span>
      {copy ? <CopyButton value={copy} className="opacity-0 group-hover/mono:opacity-100 focus:opacity-100" /> : null}
    </span>
  )
}

// KV renders a definition grid of labelled values.
export function KV({ items, className }: { items: [label: string, value: ReactNode][]; className?: string }) {
  return (
    <dl className={cn('grid grid-cols-[minmax(110px,max-content)_minmax(0,1fr)] gap-x-4 gap-y-2', className)}>
      {items.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="text-muted">{k}</dt>
          <dd className="min-w-0 text-fg">{v}</dd>
        </div>
      ))}
    </dl>
  )
}

// Spinner renders an inline loading indicator.
export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn('size-4 animate-spin text-accent', className)} />
}

// Kbd renders a keyboard shortcut hint.
export function Kbd({ children }: { children: ReactNode }) {
  return <kbd className="rounded-sm border border-line-strong bg-bg px-1.5 font-sans text-2xs text-muted">{children}</kbd>
}

// Empty renders a centered placeholder for views without data.
export function Empty({ icon, title, children, action }: { icon?: ReactNode; title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex h-full min-h-56 flex-col items-center justify-center gap-2.5 px-6 py-12 text-center">
      {icon ? <div className="mb-1 text-accent/80 [&>svg]:size-8 [&>svg]:stroke-[1.35]">{icon}</div> : null}
      <p className="text-base font-semibold text-fg">{title}</p>
      {children ? <p className="max-w-sm text-sm leading-relaxed text-muted">{children}</p> : null}
      {action ? <div className="mt-2 flex gap-2">{action}</div> : null}
    </div>
  )
}

// Page is the scaffold of a routed view: header with title and actions, then a scrollable body.
export function Page({ title, subtitle, actions, children, tabs, flush }: { title: ReactNode; subtitle?: ReactNode; actions?: ReactNode; children: ReactNode; tabs?: ReactNode; flush?: boolean }) {
  return (
    <div className="flex h-full min-w-0 flex-col">
      <header className="flex min-h-14 shrink-0 items-center justify-between gap-4 border-b px-6 py-3">
        <div className="min-w-0">
          <h1 className="truncate text-lg font-semibold tracking-tight text-fg">{title}</h1>
          {subtitle ? <p className="mt-0.5 truncate text-xs text-muted">{subtitle}</p> : null}
        </div>
        {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
      </header>
      {tabs}
      <div className={cn('min-h-0 flex-1 overflow-auto', !flush && 'p-6')}>{children}</div>
    </div>
  )
}

// Section groups related content under a small caption.
export function Section({ title, actions, children, className }: { title: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={cn('rounded-lg border bg-raised', className)}>
      <header className="flex h-9 items-center justify-between gap-2 border-b px-4">
        <h2 className="label-caps">{title}</h2>
        {actions ? <div className="flex items-center gap-1.5">{actions}</div> : null}
      </header>
      <div className="p-4">{children}</div>
    </section>
  )
}
