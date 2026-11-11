// Application frame: title bar, activity bar, contextual sidebar, routed content and the VS Code style status bar.
// It owns global UI state (command palette, identity dialog) and derives the active workspace and namespace from the URL.
import { useEffect, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, Outlet, useLocation, useNavigate } from '@tanstack/react-router'
import { Check, ChevronsUpDown, Monitor, Moon, Search, Sun } from 'lucide-react'
import { cn } from '@/lib/cn'
import { clientQueries } from '@/lib/client'
import { asApiError } from '@/lib/http'
import { hubQueries } from '@/lib/hub'
import { runtime } from '@/lib/runtime'
import { applyTheme, setSettings, useActiveIdentity, useSettings, type Theme } from '@/lib/settings'
import { CommandPalette } from '@/components/shell/command-palette'
import { IdentityDialog, IdentityMenu } from '@/components/shell/identity'
import { clientNav, hubGlobalNav, namespaceNav, workspaces, type NavItem } from '@/components/shell/nav'
import { stateTone } from '@/components/lifecycle'
import { IconButton } from '@/components/ui/button'
import { Dot, Kbd } from '@/components/ui/misc'
import { Menu, MenuItem, MenuLabel, Tooltip } from '@/components/ui/overlay'

// AppShell renders the persistent chrome around every routed view.
export function AppShell() {
  const { pathname } = useLocation()
  const settings = useSettings()
  const [palette, setPalette] = useState(false)
  const [identity, setIdentity] = useState(false)
  const workspace = !runtime.hasHub || pathname.startsWith('/client') ? 'client' : 'hub'
  const urlNs = /^\/hub\/n\/([^/]+)/.exec(pathname)?.[1]
  const namespace = urlNs ? decodeURIComponent(urlNs) : settings.namespace

  useEffect(() => applyTheme(settings.theme), [settings.theme])
  useEffect(() => {
    if (urlNs && decodeURIComponent(urlNs) !== settings.namespace) setSettings({ namespace: decodeURIComponent(urlNs) })
  }, [urlNs, settings.namespace])
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPalette((o) => !o)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="flex h-full flex-col bg-bg text-fg">
      <header className="grid h-9 shrink-0 grid-cols-[1fr_auto_1fr] items-center gap-3 border-b bg-side px-2">
        <div className="flex items-center gap-2 pl-1">
          <img src={`${runtime.base}favicon.svg`} alt="" className="size-4" />
          <span className="text-sm font-semibold tracking-tight">Alexandria</span>
          <span className="text-xs text-subtle">Vocabulary Hub console</span>
        </div>
        <button onClick={() => setPalette(true)} className="flex h-6 w-[min(40vw,420px)] items-center gap-2 rounded-md border border-line-strong bg-bg px-2 text-xs text-subtle hover:border-accent/60 hover:text-muted">
          <Search className="size-3.5" />
          <span className="flex-1 text-left">Search views, namespaces and commands</span>
          <Kbd>⌘K</Kbd>
        </button>
        <div className="flex items-center justify-end gap-1.5">
          {runtime.hasHub ? <IdentityMenu onAdd={() => setIdentity(true)} /> : null}
          <ThemeToggle theme={settings.theme} />
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <nav aria-label="Workspaces" className="flex w-11 shrink-0 flex-col items-center gap-0.5 border-r bg-side py-1.5">
          {workspaces.filter((w) => (w.id === 'hub' ? runtime.hasHub : runtime.hasClient)).map((w) => (
            <Tooltip key={w.id} content={w.label} side="right">
              <Link
                to={w.to}
                aria-label={w.label}
                className={cn(
                  'relative flex size-9 items-center justify-center text-subtle transition-colors hover:text-fg',
                  workspace === w.id && 'text-fg before:absolute before:top-1 before:bottom-1 before:left-[-4px] before:w-0.5 before:rounded-full before:bg-accent',
                )}
              >
                <w.icon className="size-[18px] stroke-[1.6]" />
              </Link>
            </Tooltip>
          ))}
        </nav>

        <aside className="flex w-56 shrink-0 flex-col border-r bg-side">
          {workspace === 'hub' ? <HubSidebar namespace={namespace} pathname={pathname} /> : <ClientSidebar pathname={pathname} />}
        </aside>

        <main className="min-w-0 flex-1 bg-bg">
          <Outlet />
        </main>
      </div>

      <StatusBar namespace={namespace} onIdentity={() => setIdentity(true)} />
      <CommandPalette open={palette} onOpenChange={setPalette} namespace={namespace} onIdentity={() => setIdentity(true)} />
      <IdentityDialog open={identity} onOpenChange={setIdentity} />
    </div>
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
        <IconButton label="Color theme" size="sm">
          <Icon className="size-3.5" />
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
  return <p className="label-caps px-3 pt-3 pb-1">{children}</p>
}

// NavList renders sidebar links with the active one highlighted.
function NavList({ items, pathname }: { items: NavItem[]; pathname: string }) {
  return (
    <ul className="px-1.5">
      {items.map((n) => {
        const active = n.exact ? pathname === n.to || pathname === `${n.to}/` : pathname.startsWith(n.to)
        return (
          <li key={n.to}>
            <Link
              to={n.to}
              className={cn('flex h-7 items-center gap-2 rounded-md px-2 text-sm transition-colors', active ? 'bg-active font-medium text-fg' : 'text-muted hover:bg-hover hover:text-fg')}
            >
              <n.icon className={cn('size-3.5 shrink-0', active && 'text-accent dark:text-info')} />
              <span className="truncate">{n.label}</span>
            </Link>
          </li>
        )
      })}
    </ul>
  )
}

// HubSidebar renders hub-wide links and the namespace switcher with its scoped links.
function HubSidebar({ namespace, pathname }: { namespace: string | null; pathname: string }) {
  const navigate = useNavigate()
  const namespaces = useQuery(hubQueries.namespaces())
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto pb-3">
      <SidebarTitle>Server · C-HUB</SidebarTitle>
      <NavList items={hubGlobalNav} pathname={pathname} />
      <SidebarTitle>Namespace</SidebarTitle>
      <div className="px-3 pb-1.5">
        <Menu
          trigger={
            <button className="flex h-7 w-full items-center gap-2 rounded-md border border-line-strong bg-bg px-2 text-sm hover:bg-hover">
              <span className={cn('flex-1 truncate text-left font-mono text-xs', !namespace && 'font-sans text-subtle')}>{namespace ?? 'Select namespace'}</span>
              <ChevronsUpDown className="size-3 text-subtle" />
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
      {namespace ? <NavList items={namespaceNav(namespace)} pathname={pathname} /> : <p className="px-3 text-xs text-subtle">Pick a namespace to manage its vocabularies, releases and events.</p>}
    </div>
  )
}

// ClientSidebar renders the client workspace links.
function ClientSidebar({ pathname }: { pathname: string }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto pb-3">
      <SidebarTitle>Client · C-CLI</SidebarTitle>
      <NavList items={clientNav} pathname={pathname} />
    </div>
  )
}

// StatusBar shows live connectivity of both binaries, the client state and the active principal.
function StatusBar({ namespace, onIdentity }: { namespace: string | null; onIdentity: () => void }) {
  const hubProbe = useQuery({ ...hubQueries.namespaces(), refetchInterval: 15_000, enabled: runtime.hasHub })
  const status = useQuery({ ...clientQueries.status(), enabled: runtime.hasClient })
  const identity = useActiveIdentity()
  const hubDown = hubProbe.isError && asApiError(hubProbe.error).status === 0
  const cell = 'flex h-full items-center gap-1.5 px-2 hover:bg-white/15'
  return (
    <footer className="tnum flex h-[22px] shrink-0 items-center justify-between bg-statusbar text-xs text-statusbar-fg">
      <div className="flex h-full items-center">
        {runtime.hasHub ? (
        <Link to="/hub/namespaces" className={cell} title={`alexandria-server at ${runtime.hubTarget}`}>
          <span className={cn('size-1.5 rounded-full', hubProbe.isPending ? 'bg-white/50' : hubDown ? 'bg-[#ffb4a8]' : 'bg-[#9ff0b6]')} />
          hub {new URL(runtime.hubTarget).host}
          {hubDown ? ' · offline' : ''}
        </Link>
        ) : null}
        {runtime.hasClient ? (
        <Link to="/client/status" className={cell} title={`alexandria-client admin at ${runtime.clientTarget}`}>
          <span className={cn('size-1.5 rounded-full', status.isPending ? 'bg-white/50' : status.isError ? 'bg-[#ffb4a8]' : 'bg-[#9ff0b6]')} />
          client {new URL(runtime.clientTarget).host}
          {status.data ? ` · ${status.data.state}` : status.isError ? ' · offline' : ''}
        </Link>
        ) : null}
        {status.data ? (
          <span className={cn(cell, 'hover:bg-transparent')}>
            <Dot tone={stateTone(status.data.state)} />
            rules r{status.data.rules.revision}
          </span>
        ) : null}
      </div>
      <div className="flex h-full items-center">
        {runtime.hasHub && namespace ? <span className={cn(cell, 'font-mono hover:bg-transparent')}>ns:{namespace}</span> : null}
        {runtime.hasHub ? (
          <button className={cell} onClick={onIdentity}>
            {identity ? `${identity.label}${identity.admin ? ' (admin)' : ''}${identity.kind === 'oidc' ? ' · oidc' : ''}` : 'anonymous'}
          </button>
        ) : (
          <span className={cn(cell, 'hover:bg-transparent')}>admin listener · unauthenticated</span>
        )}
      </div>
    </footer>
  )
}
