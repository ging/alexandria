// Form primitives: text input, textarea, native select and the labelled Field wrapper.
// All controls share one clean style so forms align on a 32px rhythm.
import type { InputHTMLAttributes, ReactNode, Ref, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'
import { ChevronDown } from 'lucide-react'
import { cn } from '@/lib/cn'

const control =
  'w-full rounded-md border border-line-strong bg-input px-2.5 text-sm text-fg placeholder:text-subtle transition-colors focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent disabled:opacity-60'

// Input renders a single-line text control.
export function Input({ className, mono, ...rest }: InputHTMLAttributes<HTMLInputElement> & { mono?: boolean; ref?: Ref<HTMLInputElement> }) {
  return <input spellCheck={false} autoComplete="off" className={cn(control, 'h-8', mono && 'font-mono', className)} {...rest} />
}

// Textarea renders a multi-line text control.
export function Textarea({ className, mono, ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement> & { mono?: boolean }) {
  return <textarea spellCheck={false} className={cn(control, 'py-2 leading-[19px]', mono && 'font-mono', className)} {...rest} />
}

// Select renders a native select with the console chrome.
export function Select({ className, children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <div className={cn('relative', className)}>
      <select className={cn(control, 'h-8 appearance-none pr-7')} {...rest}>
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute top-1/2 right-2 size-3.5 -translate-y-1/2 text-subtle" />
    </div>
  )
}

// Field labels a control and shows its hint or error.
export function Field({ label, hint, error, children, className }: { label: string; hint?: ReactNode; error?: string; children: ReactNode; className?: string }) {
  return (
    <label className={cn('flex flex-col gap-1', className)}>
      <span className="text-xs font-medium text-muted">{label}</span>
      {children}
      {error ? <span className="text-xs text-danger">{error}</span> : hint ? <span className="text-xs text-subtle">{hint}</span> : null}
    </label>
  )
}
