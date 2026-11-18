// Publication explorer: interactive resource tree plus GET inspector for the public, immutable /pub tree of the hub.
// Lets operators browse namespaces, releases, package files, vocabularies and system artefacts without memorizing URLs.
import { useEffect, useMemo, useRef, useState } from 'react'
import { useLocation } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { CornerDownRight, ExternalLink, FileBadge, FileCode2, Globe, Hash, PanelLeftClose, PanelLeftOpen, SendHorizontal } from 'lucide-react'
import { cn } from '@/lib/cn'
import { formatBytes, prettyJSON } from '@/lib/format'
import { hasOpenDialog, isEditableTarget, useWindowKeydown } from '@/lib/hotkeys'
import { asApiError } from '@/lib/http'
import { hubQueries } from '@/lib/hub'
import { runtime } from '@/lib/runtime'
import { useSettings } from '@/lib/settings'
import { CodeEditor, guessLanguage } from '@/components/code-editor'
import { Accordion, AccordionItem } from '@/components/dsl-odrl-form'
import { ProblemBanner } from '@/components/problem'
import { Button, IconButton } from '@/components/ui/button'
import { Field, Select } from '@/components/ui/form'
import { Badge, Empty, Method, Mono, Page, Spinner, Tabs } from '@/components/ui/misc'

// niToHex converts an ni:///sha-256;<base64url> digest into the lowercase hex string used by /pub/blobs/sha256/{hex}.
function niToHex(digest: string): string {
  const seg = digest.replace(/^ni:\/\/\/sha-256;/, '')
  if (!seg || seg === digest) return ''
  try {
    const pad = seg.replace(/-/g, '+').replace(/_/g, '/') + '==='.slice((seg.length + 3) % 4)
    const raw = atob(pad)
    return Array.from(raw, (c) => c.charCodeAt(0).toString(16).padStart(2, '0')).join('')
  } catch {
    return ''
  }
}

// PubSectionId identifies one collapsible group in the Publication sidebar.
type PubSectionId = 'releases' | 'vocabs' | 'system'

// PubPage renders the interactive resource navigator on the left and the HTTP response viewer on the right.
export function PubPage() {
  const { namespace: activeNs } = useSettings()
  const loc = useLocation()
  const requested = new URLSearchParams(loc.searchStr).get('path')
  const pathRef = useRef<HTMLInputElement>(null)

  const [treeOpen, setTreeOpen] = useState(true)

  useWindowKeydown((e) => {
    if (hasOpenDialog() || isEditableTarget(e.target) || e.metaKey || e.ctrlKey || e.altKey) return
    if (e.key === '[') {
      e.preventDefault()
      setTreeOpen((o) => !o)
    } else if (e.key === '/') {
      e.preventDefault()
      pathRef.current?.focus()
      pathRef.current?.select()
    }
  })
  const [openSec, setOpenSec] = useState<Record<PubSectionId, boolean>>({
    releases: true,
    vocabs: false,
    system: true,
  })
  const [selectedNs, setSelectedNs] = useState<string>(activeNs ?? '')
  const [selectedVer, setSelectedVer] = useState<string>('')
  const [draft, setDraft] = useState(requested || '/pub/_vhub/v1/context.jsonld')
  const [path, setPath] = useState<string | null>(requested || '/pub/_vhub/v1/context.jsonld')
  const [tab, setTab] = useState<'body' | 'preview' | 'headers'>('body')

  const namespacesQ = useQuery(hubQueries.namespaces())
  const namespaces = namespacesQ.data ?? []

  useEffect(() => {
    if (!selectedNs && namespaces.length > 0) {
      const preferred = activeNs && namespaces.some((n) => n.slug === activeNs) ? activeNs : namespaces[0].slug
      setSelectedNs(preferred)
    }
  }, [selectedNs, activeNs, namespaces])

  useEffect(() => {
    if (!requested) return
    const clean = requested.startsWith('/') ? requested : `/${requested}`
    setDraft(clean)
    setPath(clean)
    const seg = clean.replace(/^\/pub\//, '').split('/')
    if (seg[0] && !['_vhub', 'packages', 'blobs'].includes(seg[0])) {
      setSelectedNs(seg[0])
      if (seg[1] && seg[1] !== 'vocab') setSelectedVer(seg[1])
    }
  }, [requested])

  const listingQ = useQuery({ ...hubQueries.listing(selectedNs), enabled: !!selectedNs })
  const versions = useMemo(() => listingQ.data?.credentialSubject.versions ?? [], [listingQ.data])
  const headRecord = useMemo(() => versions.filter((v) => !v.revoked_from).at(-1) ?? versions.at(-1) ?? null, [versions])

  useEffect(() => {
    if (!versions.length) {
      setSelectedVer('')
      return
    }
    if (!selectedVer || !versions.some((v) => v.version === selectedVer)) {
      setSelectedVer(headRecord?.version ?? versions[0].version)
    }
  }, [versions, selectedVer, headRecord])

  const activeVersionRecord = versions.find((v) => v.version === selectedVer) ?? headRecord
  const manifestDigest = activeVersionRecord?.manifest ?? ''
  const manifestSeg = manifestDigest.replace(/^ni:\/\/\/sha-256;/, '')
  const manifestQ = useQuery({ ...hubQueries.manifest(manifestDigest), enabled: !!manifestDigest })
  const manifestFiles = manifestQ.data?.credentialSubject.files ?? []

  const vocabsQ = useQuery({ ...hubQueries.vocabularies(selectedNs), enabled: !!selectedNs })
  const vocabs = vocabsQ.data ?? []

  const q = useQuery({ ...hubQueries.pub(path ?? ''), enabled: !!path })

  const send = (p = draft) => {
    const clean = p.startsWith('/') ? p : `/${p}`
    setDraft(clean)
    if (clean === path) void q.refetch()
    else setPath(clean)
  }

  const res = q.data
  const finalPath = res ? new URL(res.url).pathname.slice(runtime.hubBase.length) : null
  const body = res ? (typeof res.data === 'string' ? res.text : prettyJSON(res.data)) : ''
  const type = res?.headers.get('Content-Type') ?? ''
  const isHtml = type.includes('text/html')
  const headers = res ? [...res.headers.entries()] : []

  useEffect(() => {
    if (!isHtml && tab === 'preview') setTab('body')
    if (isHtml && tab === 'body') setTab('preview')
  }, [isHtml, path]) // eslint-disable-line react-hooks/exhaustive-deps

  const toggleSec = (id: PubSectionId) => setOpenSec((prev) => ({ ...prev, [id]: !prev[id] }))

  return (
    <Page flush title="Publication" subtitle="Browse and dereference public /pub artefacts: head redirects, signed manifests, versioned files, vocabularies and SHA-256 blobs.">
      <div className="flex h-full min-h-0 flex-col">
        <form className="flex shrink-0 items-center gap-2 border-b px-4 py-2.5" onSubmit={(e) => { e.preventDefault(); send() }}>
          <IconButton size="sm" label={treeOpen ? 'Hide resource browser ([)' : 'Show resource browser ([)'} onClick={() => setTreeOpen((o) => !o)}>
            {treeOpen ? <PanelLeftClose className="size-3.5" /> : <PanelLeftOpen className="size-3.5" />}
          </IconButton>
          <div className="flex h-8 min-w-0 flex-1 items-center rounded-md border border-line-strong bg-input focus-within:border-accent focus-within:ring-1 focus-within:ring-accent">
            <span className="flex h-full items-center border-r px-2.5"><Method value="GET" /></span>
            <span className="pl-2.5 font-mono text-xs text-subtle">{new URL(runtime.hubTarget).origin}</span>
            <input
              ref={pathRef}
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => e.key === 'Escape' && e.currentTarget.blur()}
              spellCheck={false}
              className="h-full min-w-0 flex-1 bg-transparent pr-2 font-mono text-xs focus:outline-none"
              aria-label="Path"
            />
          </div>
          <Button type="submit" variant="primary" className="h-8 px-4" icon={<SendHorizontal className="size-3.5" />} loading={q.isFetching}>Send</Button>
          {path ? (
            <Button
              variant="secondary"
              className="h-8"
              icon={<ExternalLink className="size-3.5" />}
              onClick={() => window.open(`${runtime.hubBase}${path}`, '_blank', 'noopener,noreferrer')}
            >
              Raw
            </Button>
          ) : null}
        </form>

        <div className="flex min-h-0 flex-1">
          {treeOpen ? (
            <aside className="flex w-80 shrink-0 flex-col border-r bg-side/20">
              <div className="border-b p-3">
                <Field label="Namespace">
                  <Select
                    className="[&>select]:font-mono [&>select]:text-xs"
                    value={selectedNs}
                    disabled={namespaces.length === 0}
                    onChange={(e) => {
                      setSelectedNs(e.target.value)
                      setSelectedVer('')
                    }}
                  >
                    {namespaces.length === 0 ? <option value="">No namespaces</option> : null}
                    {namespaces.map((n) => (
                      <option key={n.slug} value={n.slug}>
                        {n.slug}{n.title ? ` — ${n.title}` : ''}
                      </option>
                    ))}
                  </Select>
                </Field>
              </div>

              <div className="min-h-0 flex-1 overflow-y-auto p-3">
                <Accordion>
                  <AccordionItem
                    title="Releases &amp; files"
                    summary={activeVersionRecord ? `v${activeVersionRecord.version}` : 'none'}
                    open={openSec.releases}
                    onToggle={() => toggleSec('releases')}
                  >
                    {!selectedNs ? (
                      <p className="text-xs text-subtle">Select a namespace above.</p>
                    ) : listingQ.isPending ? (
                      <Spinner />
                    ) : versions.length === 0 ? (
                      <p className="text-xs text-subtle">No versions published in {selectedNs} yet.</p>
                    ) : (
                      <div className="space-y-2.5">
                        <button
                          type="button"
                          onClick={() => send(`/pub/${selectedNs}`)}
                          className={cn(
                            'flex w-full items-center justify-between rounded-md border px-2.5 py-1.5 text-left text-xs transition-colors',
                            path === `/pub/${selectedNs}` ? 'border-accent/50 bg-accent-soft text-accent' : 'bg-raised hover:bg-hover',
                          )}
                        >
                          <span className="font-mono font-medium">/pub/{selectedNs}</span>
                          <Badge tone="info">303 head</Badge>
                        </button>

                        <Field label="Version">
                          <Select
                            className="[&>select]:font-mono [&>select]:text-xs"
                            value={activeVersionRecord?.version ?? ''}
                            onChange={(e) => setSelectedVer(e.target.value)}
                          >
                            {[...versions].reverse().map((v) => (
                              <option key={v.version} value={v.version}>
                                {v.version} ({v.revoked_from ? 'revoked' : v.version === headRecord?.version ? 'head' : v.class})
                              </option>
                            ))}
                          </Select>
                        </Field>

                        {manifestSeg ? (
                          <button
                            type="button"
                            onClick={() => send(`/pub/packages/${manifestSeg}/manifest.json`)}
                            className={cn(
                              'flex w-full items-center gap-2 rounded-md border px-2.5 py-1.5 text-left text-xs transition-colors',
                              path === `/pub/packages/${manifestSeg}/manifest.json`
                                ? 'border-accent/50 bg-accent-soft text-accent'
                                : 'bg-raised hover:bg-hover',
                            )}
                          >
                            <FileBadge className="size-3.5 shrink-0 text-subtle" />
                            <span className="min-w-0 flex-1 truncate font-mono font-medium">manifest.json</span>
                            <Badge>VC</Badge>
                          </button>
                        ) : null}

                        {manifestQ.isPending ? (
                          <Spinner />
                        ) : manifestFiles.length > 0 ? (
                          <ul className="divide-y rounded-md border bg-raised">
                            {manifestFiles.map((f) => {
                              const fileUrl = `/pub/${selectedNs}/${activeVersionRecord?.version}/${f.path}`
                              const hex = niToHex(f.digest)
                              const blobUrl = hex ? `/pub/blobs/sha256/${hex}` : ''
                              const active = path === fileUrl || (blobUrl !== '' && path === blobUrl)
                              return (
                                <li key={f.path} className={cn('flex items-center gap-1 px-2 py-1.5', active ? 'bg-accent-soft/60' : 'hover:bg-hover/60')}>
                                  <button
                                    type="button"
                                    onClick={() => send(fileUrl)}
                                    className="min-w-0 flex-1 text-left"
                                    title={f.path}
                                  >
                                    <div className="flex items-center gap-1.5">
                                      <FileCode2 className={cn('size-3.5 shrink-0', active ? 'text-accent' : 'text-subtle')} />
                                      <span className={cn('truncate font-mono text-xs', active ? 'font-semibold text-accent' : 'text-fg')}>
                                        {f.path}
                                      </span>
                                    </div>
                                    <div className="mt-0.5 flex items-center gap-1.5 pl-5 text-2xs text-subtle">
                                      <Badge>{f.role}</Badge>
                                      {f.slot ? <span>{f.slot}</span> : null}
                                    </div>
                                  </button>
                                  {blobUrl ? (
                                    <button
                                      type="button"
                                      title={`Dereference content-addressed blob (${blobUrl})`}
                                      onClick={() => send(blobUrl)}
                                      className={cn(
                                        'inline-flex h-5 shrink-0 items-center gap-0.5 rounded-sm border px-1 font-mono text-2xs transition-colors',
                                        path === blobUrl
                                          ? 'border-accent/50 bg-accent-soft text-accent'
                                          : 'border-line-strong bg-bg text-subtle hover:text-fg',
                                      )}
                                    >
                                      <Hash className="size-2.5" />
                                      blob
                                    </button>
                                  ) : null}
                                </li>
                              )
                            })}
                          </ul>
                        ) : null}
                      </div>
                    )}
                  </AccordionItem>

                  <AccordionItem
                    title="Vocabularies"
                    summary={`${vocabs.length}`}
                    open={openSec.vocabs}
                    onToggle={() => toggleSec('vocabs')}
                  >
                    {vocabsQ.isPending ? (
                      <Spinner />
                    ) : vocabs.length === 0 ? (
                      <p className="text-xs text-subtle">No vocabularies registered in {selectedNs || 'this namespace'}.</p>
                    ) : (
                      <ul className="divide-y rounded-md border bg-raised">
                        {vocabs.map((v) => {
                          const latestUrl = `/pub/${selectedNs}/vocab/${v.name}`
                          return (
                            <li key={v.name} className="space-y-1.5 p-2">
                              <button
                                type="button"
                                onClick={() => send(latestUrl)}
                                className={cn(
                                  'flex w-full items-center justify-between rounded-sm px-1.5 py-1 text-left text-xs',
                                  path === latestUrl ? 'bg-accent-soft font-medium text-accent' : 'hover:bg-hover',
                                )}
                              >
                                <span className="truncate font-mono">{v.name}</span>
                                <Badge tone="info">303 latest</Badge>
                              </button>
                              {v.revisions.length > 0 ? (
                                <div className="flex flex-wrap gap-1 px-1">
                                  {v.revisions.map((r) => {
                                    const revUrl = `/pub/${selectedNs}/vocab/${v.name}/r/${r.n}`
                                    return (
                                      <button
                                        key={r.n}
                                        type="button"
                                        onClick={() => send(revUrl)}
                                        className={cn(
                                          'h-5 rounded-sm border px-1.5 font-mono text-2xs transition-colors',
                                          path === revUrl
                                            ? 'border-accent/50 bg-accent-soft font-medium text-accent'
                                            : 'border-line-strong bg-bg text-muted hover:text-fg',
                                        )}
                                      >
                                        r{r.n}
                                      </button>
                                    )
                                  })}
                                </div>
                              ) : null}
                            </li>
                          )
                        })}
                      </ul>
                    )}
                  </AccordionItem>

                  <AccordionItem
                    title="Hub system (_vhub)"
                    summary="2 files"
                    open={openSec.system}
                    onToggle={() => toggleSec('system')}
                  >
                    <ul className="divide-y rounded-md border bg-raised">
                      {(
                        [
                          ['JSON-LD context', '/pub/_vhub/v1/context.jsonld', 'application/ld+json'],
                          ['Hub vocabulary', '/pub/_vhub/v1/vocab.ttl', 'text/turtle'],
                        ] as const
                      ).map(([label, sysUrl, mime]) => {
                        const active = path === sysUrl
                        return (
                          <li key={sysUrl}>
                            <button
                              type="button"
                              onClick={() => send(sysUrl)}
                              className={cn(
                                'flex w-full flex-col gap-0.5 px-2.5 py-2 text-left transition-colors',
                                active ? 'bg-accent-soft/60' : 'hover:bg-hover/60',
                              )}
                            >
                              <span className={cn('text-xs font-medium', active ? 'text-accent' : 'text-fg')}>{label}</span>
                              <span className="truncate font-mono text-2xs text-subtle">{mime}</span>
                            </button>
                          </li>
                        )
                      })}
                    </ul>
                  </AccordionItem>
                </Accordion>
              </div>
            </aside>
          ) : null}

          <div className="min-w-0 flex-1">
            {!path ? (
              <Empty icon={<Globe />} title="Select a published artefact">Choose any release file, manifest, vocabulary revision or system document on the left.</Empty>
            ) : q.isPending ? (
              <div className="flex h-40 items-center justify-center"><Spinner /></div>
            ) : q.isError ? (
              <div className="p-4">
                <div className="mb-2 flex items-center gap-2"><Badge tone="danger">{asApiError(q.error).status || 'ERR'}</Badge><Mono>{path}</Mono></div>
                <ProblemBanner error={q.error} />
              </div>
            ) : res ? (
              <div className="flex h-full flex-col">
                <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-b px-4 py-2 text-xs">
                  <Badge tone="success" dot>{res.status} OK</Badge>
                  <span className="text-muted">Type <span className="font-mono text-fg">{type.split(';')[0]}</span></span>
                  <span className="text-muted">Size <span className="tnum text-fg">{formatBytes(new Blob([res.text]).size)}</span></span>
                  {res.etag ? <span className="flex items-center gap-1 text-muted">ETag <Mono copy={res.etag.replace(/"/g, '')} className="text-fg">{res.etag.replace(/"/g, '').slice(0, 16)}…</Mono></span> : null}
                  <span className={cn('text-muted', res.headers.get('Cache-Control')?.includes('immutable') && 'text-success')}>{res.headers.get('Cache-Control')}</span>
                  {finalPath && finalPath !== path ? (
                    <span className="flex w-full items-center gap-1 text-muted"><CornerDownRight className="size-3" /> redirected to <Mono copy={finalPath} className="text-fg">{finalPath}</Mono></span>
                  ) : null}
                </div>
                <Tabs
                  className="px-4"
                  value={tab}
                  onChange={setTab}
                  tabs={[
                    ...(isHtml ? [{ id: 'preview' as const, label: 'Preview' }] : []),
                    { id: 'body' as const, label: isHtml ? 'HTML Source' : 'Body' },
                    { id: 'headers' as const, label: 'Headers', count: headers.length },
                  ]}
                />
                <div className="min-h-0 flex-1 p-3">
                  {tab === 'preview' && isHtml ? (
                    <iframe title={path} sandbox="" srcDoc={res.text} className="h-full w-full rounded-md border bg-white" />
                  ) : tab === 'body' ? (
                    <CodeEditor readOnly copy value={body} language={guessLanguage(body, type)} className="h-full" />
                  ) : (
                    <div className="h-full overflow-auto">
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
                    </div>
                  )}
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </Page>
  )
}
