// Formatting helpers for instants, durations, digests and IRIs shown across the console.
// All functions are pure and tolerate missing values so tables can render partial data.

const dateFmt = new Intl.DateTimeFormat('en-GB', { year: 'numeric', month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', timeZone: 'UTC', hour12: false })
const dayFmt = new Intl.DateTimeFormat('en-GB', { year: 'numeric', month: 'short', day: '2-digit', timeZone: 'UTC' })

// formatInstant renders an RFC 3339 instant in UTC, or an em dash when absent.
export function formatInstant(ts?: string | null): string {
  if (!ts) return '—'
  const d = new Date(ts)
  return Number.isNaN(d.getTime()) ? ts : `${dateFmt.format(d)} UTC`
}

// formatDay renders only the UTC calendar day of an instant.
export function formatDay(ts?: string | null): string {
  if (!ts) return '—'
  const d = new Date(ts)
  return Number.isNaN(d.getTime()) ? ts : dayFmt.format(d)
}

// formatSpan renders a millisecond span as its two most significant units.
export function formatSpan(ms: number): string {
  const s = Math.round(Math.abs(ms) / 1000)
  if (s < 60) return `${s}s`
  const units: [number, string][] = [[86400, 'd'], [3600, 'h'], [60, 'm']]
  const parts: string[] = []
  let rest = s
  for (const [size, label] of units) {
    const n = Math.floor(rest / size)
    if (n > 0) parts.push(`${n}${label}`)
    rest -= n * size
    if (parts.length === 2) break
  }
  return parts.join(' ')
}

// relativeTo renders an instant relative to a reference time ("in 2d 3h", "5m ago").
export function relativeTo(ts: string | null | undefined, now: number = Date.now()): string {
  if (!ts) return '—'
  const t = new Date(ts).getTime()
  if (Number.isNaN(t)) return ts
  const diff = t - now
  if (Math.abs(diff) < 1000) return 'now'
  return diff > 0 ? `in ${formatSpan(diff)}` : `${formatSpan(diff)} ago`
}

// shortDigest abbreviates an ni:///sha-256 digest or a hex hash for dense tables.
export function shortDigest(d?: string | null, n = 10): string {
  if (!d) return '—'
  const v = d.replace(/^ni:\/\/\/sha-256;/, '')
  return v.length > n + 4 ? `${v.slice(0, n)}…${v.slice(-4)}` : v
}

// iriTail returns the last path segment of an IRI.
export function iriTail(iri?: string | null): string {
  if (!iri) return ''
  return iri.replace(/\/+$/, '').split('/').pop() ?? ''
}

// formatBytes renders a byte count with binary units.
export function formatBytes(n?: number | null): string {
  if (n == null) return '—'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`
  return `${(n / 1024 / 1024).toFixed(1)} MiB`
}

// prettyJSON indents a value or a JSON string, returning the input unchanged when it is not JSON.
export function prettyJSON(v: unknown): string {
  if (typeof v === 'string') {
    try {
      return JSON.stringify(JSON.parse(v), null, 2)
    } catch {
      return v
    }
  }
  return JSON.stringify(v, null, 2)
}
