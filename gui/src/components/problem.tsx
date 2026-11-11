// Error surfaces for problem+json responses: an inline banner, a full-panel state and a toast helper.
// They show the F5 catalog code, HTTP status and nested findings so failures are actionable.
import type { ReactNode } from 'react'
import { AlertTriangle, KeyRound, PlugZap, ShieldAlert } from 'lucide-react'
import { toast } from 'sonner'
import { asApiError, type Problem } from '@/lib/http'
import { cn } from '@/lib/cn'
import { Badge, Empty } from '@/components/ui/misc'

// ProblemBanner renders a coded error inline, including its findings.
export function ProblemBanner({ error, className }: { error: unknown; className?: string }) {
  const p = asApiError(error).problem
  return (
    <div role="alert" className={cn('rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-sm', className)}>
      <div className="flex flex-wrap items-center gap-2">
        <AlertTriangle className="size-3.5 shrink-0 text-danger" />
        <span className="font-medium text-fg">{p.title}</span>
        {p.code && p.code !== p.title ? <Badge tone="danger" className="font-mono">{p.code}</Badge> : null}
        {p.status ? <span className="tnum text-xs text-muted">HTTP {p.status}</span> : null}
      </div>
      {p.detail ? <p className="mt-1 break-words text-muted">{p.detail}</p> : null}
      {p.findings?.length ? <FindingList findings={p.findings} /> : null}
    </div>
  )
}

// FindingList renders the nested findings of a validation problem.
export function FindingList({ findings }: { findings: Problem[] }) {
  return (
    <ul className="mt-2 space-y-1 border-t border-danger/20 pt-2">
      {findings.map((f, i) => (
        <li key={i} className="flex flex-wrap items-baseline gap-x-2 text-xs">
          {f.code ? <span className="font-mono font-medium text-danger">{f.code}</span> : null}
          <span className="text-fg">{f.detail || f.title}</span>
          {f.path ? <span className="font-mono text-subtle">{f.path}{f.line ? `:${f.line}` : ''}</span> : null}
        </li>
      ))}
    </ul>
  )
}

// ErrorState renders a failed query as a full-panel placeholder tailored to the failure class.
export function ErrorState({ error, action }: { error: unknown; action?: ReactNode }) {
  const p = asApiError(error).problem
  if (p.status === 0) {
    return (
      <Empty icon={<PlugZap />} title="Service unreachable" action={action}>
        {p.detail}
      </Empty>
    )
  }
  if (p.status === 401) {
    return (
      <Empty icon={<KeyRound />} title="Authentication required" action={action}>
        This resource needs a valid bearer token. Sign in with OIDC or paste an access token from the identity menu.
      </Empty>
    )
  }
  if (p.status === 403) {
    return (
      <Empty icon={<ShieldAlert />} title="Not allowed" action={action}>
        {p.detail || 'The active identity lacks the role required for this resource.'}
      </Empty>
    )
  }
  return (
    <div className="p-4">
      <ProblemBanner error={error} />
    </div>
  )
}

// toastError reports a failed mutation as a toast with its catalog code.
export function toastError(error: unknown): void {
  const p = asApiError(error).problem
  toast.error(p.code && p.code !== p.title ? `${p.title} · ${p.code}` : p.title, { description: p.detail })
}
