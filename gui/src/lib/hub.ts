// Typed client of alexandria-server (C-HUB): the /v1 management API and the immutable /pub publication tree.
// Calls go through the hub base (the /api/hub proxy, or the hub itself when embedded); query option factories centralize keys for TanStack Query.
import { queryOptions } from '@tanstack/react-query'
import { request, type ApiResponse } from '@/lib/http'
import { runtime } from '@/lib/runtime'

const BASE = runtime.hubBase

// Graph is a compact JSON-LD @graph list document.
interface Graph<T> {
  '@graph': T[]
}

// Namespace is a governance namespace of the hub.
export interface Namespace {
  id: string
  slug: string
  title?: string
  description?: string
  created_at: string
}

// Steward is a principal allowed to govern a namespace.
export interface Steward {
  id: string
  iss: string
  sub: string
  label?: string
  assigned_at: string
  assigned_by: string
}

// Revision is one immutable revision of a vocabulary.
export interface Revision {
  id: string
  n: number
  digest: string
  size: number
  created_at: string
  created_by: string
}

// Vocabulary is a hosted or proxied vocabulary with its revisions.
export interface Vocabulary {
  id: string
  name: string
  kind: 'hosted' | 'proxy'
  media_type: string
  upstream?: string
  title?: string
  description?: string
  revisions: Revision[]
}

// VersionRecord is the lifecycle record of a published version.
export interface VersionRecord {
  id: string
  namespace: string
  version: string
  manifest: string
  class: 'initial' | 'compatible' | 'breaking' | string
  published_at: string
  new_content_from: string
  enforced_from: string
  revoked_from: string | null
}

// Proof is the Data Integrity proof of a credential.
export interface Proof {
  type: string
  cryptosuite: string
  created: string
  verificationMethod: string
  proofPurpose: string
  proofValue: string
}

// Credential is a W3C VC 2.0 envelope signed by the hub.
export interface Credential<S> {
  '@context': string[]
  type: string[]
  issuer: string
  validFrom: string
  validUntil?: string
  credentialSubject: S
  proof: Proof
}

// ListingSubject is the subject of a VersionListing credential.
export interface ListingSubject {
  id: string
  namespace: string
  sequence: number
  versions: VersionRecord[]
}

// Draft is an editable release proposal with its optimistic-concurrency ETag.
export interface Draft {
  id: string
  namespace: string
  rev: number
  document?: unknown
}

// Finding is one coded validation finding.
export interface Finding {
  code?: string
  detail?: string
  message?: string
  title?: string
  severity?: string
  path?: string
  line?: number
  [k: string]: unknown
}

// ValidationReport is the result of draft :validate.
export interface ValidationReport {
  valid: boolean
  class?: string
  findings: Finding[] | null
}

// Candidate is a frozen proposal awaiting approval or rejection.
export interface Candidate {
  id: string
  namespace: string
  state: 'open' | 'approved' | 'rejected'
  version: string
  class: string
  diff: string[] | null
  planned: VersionRecord
  proposed_at: string
  lifecycle: Record<string, unknown>
}

// HubEvent is one entry of the append-only governance log.
export interface HubEvent {
  id: string
  type: string
  ts: string
  principal: string
  subject: string
  namespace?: string
}

// VocabularyInput is the body that registers a vocabulary.
export interface VocabularyInput {
  name: string
  kind: 'hosted' | 'proxy'
  media_type: string
  upstream?: string
  title?: string
  description?: string
}

// ns builds the path of a namespace-scoped resource.
const ns = (slug: string, rest = '') => `${BASE}/v1/namespaces/${encodeURIComponent(slug)}${rest}`

// graph unwraps a @graph list response.
async function graph<T>(url: string, auth = false): Promise<T[]> {
  return (await request<Graph<T>>(url, { auth })).data['@graph'] ?? []
}

// hubPath maps an absolute hub IRI to its same-origin proxied path.
export function hubPath(iri: string): string {
  try {
    const u = new URL(iri)
    return BASE + u.pathname + u.search
  } catch {
    return BASE + (iri.startsWith('/') ? iri : `/${iri}`)
  }
}

// allEvents pages through the event log until it is exhausted or a safety cap is reached.
async function allEvents(slug?: string): Promise<HubEvent[]> {
  const out: HubEvent[] = []
  let after = ''
  for (let page = 0; page < 20; page++) {
    const url = `${slug ? ns(slug, '/events') : `${BASE}/v1/events`}?limit=500${after ? `&after=${after}` : ''}`
    const batch = await graph<HubEvent>(url, true)
    out.push(...batch)
    if (batch.length < 500) break
    after = batch[batch.length - 1].id.split('/').pop() ?? ''
  }
  return out
}

// hub groups every server operation used by the console.
export const hub = {
  createNamespace: (body: { slug: string; title?: string; description?: string }) =>
    request<Namespace>(`${BASE}/v1/namespaces`, { method: 'POST', body, auth: true }),
  assignSteward: (slug: string, body: { iss: string; sub: string; label?: string }) =>
    request<Steward>(ns(slug, '/stewards'), { method: 'POST', body, auth: true }),
  removeSteward: (slug: string, id: string) => request(ns(slug, `/stewards/${encodeURIComponent(id)}`), { method: 'DELETE', auth: true }),
  createVocabulary: (slug: string, body: VocabularyInput) => request<Vocabulary>(ns(slug, '/vocabularies'), { method: 'POST', body, auth: true }),
  refreshVocabulary: (slug: string, name: string) => request<Revision>(ns(slug, `/vocabularies/${name}:refresh`), { method: 'POST', auth: true }),
  putContent: (slug: string, name: string, content: string, mediaType: string, ifMatch?: string) =>
    request<Revision>(ns(slug, `/vocabularies/${name}/content`), { method: 'PUT', raw: content, contentType: mediaType, ifMatch, auth: true }),
  createDraft: (slug: string, doc: string) => request<Draft>(ns(slug, '/drafts'), { method: 'POST', raw: doc, auth: true }),
  getDraft: (slug: string, id: string) => request<Draft>(ns(slug, `/drafts/${id}`), { auth: true }),
  replaceDraft: (slug: string, id: string, doc: string, ifMatch: string) =>
    request<Draft>(ns(slug, `/drafts/${id}`), { method: 'PUT', raw: doc, ifMatch, auth: true }),
  validateDraft: (slug: string, id: string) => request<ValidationReport>(ns(slug, `/drafts/${id}:validate`), { method: 'POST', auth: true }),
  proposeDraft: (slug: string, id: string) => request<Candidate>(ns(slug, `/drafts/${id}:propose`), { method: 'POST', auth: true }),
  approveCandidate: (slug: string, id: string, graceWindow?: string) =>
    request<Credential<VersionRecord>>(ns(slug, `/candidates/${id}:approve`), { method: 'POST', body: graceWindow ? { grace_window: graceWindow } : {}, auth: true }),
  rejectCandidate: (slug: string, id: string, comment: string) =>
    request<Candidate>(ns(slug, `/candidates/${id}:reject`), { method: 'POST', body: { comment }, auth: true }),
  revokeVersion: (slug: string, version: string, body: { justification: string; revoked_from?: string }) =>
    request<Credential<VersionRecord>>(ns(slug, `/versions/${version}:revoke`), { method: 'POST', body, auth: true }),
  fetchPub: (path: string): Promise<ApiResponse<unknown>> => request(BASE + path),
}

// hubQueries holds the query option factories of the server workspace.
export const hubQueries = {
  namespaces: () => queryOptions({ queryKey: ['hub', 'namespaces'], queryFn: () => graph<Namespace>(`${BASE}/v1/namespaces`) }),
  namespace: (slug: string) => queryOptions({ queryKey: ['hub', 'ns', slug], queryFn: async () => (await request<Namespace>(ns(slug))).data }),
  stewards: (slug: string) => queryOptions({ queryKey: ['hub', 'ns', slug, 'stewards'], queryFn: () => graph<Steward>(ns(slug, '/stewards'), true) }),
  vocabularies: (slug: string) =>
    queryOptions({ queryKey: ['hub', 'ns', slug, 'vocabularies'], queryFn: () => graph<Vocabulary>(ns(slug, '/vocabularies')) }),
  vocabulary: (slug: string, name: string) =>
    queryOptions({ queryKey: ['hub', 'ns', slug, 'vocabularies', name], queryFn: async () => (await request<Vocabulary>(ns(slug, `/vocabularies/${name}`))).data }),
  revisionContent: (iri: string) =>
    queryOptions({ queryKey: ['hub', 'pub', iri], queryFn: async () => (await request(hubPath(iri))).text, staleTime: Infinity }),
  listing: (slug: string) =>
    queryOptions({
      queryKey: ['hub', 'ns', slug, 'versions'],
      queryFn: async () => (await request<Credential<ListingSubject>>(ns(slug, '/versions'))).data,
      retry: false,
    }),
  record: (slug: string, version: string) =>
    queryOptions({
      queryKey: ['hub', 'ns', slug, 'versions', version],
      queryFn: async () => (await request<Credential<VersionRecord>>(ns(slug, `/versions/${version}`))).data,
    }),
  events: (slug?: string) =>
    queryOptions({ queryKey: slug ? ['hub', 'ns', slug, 'events'] : ['hub', 'events'], queryFn: () => allEvents(slug), retry: false }),
  draft: (slug: string, id: string) =>
    queryOptions({
      queryKey: ['hub', 'ns', slug, 'drafts', id],
      queryFn: async () => {
        const r = await hub.getDraft(slug, id)
        return { draft: r.data, etag: r.etag ?? '' }
      },
    }),
  candidate: (slug: string, id: string) =>
    queryOptions({
      queryKey: ['hub', 'ns', slug, 'candidates', id],
      queryFn: async () => (await request<Candidate>(ns(slug, `/candidates/${id}`), { auth: true })).data,
    }),
  pub: (path: string) =>
    queryOptions({ queryKey: ['hub', 'pubpath', path], queryFn: () => hub.fetchPub(path), retry: false, staleTime: 30_000 }),
}
