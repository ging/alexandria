// Publication explorer: a Postman-style GET console for the public, immutable /pub tree of the hub.
// It follows head and vocabulary redirects and shows status, caching headers, ETag and the artefact body.
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { CornerDownRight, Globe, SendHorizontal } from 'lucide-react'
import { cn } from '@/lib/cn'
import { formatBytes, prettyJSON } from '@/lib/format'
import { asApiError } from '@/lib/http'
import { hubQueries } from '@/lib/hub'
import { runtime } from '@/lib/runtime'
import { useSettings } from '@/lib/settings'
import { CodeEditor, guessLanguage } from '@/components/code-editor'
import { ProblemBanner } from '@/components/problem'
import { Button } from '@/components/ui/button'
import { Badge, Empty, Method, Mono, Page, Spinner, Tabs } from '@/components/ui/misc'

// PubPage renders the request bar, shortcuts and the response viewer.
export function PubPage() {
  const { namespace } = useSettings()
  const [draft, setDraft] = useState('/pub/_vhub/v1/context.jsonld')
  const [path, setPath] = useState<string | null>(null)
  const [tab, setTab] = useState<'body' | 'headers'>('body')
  const q = useQuery({ ...hubQueries.pub(path ?? ''), enabled: !!path })
  const send = (p = draft) => {
    const clean = p.startsWith('/') ? p : `/${p}`
    setDraft(clean)
    if (clean === path) void q.refetch()
    else setPath(clean)
  }
  const ns = namespace ?? '{ns}'
  const shortcuts: [string, string][] = [
    ['JSON-LD context', '/pub/_vhub/v1/context.jsonld'],
    ['Vocabulary (Turtle)', '/pub/_vhub/v1/vocab.ttl'],
    ...(namespace ? ([['Head manifest', `/pub/${ns}`]] as [string, string][]) : []),
  ]
  const res = q.data
  const finalPath = res ? new URL(res.url).pathname.slice(runtime.hubBase.length) : null
  const body = res ? (typeof res.data === 'string' ? res.text : prettyJSON(res.data)) : ''
  const type = res?.headers.get('Content-Type') ?? ''
  const headers = res ? [...res.headers.entries()] : []
  return (
    <Page flush title="Publication" subtitle="Public, cacheable artefacts: head redirects, package manifests, versioned files, vocabulary revisions and content-addressed blobs.">
      <div className="flex h-full flex-col">
        <form className="flex shrink-0 items-center gap-2 border-b px-6 py-3" onSubmit={(e) => { e.preventDefault(); send() }}>
          <div className="flex h-8 min-w-0 flex-1 items-center rounded-md border border-line-strong bg-input focus-within:border-accent focus-within:ring-1 focus-within:ring-accent">
            <span className="flex h-full items-center border-r px-2.5"><Method value="GET" /></span>
            <span className="pl-2.5 font-mono text-xs text-subtle">{new URL(runtime.hubTarget).origin}</span>
            <input value={draft} onChange={(e) => setDraft(e.target.value)} spellCheck={false} className="h-full min-w-0 flex-1 bg-transparent pr-2 font-mono text-xs focus:outline-none" aria-label="Path" />
          </div>
          <Button type="submit" variant="primary" className="h-8 px-4" icon={<SendHorizontal className="size-3.5" />} loading={q.isFetching}>Send</Button>
        </form>
        <div className="flex shrink-0 flex-wrap items-center gap-1.5 border-b bg-side px-6 py-2">
          <span className="label-caps mr-1">Shortcuts</span>
          {shortcuts.map(([label, p]) => (
            <button key={p} onClick={() => send(p)} className="h-5 rounded-sm border bg-bg px-1.5 text-xs text-muted hover:border-accent/60 hover:text-fg">{label}</button>
          ))}
          <span className="ml-2 font-mono text-2xs text-subtle">/pub/{ns}/vocab/{'{name}'} · /pub/{ns}/{'{version}'}/{'{file}'} · /pub/packages/{'{digest}'}/manifest.json · /pub/blobs/sha256/{'{hex}'}</span>
        </div>
        <div className="min-h-0 flex-1">
          {!path ? (
            <Empty icon={<Globe />} title="Dereference a /pub URL">Pick a shortcut or type a path. Redirects (303) are followed and reported.</Empty>
          ) : q.isPending ? (
            <div className="flex h-40 items-center justify-center"><Spinner /></div>
          ) : q.isError ? (
            <div className="p-4">
              <div className="mb-2 flex items-center gap-2"><Badge tone="danger">{asApiError(q.error).status || 'ERR'}</Badge><Mono>{path}</Mono></div>
              <ProblemBanner error={q.error} />
            </div>
          ) : res ? (
            <div className="flex h-full flex-col">
              <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-b px-6 py-2 text-xs">
                <Badge tone="success" dot>{res.status} OK</Badge>
                <span className="text-muted">Type <span className="font-mono text-fg">{type.split(';')[0]}</span></span>
                <span className="text-muted">Size <span className="tnum text-fg">{formatBytes(new Blob([res.text]).size)}</span></span>
                {res.etag ? <span className="flex items-center gap-1 text-muted">ETag <Mono copy={res.etag.replace(/"/g, '')} className="text-fg">{res.etag.replace(/"/g, '').slice(0, 16)}…</Mono></span> : null}
                <span className={cn('text-muted', res.headers.get('Cache-Control')?.includes('immutable') && 'text-success')}>{res.headers.get('Cache-Control')}</span>
                {finalPath && finalPath !== path ? (
                  <span className="flex w-full items-center gap-1 text-muted"><CornerDownRight className="size-3" /> redirected to <Mono copy={finalPath} className="text-fg">{finalPath}</Mono></span>
                ) : null}
              </div>
              <Tabs value={tab} onChange={setTab} tabs={[{ id: 'body', label: 'Body' }, { id: 'headers', label: 'Headers', count: headers.length }]} />
              <div className="min-h-0 flex-1 p-3">
                {tab === 'body' ? (
                  <CodeEditor readOnly copy value={body} language={guessLanguage(body, type)} className="h-full" />
                ) : (
                  <table className="w-full text-xs">
                    <tbody>
                      {headers.map(([k, v]) => (
                        <tr key={k} className="border-b">
                          <td className="w-48 py-1.5 pr-4 font-mono text-muted">{k}</td>
                          <td className="py-1.5 font-mono break-all">{v}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </Page>
  )
}
