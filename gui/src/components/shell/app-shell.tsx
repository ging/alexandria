// Application frame: clean top bar, single contextual sidebar and routed content canvas.
// It owns global UI state (command palette, identity dialog) and derives the active workspace and namespace from the URL.
import { useEffect, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, Outlet, useLocation, useNavigate } from '@tanstack/react-router'
import { Check, ChevronsUpDown, Monitor, Moon, Sun } from 'lucide-react'
import { cn } from '@/lib/cn'
import { hubQueries } from '@/lib/hub'
import { runtime } from '@/lib/runtime'
import { applyTheme, setSettings, useSettings, type Theme } from '@/lib/settings'
import { CommandPalette } from '@/components/shell/command-palette'
import { IdentityDialog, IdentityMenu } from '@/components/shell/identity'
import { clientNav, hubGlobalNav, namespaceNav, workspaces, type NavItem } from '@/components/shell/nav'
import { IconButton } from '@/components/ui/button'
import { Menu, MenuItem, MenuLabel } from '@/components/ui/overlay'

// AppShell renders the persistent chrome around every routed view.
export function AppShell() {
  const { pathname } = useLocation()
  const settings = useSettings()
  const [palette, setPalette] = useState(false)
  const [identity, setIdentity] = useState(false)
  const workspace = !runtime.hasHub || pathname.startsWith('/client') ? 'client' : 'hub'
  const urlNs = /^\/hub\/n\/([^/]+)/.exec(pathname)?.[1]
  const namespace = urlNs ? decodeURIComponent(urlNs) : settings.namespace
  const bothWorkspaces = runtime.hasHub && runtime.hasClient

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
          {runtime.hasHub ? <IdentityMenu onAdd={() => setIdentity(true)} /> : null}
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

// HubSidebar renders hub-wide links and the namespace switcher with its scoped links.
function HubSidebar({ namespace, pathname }: { namespace: string | null; pathname: string }) {
  const navigate = useNavigate()
  const namespaces = useQuery(hubQueries.namespaces())
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto pb-4">
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
