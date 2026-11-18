// Application frame: clean top bar, single contextual sidebar, global keyboard navigation and routed content canvas.
// Owns global UI state (command palette, shortcut help, identity dialog) and derives the active workspace and namespace from the URL.
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, Outlet, useLocation, useNavigate } from '@tanstack/react-router'
import { Check, ChevronsUpDown, Keyboard, Monitor, Moon, Search, Sun } from 'lucide-react'
import { cn } from '@/lib/cn'
import { shortDigest } from '@/lib/format'
import { hasOpenDialog, isEditableTarget, useWindowKeydown } from '@/lib/hotkeys'
import { hubQueries } from '@/lib/hub'
import { runtime } from '@/lib/runtime'
import { applyTheme, setSettings, useSettings, type Theme } from '@/lib/settings'
import { CommandPalette } from '@/components/shell/command-palette'
import { IdentityDialog, IdentityMenu } from '@/components/shell/identity'
import { clientNav, hubGlobalNav, namespaceNav, workspaces, type NavItem } from '@/components/shell/nav'
import { IconButton } from '@/components/ui/button'
import { CopyButton, Kbd } from '@/components/ui/misc'
import { Dialog, Menu, MenuItem, MenuLabel } from '@/components/ui/overlay'

// AppShell renders the persistent chrome around every routed view.
export function AppShell() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const settings = useSettings()
  const [palette, setPalette] = useState(false)
  const [shortcutsOpen, setShortcutsOpen] = useState(false)
  const [identity, setIdentity] = useState(false)
  const [chordPending, setChordPending] = useState(false)
  const chordTimer = useRef<number | null>(null)

  const workspace = !runtime.hasHub || pathname.startsWith('/client') ? 'client' : 'hub'
  const urlNs = /^\/hub\/n\/([^/]+)/.exec(pathname)?.[1]
  const namespace = urlNs ? decodeURIComponent(urlNs) : settings.namespace
  const bothWorkspaces = runtime.hasHub && runtime.hasClient

  useEffect(() => applyTheme(settings.theme), [settings.theme])
  useEffect(() => {
    if (urlNs && decodeURIComponent(urlNs) !== settings.namespace) setSettings({ namespace: decodeURIComponent(urlNs) })
  }, [urlNs, settings.namespace])

  const clearChord = () => {
    if (chordTimer.current) {
      window.clearTimeout(chordTimer.current)
      chordTimer.current = null
    }
    setChordPending(false)
  }

  useWindowKeydown((e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      clearChord()
      setShortcutsOpen(false)
      setPalette((o) => !o)
      return
    }
    if (hasOpenDialog() || isEditableTarget(e.target)) {
      if (chordPending) clearChord()
      return
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return

    if (e.key === '?') {
      e.preventDefault()
      clearChord()
      setShortcutsOpen((o) => !o)
      return
    }

    if (chordPending) {
      e.preventDefault()
      clearChord()
      const k = e.key.toLowerCase()
      const ns = namespace || 'default'
      const dest: Record<string, string | undefined> = {
        ...(runtime.hasHub && runtime.hasClient
          ? { w: workspace === 'client' ? (namespace ? `/hub/n/${ns}` : '/hub/namespaces') : '/client/status' }
          : {}),
        ...(runtime.hasHub
          ? {
              n: '/hub/namespaces',
              p: '/hub/pub',
              o: `/hub/n/${ns}`,
              v: `/hub/n/${ns}/vocabularies`,
              a: `/hub/n/${ns}/authoring`,
              l: `/hub/n/${ns}/versions`,
              e: workspace === 'hub' && namespace ? `/hub/n/${ns}/events` : '/hub/events',
            }
          : {}),
        ...(runtime.hasClient
          ? {
              s: '/client/status',
              c: '/client/versions',
              u: '/client/rules',
              x: '/client/explain',
              g: '/client/agreements',
              t: '/client/audit',
              m: '/client/metrics',
            }
          : {}),
      }
      const target = dest[k]
      if (target) void navigate({ to: target })
      return
    }

    if (e.key === 'g') {
      e.preventDefault()
      setChordPending(true)
      chordTimer.current = window.setTimeout(() => {
        chordTimer.current = null
        setChordPending(false)
      }, 1200)
    }
  })

  return (
    <div className="flex h-full flex-col bg-bg text-fg">
      <header className="flex h-12 shrink-0 items-center justify-between gap-4 border-b bg-bg px-4">
        <div className="flex items-center gap-3">
          <Link to="/" className="flex items-center gap-2.5">
            <img src={`${runtime.base}favicon.svg`} alt="" className="size-6 rounded-md" />
            <span className="text-base font-semibold tracking-tight">Alexandria</span>
          </Link>
          <span className="h-4 w-px bg-line" />
          {bothWorkspaces ? (
            <nav aria-label="Workspaces" className="flex items-center gap-0.5 rounded-lg border bg-side p-0.5">
              {workspaces.map((w) => {
                const active = workspace === w.id
                return (
                  <Link
                    key={w.id}
                    to={w.to}
                    title={w.hint}
                    className={cn(
                      'flex h-7 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium transition-all',
                      active ? 'bg-bg text-fg shadow-2xs ring-1 ring-line' : 'text-muted hover:text-fg',
                    )}
                  >
                    <w.icon className={cn('size-3.5', active ? 'text-accent' : 'text-subtle')} />
                    {w.label}
                  </Link>
                )
              })}
            </nav>
          ) : (
            <span className="text-xs font-medium text-subtle">{runtime.hasHub ? 'Vocabulary Hub' : 'Edge Proxy'}</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setPalette(true)}
            title="Command palette (⌘K)"
            className="hidden h-7 items-center gap-2 rounded-md border border-line-strong bg-side px-2.5 text-xs text-muted transition-colors hover:bg-hover hover:text-fg sm:inline-flex"
          >
            <Search className="size-3.5 text-subtle" />
            <span>Go to…</span>
            <Kbd>⌘K</Kbd>
          </button>
          {runtime.hasHub ? <IdentityMenu onAdd={() => setIdentity(true)} /> : null}
          <IconButton label="Keyboard shortcuts (?)" size="md" onClick={() => setShortcutsOpen(true)}>
            <Keyboard className="size-4" />
          </IconButton>
          <ThemeToggle theme={settings.theme} />
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <aside className="flex w-60 shrink-0 flex-col border-r bg-side">
          {workspace === 'hub' ? <HubSidebar namespace={namespace} pathname={pathname} /> : <ClientSidebar pathname={pathname} />}
        </aside>

        <main className="min-w-0 flex-1 bg-bg">
          <Outlet />
        </main>
      </div>

      {chordPending ? (
        <div className="fixed bottom-4 left-4 z-50 flex animate-pop items-center gap-2 rounded-md border border-line-strong bg-raised px-3 py-1.5 text-xs text-fg shadow-pop">
          <Kbd>g</Kbd>
          <span className="text-muted">then</span>
          {runtime.hasHub ? (
            <span className="font-mono text-2xs text-subtle">[o] Overview · [a] Authoring · [v] Vocabs · [l] Releases · [p] Pub</span>
          ) : null}
          {runtime.hasClient ? (
            <span className="font-mono text-2xs text-subtle">[s] Status · [u] Rules · [x] Explain · [c] Versions</span>
          ) : null}
        </div>
      ) : null}

      <CommandPalette
        open={palette}
        onOpenChange={setPalette}
        namespace={namespace}
        onIdentity={() => setIdentity(true)}
        onShortcuts={() => setShortcutsOpen(true)}
      />
      <KeyboardShortcutsDialog open={shortcutsOpen} onOpenChange={setShortcutsOpen} />
      <IdentityDialog open={identity} onOpenChange={setIdentity} />
    </div>
  )
}

// KeyboardShortcutsDialog lists every global and per-view shortcut in a scannable reference modal.
function KeyboardShortcutsDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const groups: { title: string; items: [string, string][] }[] = [
    {
      title: 'Global',
      items: [
        ['⌘K / Ctrl+K', 'Open command palette'],
        ['?', 'Toggle this shortcuts guide'],
        ['/', 'Focus table filter or /pub URL bar'],
        ['Esc', 'Close active drawer, dialog or filter'],
        ...(runtime.hasHub && runtime.hasClient ? [['g  w', 'Switch Server ↔ Client workspace'] as [string, string]] : []),
      ],
    },
    ...(runtime.hasHub
      ? [
          {
            title: 'Server navigation (press g, then…)',
            items: [
              ['g  n', 'Namespaces'],
              ['g  p', 'Publication (/pub)'],
              ['g  o', 'Namespace Overview'],
              ['g  v', 'Vocabularies'],
              ['g  a', 'Authoring'],
              ['g  l', 'Releases / Versions'],
              ['g  e', 'Events'],
            ] as [string, string][],
          },
        ]
      : []),
    ...(runtime.hasClient
      ? [
          {
            title: 'Client navigation (press g, then…)',
            items: [
              ['g  s', 'Status'],
              ['g  c', 'Client Versions'],
              ['g  u', 'Rules'],
              ['g  x', 'Explain'],
              ['g  g', 'Agreements'],
              ['g  t', 'Audit'],
              ['g  m', 'Metrics'],
            ] as [string, string][],
          },
        ]
      : []),
    {
      title: 'Tables & Workspaces',
      items: [
        ['↑ / ↓  (or j / k)', 'Move between table rows'],
        ['Enter', 'Open selected row details'],
        ['[', 'Toggle sidebar in Authoring / Publication'],
        ['n', 'New draft inline (Authoring)'],
        ['1 / 2', 'Switch ODRL Profile ↔ Space Profile'],
        ['f / s / y', 'Studio mode: Form / Split / YAML'],
        ['⌘S / Ctrl+S', 'Save draft'],
        ['⌘↵ / Ctrl+↵', 'Validate draft · Create draft · Run Explain'],
        ['r', 'Sync client with hub now (Status / Versions)'],
      ],
    },
  ]

  return (
    <Dialog open={open} onOpenChange={onOpenChange} width="max-w-2xl" title="Keyboard shortcuts" description="Single-key shortcuts stay disabled while typing in inputs or CodeMirror.">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {groups.map((g) => (
          <div key={g.title} className="space-y-1.5">
            <p className="label-caps">{g.title}</p>
            <dl className="divide-y rounded-md border bg-side/40">
              {g.items.map(([keys, label]) => (
                <div key={keys + label} className="flex items-center justify-between gap-2 px-2.5 py-1.5 text-xs">
                  <dt className="text-muted">{label}</dt>
                  <dd className="shrink-0 font-mono"><Kbd>{keys}</Kbd></dd>
                </div>
              ))}
            </dl>
          </div>
        ))}
      </div>
    </Dialog>
  )
}

// ThemeToggle switches between light, dark and system schemes.
function ThemeToggle({ theme }: { theme: Theme }) {
  const Icon = theme === 'light' ? Sun : theme === 'dark' ? Moon : Monitor
  const options: [Theme, string, typeof Sun][] = [['light', 'Light', Sun], ['dark', 'Dark', Moon], ['system', 'System', Monitor]]
  return (
    <Menu
      align="end"
      trigger={
        <IconButton label="Color theme" size="md">
          <Icon className="size-4" />
        </IconButton>
      }
    >
      <MenuLabel>Color theme</MenuLabel>
      {options.map(([id, label, I]) => (
        <MenuItem key={id} icon={<I className="size-3.5" />} onSelect={() => setSettings({ theme: id })} hint={theme === id ? <Check className="size-3" /> : undefined}>
          {label}
        </MenuItem>
      ))}
    </Menu>
  )
}

// SidebarTitle renders the caption of a sidebar group.
function SidebarTitle({ children }: { children: ReactNode }) {
  return <p className="label-caps px-3.5 pt-4 pb-1.5">{children}</p>
}

// NavList renders sidebar links with the active one highlighted by the warm acid accent.
function NavList({ items, pathname }: { items: NavItem[]; pathname: string }) {
  return (
    <ul className="space-y-0.5 px-2.5">
      {items.map((n) => {
        const active = n.exact ? pathname === n.to || pathname === `${n.to}/` : pathname.startsWith(n.to)
        return (
          <li key={n.to}>
            <Link
              to={n.to}
              className={cn(
                'group flex h-8 items-center gap-2.5 rounded-md px-2.5 text-sm transition-colors',
                active ? 'bg-accent-soft font-medium text-accent' : 'text-muted hover:bg-hover hover:text-fg',
              )}
            >
              <n.icon className={cn('size-4 shrink-0 transition-colors', active ? 'text-accent' : 'text-subtle group-hover:text-fg')} />
              <span className="truncate">{n.label}</span>
            </Link>
          </li>
        )
      })}
    </ul>
  )
}

// HubSidebar renders hub-wide links, the namespace switcher with its scoped links and the copyable hub DID.
function HubSidebar({ namespace, pathname }: { namespace: string | null; pathname: string }) {
  const navigate = useNavigate()
  const namespaces = useQuery(hubQueries.namespaces())
  const listing = useQuery({ ...hubQueries.listing(namespace ?? ''), enabled: !runtime.hubDid && !!namespace })
  const hubDid = runtime.hubDid || listing.data?.issuer || ''
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto pb-3">
      <SidebarTitle>Governance</SidebarTitle>
      <NavList items={hubGlobalNav} pathname={pathname} />
      <SidebarTitle>Namespace</SidebarTitle>
      <div className="px-2.5 pb-2">
        <Menu
          trigger={
            <button className="flex h-8 w-full items-center gap-2 rounded-md border border-line-strong bg-bg px-2.5 text-sm transition-colors hover:border-accent/50 hover:bg-hover">
              <span className={cn('flex-1 truncate text-left font-mono text-xs', !namespace && 'font-sans text-subtle')}>{namespace ?? 'Select namespace'}</span>
              <ChevronsUpDown className="size-3.5 text-subtle" />
            </button>
          }
        >
          <MenuLabel>Namespaces</MenuLabel>
          {namespaces.data?.length ? (
            namespaces.data.map((n) => (
              <MenuItem key={n.slug} icon={namespace === n.slug ? <Check className="size-3.5" /> : null} hint={n.title} onSelect={() => void navigate({ to: `/hub/n/${n.slug}` })}>
                <span className="font-mono text-xs">{n.slug}</span>
              </MenuItem>
            ))
          ) : (
            <p className="px-2 py-1.5 text-xs text-muted">{namespaces.isError ? 'Hub unreachable.' : 'No namespaces yet.'}</p>
          )}
        </Menu>
      </div>
      {namespace ? <NavList items={namespaceNav(namespace)} pathname={pathname} /> : <p className="px-3.5 text-xs leading-relaxed text-subtle">Pick a namespace to manage its vocabularies, releases and events.</p>}
      {hubDid ? (
        <div className="mt-auto border-t px-3.5 pt-3">
          <p className="label-caps mb-1">Hub DID</p>
          <div className="flex items-center justify-between gap-1.5 rounded-md border bg-bg px-2 py-1" title={hubDid}>
            <span className="truncate font-mono text-2xs text-muted">{shortDigest(hubDid, 24)}</span>
            <CopyButton value={hubDid} />
          </div>
        </div>
      ) : null}
    </div>
  )
}

// ClientSidebar renders the client workspace links.
function ClientSidebar({ pathname }: { pathname: string }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto pb-4">
      <SidebarTitle>Edge Proxy</SidebarTitle>
      <NavList items={clientNav} pathname={pathname} />
    </div>
  )
}
