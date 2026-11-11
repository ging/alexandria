// Command palette (⌘K): keyboard-first navigation across both workspaces plus theme and identity commands.
// Built on cmdk inside the shared dialog chrome.
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Command } from 'cmdk'
import { Dialog as RDialog } from 'radix-ui'
import { Boxes, KeyRound, Monitor, Moon, Search, Sun } from 'lucide-react'
import type { ReactNode } from 'react'
import { hubQueries } from '@/lib/hub'
import { runtime } from '@/lib/runtime'
import { setSettings, type Theme } from '@/lib/settings'
import { clientNav, hubGlobalNav, namespaceNav } from '@/components/shell/nav'

// CommandPalette renders the searchable command list.
export function CommandPalette({ open, onOpenChange, namespace, onIdentity }: { open: boolean; onOpenChange: (o: boolean) => void; namespace: string | null; onIdentity: () => void }) {
  const navigate = useNavigate()
  const namespaces = useQuery({ ...hubQueries.namespaces(), enabled: open && runtime.hasHub })
  const go = (to: string) => {
    onOpenChange(false)
    void navigate({ to })
  }
  const theme = (t: Theme) => {
    setSettings({ theme: t })
    onOpenChange(false)
  }
  return (
    <RDialog.Root open={open} onOpenChange={onOpenChange}>
      <RDialog.Portal>
        <RDialog.Overlay className="fixed inset-0 z-40 animate-in bg-black/40" />
        <RDialog.Content className="fixed top-[14vh] left-1/2 z-50 w-[min(560px,calc(100vw-2rem))] -translate-x-1/2 animate-pop overflow-hidden rounded-lg border border-line-strong bg-raised shadow-pop focus:outline-none">
          <RDialog.Title className="sr-only">Command palette</RDialog.Title>
          <RDialog.Description className="sr-only">Search destinations and commands</RDialog.Description>
          <Command loop>
            <div className="flex items-center gap-2 border-b px-3">
              <Search className="size-3.5 text-subtle" />
              <Command.Input autoFocus placeholder="Go to a view, namespace or command…" className="h-10 flex-1 bg-transparent text-base placeholder:text-subtle focus:outline-none" />
            </div>
            <Command.List className="max-h-[52vh] overflow-y-auto p-1">
              <Command.Empty className="px-3 py-6 text-center text-sm text-muted">No matches.</Command.Empty>
              {runtime.hasHub ? (
              <Group heading="alexandria-server">
                {hubGlobalNav.map((n) => (
                  <Item key={n.to} icon={<n.icon className="size-3.5" />} onSelect={() => go(n.to)} hint={n.hint}>
                    {n.label}
                  </Item>
                ))}
                {namespace
                  ? namespaceNav(namespace).map((n) => (
                      <Item key={n.to} icon={<n.icon className="size-3.5" />} onSelect={() => go(n.to)} hint={namespace}>
                        {n.label}
                      </Item>
                    ))
                  : null}
              </Group>
              ) : null}
              {namespaces.data?.length ? (
                <Group heading="Namespaces">
                  {namespaces.data.map((n) => (
                    <Item key={n.slug} icon={<Boxes className="size-3.5" />} onSelect={() => go(`/hub/n/${n.slug}`)} hint={n.title}>
                      Open namespace {n.slug}
                    </Item>
                  ))}
                </Group>
              ) : null}
              {runtime.hasClient ? (
              <Group heading="alexandria-client">
                {clientNav.map((n) => (
                  <Item key={n.to} icon={<n.icon className="size-3.5" />} onSelect={() => go(n.to)} hint={n.hint}>
                    Client: {n.label}
                  </Item>
                ))}
              </Group>
              ) : null}
              <Group heading="Commands">
                {runtime.hasHub ? (
                  <Item icon={<KeyRound className="size-3.5" />} onSelect={() => { onOpenChange(false); onIdentity() }}>
                    Sign in with OIDC or add access token
                  </Item>
                ) : null}
                <Item icon={<Sun className="size-3.5" />} onSelect={() => theme('light')}>Theme: Light</Item>
                <Item icon={<Moon className="size-3.5" />} onSelect={() => theme('dark')}>Theme: Dark</Item>
                <Item icon={<Monitor className="size-3.5" />} onSelect={() => theme('system')}>Theme: System</Item>
              </Group>
            </Command.List>
          </Command>
        </RDialog.Content>
      </RDialog.Portal>
    </RDialog.Root>
  )
}

// Group renders a captioned command group.
function Group({ heading, children }: { heading: string; children: ReactNode }) {
  return (
    <Command.Group heading={heading} className="[&_[cmdk-group-heading]]:label-caps [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:pb-1">
      {children}
    </Command.Group>
  )
}

// Item renders one command row.
function Item({ children, icon, hint, onSelect }: { children: ReactNode; icon: ReactNode; hint?: string; onSelect: () => void }) {
  return (
    <Command.Item onSelect={onSelect} className="flex h-7 cursor-default items-center gap-2 rounded-sm px-2 text-sm text-fg data-[selected=true]:bg-accent data-[selected=true]:text-accent-fg">
      <span className="opacity-80">{icon}</span>
      <span className="flex-1 truncate">{children}</span>
      {hint ? <span className="truncate text-xs opacity-60">{hint}</span> : null}
    </Command.Item>
  )
}
