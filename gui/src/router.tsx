// Route tree of the console (TanStack Router, code-based): hub and client workspaces under the shell, plus the OAuth callback.
// Only the workspaces offered by the runtime are registered; the index redirects into the first available one.
import { createRootRoute, createRoute, createRouter, Outlet, redirect } from '@tanstack/react-router'
import { runtime } from '@/lib/runtime'
import { getSettings } from '@/lib/settings'
import { AppShell } from '@/components/shell/app-shell'
import { Empty } from '@/components/ui/misc'
import { AuthCallbackPage } from '@/routes/auth-callback'
import { AgreementsPage } from '@/routes/client/agreements'
import { AuditPage } from '@/routes/client/audit'
import { ExplainPage } from '@/routes/client/explain'
import { MetricsPage } from '@/routes/client/metrics'
import { RulesPage } from '@/routes/client/rules'
import { StatusPage } from '@/routes/client/status'
import { ClientVersionsPage } from '@/routes/client/versions'
import { AuthoringPage } from '@/routes/hub/authoring'
import { HubEventsPage, NamespaceEventsPage } from '@/routes/hub/events'
import { NamespacesPage } from '@/routes/hub/namespaces'
import { OverviewPage } from '@/routes/hub/overview'
import { PubPage } from '@/routes/hub/pub'
import { VersionsPage } from '@/routes/hub/versions'
import { VocabulariesPage } from '@/routes/hub/vocabularies'

const root = createRootRoute({
  component: Outlet,
  notFoundComponent: () => <Empty title="Not found">This view does not exist.</Empty>,
})

const shell = createRoute({ getParentRoute: () => root, id: 'shell', component: AppShell })

const index = createRoute({
  getParentRoute: () => shell,
  path: '/',
  beforeLoad: () => {
    const ns = getSettings().namespace
    throw redirect({ to: !runtime.hasHub ? '/client/status' : ns ? `/hub/n/${ns}` : '/hub/namespaces' })
  },
})

// page declares one routed view under the shell.
const page = (path: string, component: () => React.ReactNode) => createRoute({ getParentRoute: () => shell, path, component })

const routeTree = root.addChildren([
  createRoute({ getParentRoute: () => root, path: '/auth/callback', component: AuthCallbackPage }),
  shell.addChildren([
    index,
    ...(runtime.hasHub ? hubPages() : []),
    ...(runtime.hasClient ? clientPages() : []),
  ]),
])

// hubPages declares the server workspace routes.
function hubPages() {
  return [
    page('/hub/namespaces', NamespacesPage),
    page('/hub/events', HubEventsPage),
    page('/hub/pub', PubPage),
    page('/hub/n/$ns', OverviewPage),
    page('/hub/n/$ns/vocabularies', VocabulariesPage),
    page('/hub/n/$ns/authoring', AuthoringPage),
    page('/hub/n/$ns/versions', VersionsPage),
    page('/hub/n/$ns/events', NamespaceEventsPage),
  ]
}

// clientPages declares the client workspace routes.
function clientPages() {
  return [
    page('/client/status', StatusPage),
    page('/client/versions', ClientVersionsPage),
    page('/client/rules', RulesPage),
    page('/client/explain', ExplainPage),
    page('/client/agreements', AgreementsPage),
    page('/client/audit', AuditPage),
    page('/client/metrics', MetricsPage),
  ]
}

// router is the application router instance.
export const router = createRouter({ routeTree, defaultPreload: 'intent', basepath: runtime.base })
