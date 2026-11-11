// Navigation model of the console: the two workspaces and their sidebar entries.
// Shared by the sidebar and the command palette so both stay in sync.
import { Activity, BookMarked, BookOpen, Boxes, ClipboardCheck, FileSignature, Gauge, GitBranch, Globe, Handshake, History, LayoutDashboard, ListTree, PencilRuler, ScrollText, Search, type LucideIcon } from 'lucide-react'

// NavItem is one sidebar destination.
export interface NavItem {
  label: string
  to: string
  icon: LucideIcon
  exact?: boolean
  hint?: string
}

// hubGlobalNav lists the hub-wide destinations.
export const hubGlobalNav: NavItem[] = [
  { label: 'Namespaces', to: '/hub/namespaces', icon: Boxes, hint: 'Governance spaces of the hub' },
  { label: 'Publication', to: '/hub/pub', icon: Globe, hint: 'Immutable /pub artefacts' },
  { label: 'Hub events', to: '/hub/events', icon: ScrollText, hint: 'Global audit log (admin)' },
]

// namespaceNav lists the destinations scoped to one namespace.
export const namespaceNav = (ns: string): NavItem[] => [
  { label: 'Overview', to: `/hub/n/${ns}`, icon: LayoutDashboard, exact: true },
  { label: 'Vocabularies', to: `/hub/n/${ns}/vocabularies`, icon: BookOpen },
  { label: 'Authoring', to: `/hub/n/${ns}/authoring`, icon: PencilRuler },
  { label: 'Versions', to: `/hub/n/${ns}/versions`, icon: GitBranch },
  { label: 'Events', to: `/hub/n/${ns}/events`, icon: History },
]

// clientNav lists the destinations of the client workspace.
export const clientNav: NavItem[] = [
  { label: 'Status', to: '/client/status', icon: Activity, hint: 'State machine and trust' },
  { label: 'Versions', to: '/client/versions', icon: ListTree, hint: 'Synchronized lifecycle records' },
  { label: 'Rules', to: '/client/rules', icon: FileSignature, hint: 'Dynamic rule set' },
  { label: 'Explain', to: '/client/explain', icon: Search, hint: 'Dry-run a DSP message' },
  { label: 'Agreements', to: '/client/agreements', icon: Handshake, hint: 'Captured agreement bindings' },
  { label: 'Audit', to: '/client/audit', icon: ClipboardCheck, hint: 'Passive catalog audit' },
  { label: 'Metrics', to: '/client/metrics', icon: Gauge, hint: 'OpenMetrics exposition' },
]

// workspaces lists the activity-bar entries.
export const workspaces = [
  { id: 'hub', label: 'alexandria-server', to: '/hub/namespaces', icon: BookMarked },
  { id: 'client', label: 'alexandria-client', to: '/client/status', icon: Activity },
] as const
