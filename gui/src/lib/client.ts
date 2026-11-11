// Typed client of the alexandria-client (C-CLI) admin listener: status, versions, rules, explain, agreements, audit and metrics.
// Calls go through the client base (the /api/client proxy, or the admin listener itself when embedded); the admin API is unauthenticated and bound to its own listener.
import { queryOptions } from '@tanstack/react-query'
import { ApiError, request } from '@/lib/http'
import type { VersionRecord } from '@/lib/hub'
import { runtime } from '@/lib/runtime'

const BASE = runtime.clientBase

// ClientState is the operational state of the client.
export type ClientState = 'Onboarding' | 'Active' | 'Transition' | 'Degraded' | 'FailClosed'

// Status is the GET /status document.
export interface Status {
  namespace: string
  state: ClientState
  versions: { version: string; manifest: string; local_role: 'head' | 'in_force' | 'scheduled' | 'retained' | string }[]
  last_accepted_listing: { sequence: number; valid_from: string; valid_until: string } | null
  hub_did: { did: string; resolved_at: string | null }
  rules: { revision: number; count: number }
  clock_skew_estimate_ms: number
}

// ClientVersion is one entry of GET /versions.
export interface ClientVersion extends VersionRecord {
  state: string
  retained: boolean
}

// Mode is what a rule does with a classification.
export type Mode = 'observe' | 'shadow' | 'enforce'

// Rule is one entry of the dynamic rule set (Appendix A.6).
export interface Rule {
  id: string
  description?: string
  priority: number
  listener: 'dsp-ingress' | 'mgmt-ingress' | 'dsp-egress' | string
  match: { method: string; path: string; body?: { pointer: string; [k: string]: unknown }[] }
  message: string
  verify: { select?: string; gates: string[] }
  mode?: Mode
  validate_response?: { gate: string; mode?: Mode }
}

// Ruleset is the rule set with its revision.
export interface Ruleset {
  revision: number
  rules: Rule[]
}

// GateExplanation is the classification of one gate in an explanation.
export interface GateExplanation {
  gate: string
  mode: string
  evaluated: boolean
  classification?: string
  codes: string[] | null
  matched: string | null
  sunset?: string
  action: string
  findings: { slot: string; source: string; severity: string; focus?: string; path?: string; message: string }[] | null
}

// Explanation is the side-effect-free decision of POST /rules:explain.
export interface Explanation {
  rule: string | null
  rules_revision: number
  state: string
  gates: GateExplanation[] | null
  action: string
  vhub_verdict?: string
  code?: string
}

// ExplainRequest is the body of POST /rules:explain.
export interface ExplainRequest {
  listener: string
  method: string
  path: string
  body: unknown
}

// Agreement is a captured agreement binding with its usability.
export interface Agreement {
  agreement_id: string
  provider_pid: string
  consumer_pid: string
  role: string
  version: string
  manifest: string
  declared_profile: string | null
  recorded_at: string
  source: string
  usable: boolean
}

// AuditReport is the latest passive catalog audit.
export interface AuditReport {
  generated_at: string
  version: string
  entries: { response: number; at: string; classification: string; code: string }[] | null
}

// Health summarizes the liveness and readiness probes.
export interface Health {
  live: boolean
  ready: boolean
}

// probe reports whether an endpoint answers 2xx.
async function probe(path: string): Promise<boolean> {
  try {
    await request(BASE + path)
    return true
  } catch (e) {
    if (e instanceof ApiError && e.status === 0) throw e
    return false
  }
}

// etagOf renders the If-Match token of a rule set revision.
export const etagOf = (revision: number) => `"${revision}"`

// client groups the mutating admin operations.
export const client = {
  addRule: (rule: string, revision: number) => request<Ruleset>(`${BASE}/rules`, { method: 'POST', raw: rule, ifMatch: etagOf(revision) }),
  updateRule: (id: string, rule: string, revision: number) =>
    request<Ruleset>(`${BASE}/rules/${encodeURIComponent(id)}`, { method: 'PUT', raw: rule, ifMatch: etagOf(revision) }),
  deleteRule: (id: string, revision: number) => request(`${BASE}/rules/${encodeURIComponent(id)}`, { method: 'DELETE', ifMatch: etagOf(revision) }),
  replaceRules: (ruleset: string, revision: number) => request<Ruleset>(`${BASE}/rules`, { method: 'PUT', raw: ruleset, ifMatch: etagOf(revision) }),
  explain: (body: ExplainRequest) => request<Explanation>(`${BASE}/rules:explain`, { method: 'POST', body }),
  agreement: (id: string) => request<Agreement>(`${BASE}/agreements/${id}`),
}

// clientQueries holds the query option factories of the client workspace.
export const clientQueries = {
  status: () =>
    queryOptions({ queryKey: ['client', 'status'], queryFn: async () => (await request<Status>(`${BASE}/status`)).data, refetchInterval: 5000, retry: false }),
  health: () =>
    queryOptions({
      queryKey: ['client', 'health'],
      queryFn: async (): Promise<Health> => ({ live: await probe('/healthz'), ready: await probe('/readyz') }),
      refetchInterval: 5000,
      retry: false,
    }),
  versions: () => queryOptions({ queryKey: ['client', 'versions'], queryFn: async () => (await request<ClientVersion[]>(`${BASE}/versions`)).data }),
  rules: () => queryOptions({ queryKey: ['client', 'rules'], queryFn: async () => (await request<Ruleset>(`${BASE}/rules`)).data }),
  audit: () =>
    queryOptions({
      queryKey: ['client', 'audit'],
      queryFn: async (): Promise<AuditReport | null> => {
        try {
          return (await request<AuditReport>(`${BASE}/audit/reports/latest`)).data
        } catch (e) {
          if (e instanceof ApiError && e.status === 404) return null
          throw e
        }
      },
      retry: false,
    }),
  metrics: () =>
    queryOptions({ queryKey: ['client', 'metrics'], queryFn: async () => (await request<string>(`${BASE}/metrics`)).text, refetchInterval: 5000, retry: false }),
}
