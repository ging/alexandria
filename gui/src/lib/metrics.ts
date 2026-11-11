// OpenMetrics text parser for the three metric families exposed by alexandria-client (§4.11).
// It only understands the simple "name{labels} value" sample lines the client emits.

// Sample is one parsed metric sample.
export interface Sample {
  name: string
  labels: Record<string, string>
  value: number
}

// parseMetrics extracts every sample of an OpenMetrics exposition.
export function parseMetrics(text: string): Sample[] {
  const out: Sample[] = []
  for (const line of text.split('\n')) {
    if (!line || line.startsWith('#')) continue
    const m = /^([a-zA-Z_:][\w:]*)(?:\{(.*)\})?\s+(\S+)/.exec(line)
    if (!m) continue
    const labels: Record<string, string> = {}
    for (const l of (m[2] ?? '').matchAll(/(\w+)="((?:[^"\\]|\\.)*)"/g)) labels[l[1]] = l[2]
    out.push({ name: m[1], labels, value: Number(m[3]) })
  }
  return out
}
