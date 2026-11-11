// Lifecycle visuals shared by both workspaces: tone mapping for states and classes, and the version timeline.
// The timeline places published, new-content, enforced and revoked instants on one proportional axis.
import { cn } from '@/lib/cn'
import { formatInstant, relativeTo } from '@/lib/format'
import type { VersionRecord } from '@/lib/hub'
import { Badge, type Tone } from '@/components/ui/misc'
import { Tooltip } from '@/components/ui/overlay'

// stateTone maps client, lifecycle and candidate states to a badge tone.
export function stateTone(state: string): Tone {
  switch (state) {
    case 'Active':
    case 'Published':
    case 'head':
    case 'approved':
    case 'ok':
    case 'forward':
      return 'success'
    case 'Transition':
    case 'Deprecated':
    case 'in_force':
    case 'warning':
    case 'forward-notice':
      return 'warning'
    case 'Degraded':
    case 'Superseded':
      return 'purple'
    case 'FailClosed':
    case 'Revoked':
    case 'rejected':
    case 'violation':
    case 'reject':
      return 'danger'
    case 'Scheduled':
    case 'scheduled':
    case 'open':
    case 'Onboarding':
    case 'info':
      return 'info'
    default:
      return 'neutral'
  }
}

// StateBadge renders a state with its mapped tone.
export function StateBadge({ state, dot = true }: { state: string; dot?: boolean }) {
  return (
    <Badge tone={stateTone(state)} dot={dot}>
      {state}
    </Badge>
  )
}

// ClassBadge renders the release class of a version.
export function ClassBadge({ value }: { value: string }) {
  return <Badge tone={value === 'breaking' ? 'danger' : value === 'compatible' ? 'success' : 'info'}>{value}</Badge>
}

// ModeBadge renders a rule mode with increasing emphasis from observe to enforce.
export function ModeBadge({ mode }: { mode: string }) {
  return <Badge tone={mode === 'enforce' ? 'danger' : mode === 'shadow' ? 'warning' : 'neutral'}>{mode}</Badge>
}

const marks: { key: keyof VersionRecord; label: string; color: string }[] = [
  { key: 'published_at', label: 'Published', color: 'bg-info' },
  { key: 'new_content_from', label: 'New content from', color: 'bg-success' },
  { key: 'enforced_from', label: 'Enforced from', color: 'bg-warning' },
  { key: 'revoked_from', label: 'Revoked from', color: 'bg-danger' },
]

// Timeline draws one lane per version across the common time range, with an optional "now" marker.
export function Timeline({ records, now = Date.now(), selected, onSelect }: { records: VersionRecord[]; now?: number; selected?: string | null; onSelect?: (version: string) => void }) {
  if (records.length === 0) return null
  const times = records.flatMap((r) => marks.map((m) => r[m.key]).filter(Boolean).map((t) => new Date(t as string).getTime()))
  const lo = Math.min(...times, now)
  const hi = Math.max(...times, now)
  const pad = Math.max((hi - lo) * 0.06, 3_600_000)
  const from = lo - pad
  const span = hi + pad - from
  const x = (t: number) => `${((t - from) / span) * 100}%`
  return (
    <div className="space-y-2">
      <div className="relative">
        <div className="pointer-events-none absolute inset-y-0 z-10 w-px bg-accent" style={{ left: `calc(64px + (100% - 64px) * ${(now - from) / span})` }}>
          <span className="absolute -top-0.5 left-1 text-2xs font-medium whitespace-nowrap text-accent dark:text-info">now</span>
        </div>
        <div className="space-y-1 pt-4">
          {records.map((r) => {
            const start = new Date(r.published_at).getTime()
            const live = new Date(r.new_content_from).getTime()
            const end = r.revoked_from ? new Date(r.revoked_from).getTime() : hi + pad
            return (
              <button
                key={r.version}
                type="button"
                onClick={() => onSelect?.(r.version)}
                className={cn('flex h-6 w-full items-center rounded-sm text-left', selected === r.version ? 'bg-accent-soft' : 'hover:bg-hover')}
              >
                <span className="w-16 shrink-0 pl-1.5 font-mono text-xs font-medium">{r.version}</span>
                <span className="relative h-full flex-1">
                  <span className="absolute top-1/2 h-px -translate-y-1/2 border-t border-dashed border-line-strong" style={{ left: x(start), width: `calc(${x(live)} - ${x(start)})` }} />
                  <span className={cn('absolute top-1/2 h-1.5 -translate-y-1/2 rounded-full', r.revoked_from ? 'bg-danger/35' : 'bg-success/35')} style={{ left: x(live), width: `calc(${x(end)} - ${x(live)})` }} />
                  {marks.map((m) =>
                    r[m.key] ? (
                      <Tooltip key={m.key} side="top" content={<span><b>{m.label}</b> · {formatInstant(r[m.key] as string)} ({relativeTo(r[m.key] as string, now)})</span>}>
                        <span className={cn('absolute top-1/2 size-2 -translate-x-1/2 -translate-y-1/2 rounded-full ring-2 ring-bg', m.color)} style={{ left: x(new Date(r[m.key] as string).getTime()) }} />
                      </Tooltip>
                    ) : null,
                  )}
                </span>
              </button>
            )
          })}
        </div>
      </div>
      <div className="flex flex-wrap gap-x-4 gap-y-1 pl-16 text-xs text-muted">
        {marks.map((m) => (
          <span key={m.key} className="inline-flex items-center gap-1.5">
            <span className={cn('size-2 rounded-full', m.color)} />
            {m.label}
          </span>
        ))}
      </div>
    </div>
  )
}
