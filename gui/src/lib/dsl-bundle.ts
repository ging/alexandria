// Typed model, ODRL 2.2 vocabulary presets and bidirectional YAML serializer for alexandria.profiles/v1 bundles.
// Used by DslStudio so visual form edits and raw YAML edits stay synchronized without losing structure.
import YAML from 'yaml'

// LangMap maps BCP 47 language tags to localized text; "en" is required by the DSL schema.
export type LangMap = Record<string, string>

// DslSignature models the operator and operand constraints of a left_operand (§9).
export interface DslSignature {
  operators: string[]
  datatype?: string
  value_kind?: 'literal' | 'iri'
  values?: string[]
  scheme?: string
  allow_reference?: boolean
}

// DslEnforcement models optional Cedar PDP enforcement metadata on a left_operand (§9.5).
export interface DslEnforcement {
  cedar_path: string
  pdp_level?: string
  eval_timing?: 'once' | 'continuous'
}

// DslAdopted models one entry under adopted.<category> (§8.4).
export interface DslAdopted {
  term: string
  signature?: DslSignature
  enforcement?: DslEnforcement
}

// DslTerm models one profile-defined term in OdrlProfile.terms (§8.5).
export interface DslTerm {
  id: string
  kind: string
  label: LangMap
  definition: LangMap
  note?: LangMap
  included_in?: string
  subclass_of?: string
  signature?: DslSignature
  enforcement?: DslEnforcement
  status?: 'active' | 'deprecated' | 'retired'
  replaced_by?: string
  extra_turtle?: string
}

// OdrlProfileDoc models a kind: OdrlProfile document (§8).
export interface OdrlProfileDoc {
  dsl: 'alexandria.profiles/v1'
  kind: 'OdrlProfile'
  profile: {
    id: string
    version: string
    title: LangMap
    description?: LangMap
    publisher: string
    license: string
    base_url: string
  }
  namespaces: Record<string, string>
  adopted: Record<string, DslAdopted[]>
  terms: DslTerm[]
  rules: {
    actions: 'closed' | 'open'
    left_operands: 'closed' | 'open'
  }
}

// DslConformance pairs an artefact role with a validation gate (§11.3).
export interface DslConformance {
  role: string
  applies_to: string
}

// DslBinding binds one specification slot in SpaceProfile.bindings (§11.3).
export interface DslBinding {
  slot: string
  profile: string
  source: 'internal' | 'vendored'
  version: string
  obligation: 'mandatory' | 'recommended' | 'optional'
  via?: string[]
  conformance: DslConformance[]
}

// DslLifecycle models optional lifecycle overrides on SpaceProfile (§11.6).
export interface DslLifecycle {
  propagation_window?: string
  grace_window?: string
  sync_interval?: string
  clock_skew?: string
  lease_margin?: string
  max_concurrent_versions?: number
  conformance_required?: 'none' | 'syntactic' | 'semantic' | 'behavioral'
  freshness?: {
    ttl?: string
    max_staleness?: string
  }
  degraded?: Record<string, 'continue' | 'reject'>
}

// SpaceProfileDoc models a kind: SpaceProfile document (§11).
export interface SpaceProfileDoc {
  dsl: 'alexandria.profiles/v1'
  kind: 'SpaceProfile'
  space: {
    id: string
    version: string
    title: LangMap
    description?: LangMap
    operator: string
    base_url: string
  }
  lifecycle?: DslLifecycle
  bindings: DslBinding[]
}

// DslBundle represents the parsed multi-document YAML stream of a hub draft.
export interface DslBundle {
  odrls: OdrlProfileDoc[]
  space: SpaceProfileDoc | null
}

// ODRL_OPERATORS lists the twelve standard ODRL 2.2 comparison operators.
export const ODRL_OPERATORS = ['eq', 'neq', 'lt', 'lteq', 'gt', 'gteq', 'isA', 'hasPart', 'isPartOf', 'isAnyOf', 'isAllOf', 'isNoneOf'] as const

// ORDERED_OPS lists operators that require an ordered XSD datatype (§9.4).
export const ORDERED_OPS = new Set(['lt', 'lteq', 'gt', 'gteq'])

// IRI_OPS lists operators that require value_kind iri (§9.4).
export const IRI_OPS = new Set(['isA', 'hasPart', 'isPartOf'])

// XSD_DATATYPES lists common XSD literal types offered in the signature builder.
export const XSD_DATATYPES = ['xsd:string', 'xsd:integer', 'xsd:decimal', 'xsd:boolean', 'xsd:dateTime', 'xsd:date', 'xsd:time', 'xsd:anyURI'] as const

// ORDERED_DATATYPES lists datatypes admitting ordered comparisons (§9.4).
export const ORDERED_DATATYPES = new Set(['xsd:integer', 'xsd:decimal', 'xsd:double', 'xsd:float', 'xsd:date', 'xsd:dateTime', 'xsd:time'])

// POPULAR_ODRL_ACTIONS lists frequent ODRL 2.2 actions surfaced as quick-toggle pills.
export const POPULAR_ODRL_ACTIONS = ['odrl:aggregate', 'odrl:anonymize', 'odrl:read', 'odrl:distribute', 'odrl:derive', 'odrl:modify', 'odrl:archive', 'odrl:delete'] as const

// ALL_ODRL_ACTIONS lists every ODRL 2.2 action accepted by the N3 vocabulary check.
export const ALL_ODRL_ACTIONS = [
  'acceptTracking', 'aggregate', 'annotate', 'anonymize', 'append', 'appendTo', 'archive', 'attribute',
  'compensate', 'concurrentUse', 'copy', 'delete', 'derive', 'digitize', 'display', 'distribute',
  'ensureExclusivity', 'execute', 'export', 'extract', 'give', 'grantUse', 'include', 'index',
  'inform', 'install', 'lease', 'lend', 'license', 'modify', 'move', 'nextPolicy', 'obtainConsent',
  'pay', 'play', 'present', 'print', 'read', 'reproduce', 'reviewPolicy', 'secondaryUse', 'sell',
  'stream', 'synchronize', 'textToSpeech', 'transfer', 'transform', 'translate', 'uninstall', 'use',
  'watermark', 'write', 'writeTo',
] as const

// ALL_ODRL_LEFT_OPERANDS lists every ODRL 2.2 left operand accepted by the N3 vocabulary check.
export const ALL_ODRL_LEFT_OPERANDS = [
  'dateTime', 'purpose', 'count', 'spatial', 'elapsedTime', 'delayPeriod', 'deliveryChannel',
  'event', 'fileFormat', 'industry', 'language', 'media', 'meteredTime', 'payAmount', 'percentage',
  'product', 'recipient', 'resolution', 'spatialCoordinates', 'systemDevice', 'timeInterval',
  'unitOfCount', 'version', 'virtualLocation', 'absolutePosition', 'absoluteSpatialPosition',
  'absoluteTemporalPosition', 'absoluteSize', 'relativePosition', 'relativeSpatialPosition',
  'relativeTemporalPosition', 'relativeSize', 'device', 'system',
] as const

// TERM_KINDS lists the allowed kinds for profile-defined terms.
export const TERM_KINDS = [
  'action', 'left_operand', 'right_operand', 'operator', 'party_function', 'asset_relation',
  'conflict_term', 'logical_operand', 'rule_subclass', 'policy_subclass', 'constraint_subclass',
] as const

// SLOTS lists the specification slots of SpaceProfile.bindings.
export const SLOTS = ['odrl', 'dsp', 'dcat', 'dcp'] as const

// CONFORMANCE_PRESETS lists common (role, gate) pairs for one-click selection.
export const CONFORMANCE_PRESETS: DslConformance[] = [
  { role: 'json-schema', applies_to: 'policy_intake' },
  { role: 'shacl', applies_to: 'policy_intake' },
  { role: 'json-schema', applies_to: 'dsp_message' },
  { role: 'shacl', applies_to: 'dsp_message' },
  { role: 'json-schema', applies_to: 'catalog_entry' },
  { role: 'shacl', applies_to: 'catalog_entry' },
]

// CONFORMANCE_ROLES lists all allowed conformance artefact roles.
export const CONFORMANCE_ROLES = ['json-schema', 'shacl', 'cedar-mapping', 'cedar-schema'] as const

// GATES lists the validation gates recognized by the DSL schema.
export const GATES = ['policy_intake', 'dsp_message', 'catalog_entry', 'pdp_load', 'credential_presentation'] as const

// CLIENT_GATES lists the six gates accepted under lifecycle.degraded (§11.6).
export const CLIENT_GATES = ['policy_intake', 'dsp_message', 'catalog_entry', 'agreement_admission', 'pdp_load', 'credential_presentation'] as const

// DEFAULT_SUBCLASS_OF maps *_subclass term kinds to their canonical ODRL superclass CURIE.
export const DEFAULT_SUBCLASS_OF: Record<string, string> = {
  rule_subclass: 'odrl:Rule',
  policy_subclass: 'odrl:Policy',
  constraint_subclass: 'odrl:Constraint',
}

// RESERVED_PREFIXES lists namespace prefixes reserved by §6.
export const RESERVED_PREFIXES = new Set(['odrl', 'xsd', 'rdf', 'rdfs', 'owl', 'skos', 'dct', 'sig', 'alx'])

// presetSignatureFor returns a sensible initial signature for a standard ODRL left operand.
export function presetSignatureFor(curie: string): DslSignature {
  const local = curie.replace(/^odrl:/, '')
  if (local === 'dateTime') return { datatype: 'xsd:dateTime', operators: ['lt', 'lteq', 'gt', 'gteq'] }
  if (local === 'count') return { datatype: 'xsd:integer', operators: ['lteq', 'lt', 'eq'] }
  if (local === 'percentage' || local === 'payAmount') return { datatype: 'xsd:decimal', operators: ['lteq', 'gteq', 'eq'] }
  if (local === 'purpose' || local === 'spatial' || local === 'industry' || local === 'recipient') {
    return { value_kind: 'iri', operators: ['eq', 'isAnyOf'] }
  }
  return { datatype: 'xsd:string', operators: ['eq', 'isAnyOf'] }
}

// bumpSemver increments the major, minor or patch component of a SemVer string.
export function bumpSemver(version: string, part: 'major' | 'minor' | 'patch'): string {
  const m = /^(\d+)\.(\d+)\.(\d+)/.exec(version.trim())
  if (!m) return '1.0.0'
  const [maj, min, pat] = [Number(m[1]), Number(m[2]), Number(m[3])]
  if (part === 'major') return `${maj + 1}.0.0`
  if (part === 'minor') return `${maj}.${min + 1}.0`
  return `${maj}.${min}.${pat + 1}`
}

// defaultOdrlProfile constructs a valid starter OdrlProfile for a namespace.
export function defaultOdrlProfile(base: string, slug: string, index = 1): OdrlProfileDoc {
  const suffix = index > 1 ? `-${index}` : ''
  const prefix = index > 1 ? `ex${index}` : 'ex'
  return {
    dsl: 'alexandria.profiles/v1',
    kind: 'OdrlProfile',
    profile: {
      id: `${base}/odrl/profile/${slug}${suffix}`,
      version: '1.0.0',
      title: { en: `${slug} ODRL profile${suffix ? ` (${index})` : ''}` },
      publisher: 'Data Space Authority',
      license: 'https://creativecommons.org/licenses/by/4.0/',
      base_url: `${base}/odrl/profile/${slug}${suffix}`,
    },
    namespaces: { [prefix]: `${base}/ns/${slug}${suffix}#` },
    adopted: {
      left_operands: [{ term: 'odrl:dateTime', signature: { datatype: 'xsd:dateTime', operators: ['lt', 'lteq', 'gt', 'gteq'] } }],
    },
    terms: [
      {
        id: 'access',
        kind: 'action',
        included_in: 'odrl:use',
        label: { en: 'Access' },
        definition: { en: 'To access the asset under the governed policy.' },
      },
    ],
    rules: { actions: 'closed', left_operands: 'closed' },
  }
}

// defaultSpaceProfile constructs a valid starter SpaceProfile bound to odrlId and DSP 2025-1.
export function defaultSpaceProfile(base: string, slug: string, odrlId: string): SpaceProfileDoc {
  return {
    dsl: 'alexandria.profiles/v1',
    kind: 'SpaceProfile',
    space: {
      id: `${base}/spaces/${slug}`,
      version: '1.0.0',
      title: { en: `${slug} data space` },
      operator: `${base}/party/authority`,
      base_url: `${base}/pub/${slug}`,
    },
    bindings: [
      {
        slot: 'odrl',
        profile: odrlId,
        source: 'internal',
        version: '=1.0.0',
        obligation: 'mandatory',
        conformance: [
          { role: 'json-schema', applies_to: 'policy_intake' },
          { role: 'shacl', applies_to: 'policy_intake' },
        ],
      },
      {
        slot: 'dsp',
        profile: 'https://w3id.org/dspace/2025/1',
        source: 'vendored',
        version: '=2025-1',
        obligation: 'mandatory',
        conformance: [{ role: 'json-schema', applies_to: 'dsp_message' }],
      },
    ],
  }
}

// asLangMap normalizes an unknown YAML value into a LangMap with at least an "en" key.
function asLangMap(v: unknown, fallback = ''): LangMap {
  if (v && typeof v === 'object' && !Array.isArray(v)) {
    const out: LangMap = {}
    for (const [k, val] of Object.entries(v as Record<string, unknown>)) {
      if (val !== undefined && val !== null) out[k] = String(val)
    }
    if (!('en' in out)) out.en = fallback
    return out
  }
  return { en: typeof v === 'string' ? v : fallback }
}

// asOptionalLangMap normalizes an optional localized map, returning undefined when empty.
function asOptionalLangMap(v: unknown): LangMap | undefined {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return undefined
  const out: LangMap = {}
  for (const [k, val] of Object.entries(v as Record<string, unknown>)) {
    if (val !== undefined && val !== null && String(val).trim() !== '') out[k] = String(val)
  }
  return Object.keys(out).length ? out : undefined
}

// asSignature normalizes a raw YAML signature object.
function asSignature(v: unknown): DslSignature | undefined {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return undefined
  const r = v as Record<string, unknown>
  const ops = Array.isArray(r.operators) ? r.operators.map(String).filter(Boolean) : []
  const sig: DslSignature = { operators: ops.length ? ops : ['eq'] }
  if (typeof r.datatype === 'string' && r.datatype) sig.datatype = r.datatype
  if (r.value_kind === 'literal' || r.value_kind === 'iri') sig.value_kind = r.value_kind
  if (Array.isArray(r.values) && r.values.length) sig.values = r.values.map(String).filter(Boolean)
  if (typeof r.scheme === 'string' && r.scheme) sig.scheme = r.scheme
  if (typeof r.allow_reference === 'boolean' && r.allow_reference) sig.allow_reference = true
  return sig
}

// asEnforcement normalizes a raw YAML enforcement object.
function asEnforcement(v: unknown): DslEnforcement | undefined {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return undefined
  const r = v as Record<string, unknown>
  if (typeof r.cedar_path !== 'string' || !r.cedar_path) return undefined
  const out: DslEnforcement = { cedar_path: r.cedar_path }
  if (typeof r.pdp_level === 'string' && r.pdp_level) out.pdp_level = r.pdp_level
  if (r.eval_timing === 'once' || r.eval_timing === 'continuous') out.eval_timing = r.eval_timing
  return out
}

// normalizeOdrl converts a raw parsed YAML document into an OdrlProfileDoc.
function normalizeOdrl(raw: Record<string, unknown>): OdrlProfileDoc {
  const p = (raw.profile && typeof raw.profile === 'object' ? raw.profile : {}) as Record<string, unknown>
  const nsRaw = (raw.namespaces && typeof raw.namespaces === 'object' ? raw.namespaces : {}) as Record<string, unknown>
  const namespaces: Record<string, string> = {}
  for (const [k, v] of Object.entries(nsRaw)) {
    if (typeof v === 'string') namespaces[k] = v
  }
  const adoptedRaw = (raw.adopted && typeof raw.adopted === 'object' ? raw.adopted : {}) as Record<string, unknown>
  const adopted: Record<string, DslAdopted[]> = {}
  for (const [cat, list] of Object.entries(adoptedRaw)) {
    if (!Array.isArray(list)) continue
    adopted[cat] = list
      .map((item): DslAdopted | null => {
        if (typeof item === 'string') return { term: item }
        if (item && typeof item === 'object' && typeof (item as Record<string, unknown>).term === 'string') {
          const obj = item as Record<string, unknown>
          return { term: String(obj.term), signature: asSignature(obj.signature), enforcement: asEnforcement(obj.enforcement) }
        }
        return null
      })
      .filter((x): x is DslAdopted => x !== null)
  }
  const termsRaw = Array.isArray(raw.terms) ? raw.terms : []
  const terms: DslTerm[] = termsRaw
    .filter((t): t is Record<string, unknown> => !!t && typeof t === 'object')
    .map((t) => ({
      id: typeof t.id === 'string' ? t.id : '',
      kind: typeof t.kind === 'string' ? t.kind : 'action',
      label: asLangMap(t.label),
      definition: asLangMap(t.definition),
      note: asOptionalLangMap(t.note),
      included_in: typeof t.included_in === 'string' && t.included_in ? t.included_in : undefined,
      subclass_of: typeof t.subclass_of === 'string' && t.subclass_of ? t.subclass_of : undefined,
      signature: asSignature(t.signature),
      enforcement: asEnforcement(t.enforcement),
      status: t.status === 'deprecated' || t.status === 'retired' || t.status === 'active' ? t.status : undefined,
      replaced_by: typeof t.replaced_by === 'string' && t.replaced_by ? t.replaced_by : undefined,
      extra_turtle: typeof t.extra_turtle === 'string' && t.extra_turtle ? t.extra_turtle : undefined,
    }))
  const rulesRaw = (raw.rules && typeof raw.rules === 'object' ? raw.rules : {}) as Record<string, unknown>
  return {
    dsl: 'alexandria.profiles/v1',
    kind: 'OdrlProfile',
    profile: {
      id: typeof p.id === 'string' ? p.id : '',
      version: typeof p.version === 'string' ? p.version : '1.0.0',
      title: asLangMap(p.title),
      description: asOptionalLangMap(p.description),
      publisher: typeof p.publisher === 'string' ? p.publisher : '',
      license: typeof p.license === 'string' ? p.license : 'https://creativecommons.org/licenses/by/4.0/',
      base_url: typeof p.base_url === 'string' ? p.base_url : '',
    },
    namespaces,
    adopted,
    terms,
    rules: {
      actions: rulesRaw.actions === 'open' ? 'open' : 'closed',
      left_operands: rulesRaw.left_operands === 'open' ? 'open' : 'closed',
    },
  }
}

// normalizeSpace converts a raw parsed YAML document into a SpaceProfileDoc.
function normalizeSpace(raw: Record<string, unknown>): SpaceProfileDoc {
  const s = (raw.space && typeof raw.space === 'object' ? raw.space : {}) as Record<string, unknown>
  const bindingsRaw = Array.isArray(raw.bindings) ? raw.bindings : []
  const bindings: DslBinding[] = bindingsRaw
    .filter((b): b is Record<string, unknown> => !!b && typeof b === 'object')
    .map((b) => ({
      slot: typeof b.slot === 'string' ? b.slot : 'odrl',
      profile: typeof b.profile === 'string' ? b.profile : '',
      source: b.source === 'vendored' ? 'vendored' : 'internal',
      version: typeof b.version === 'string' ? b.version : '=1.0.0',
      obligation: b.obligation === 'recommended' || b.obligation === 'optional' ? b.obligation : 'mandatory',
      via: Array.isArray(b.via) && b.via.length ? b.via.map(String) : undefined,
      conformance: Array.isArray(b.conformance)
        ? b.conformance
            .filter((c): c is Record<string, unknown> => !!c && typeof c === 'object')
            .map((c) => ({
              role: typeof c.role === 'string' ? c.role : 'json-schema',
              applies_to: typeof c.applies_to === 'string' ? c.applies_to : 'policy_intake',
            }))
        : [],
    }))
  const lc = raw.lifecycle && typeof raw.lifecycle === 'object' && !Array.isArray(raw.lifecycle) ? (raw.lifecycle as DslLifecycle) : undefined
  return {
    dsl: 'alexandria.profiles/v1',
    kind: 'SpaceProfile',
    space: {
      id: typeof s.id === 'string' ? s.id : '',
      version: typeof s.version === 'string' ? s.version : '1.0.0',
      title: asLangMap(s.title),
      description: asOptionalLangMap(s.description),
      operator: typeof s.operator === 'string' ? s.operator : '',
      base_url: typeof s.base_url === 'string' ? s.base_url : '',
    },
    lifecycle: lc,
    bindings,
  }
}

// parseDslBundle parses a YAML stream into a DslBundle, reporting syntax errors without throwing.
export function parseDslBundle(yamlText: string): { bundle: DslBundle | null; error: string | null } {
  const trimmed = yamlText.trim()
  if (!trimmed) return { bundle: { odrls: [], space: null }, error: null }
  if (trimmed.startsWith('{')) return { bundle: null, error: 'JSON PoC document (not a YAML DSL stream)' }
  try {
    const docs = YAML.parseAllDocuments(yamlText, { strict: false })
    for (const d of docs) {
      if (d.errors.length > 0) {
        const first = d.errors[0]
        const line = first.linePos?.[0]?.line
        return { bundle: null, error: `${line ? `Line ${line}: ` : ''}${first.message.split('\n')[0]}` }
      }
    }
    const odrls: OdrlProfileDoc[] = []
    let space: SpaceProfileDoc | null = null
    for (const d of docs) {
      const js = d.toJSON() as unknown
      if (!js || typeof js !== 'object' || Array.isArray(js)) continue
      const obj = js as Record<string, unknown>
      if (obj.kind === 'OdrlProfile') odrls.push(normalizeOdrl(obj))
      else if (obj.kind === 'SpaceProfile' && !space) space = normalizeSpace(obj)
    }
    return { bundle: { odrls, space }, error: null }
  } catch (e) {
    return { bundle: null, error: e instanceof Error ? e.message : String(e) }
  }
}

// cleanLangMap strips empty secondary language entries while keeping "en".
function cleanLangMap(m: LangMap | undefined): LangMap | undefined {
  if (!m) return undefined
  const out: LangMap = {}
  for (const [k, v] of Object.entries(m)) {
    if (k === 'en' || v.trim() !== '') out[k] = v
  }
  return Object.keys(out).length ? out : undefined
}

// cleanSignature builds a compact serializable signature object.
function cleanSignature(s: DslSignature | undefined): Record<string, unknown> | undefined {
  if (!s) return undefined
  const out: Record<string, unknown> = {}
  if (s.datatype) out.datatype = s.datatype
  if (s.value_kind && !s.datatype) out.value_kind = s.value_kind
  if (s.values?.length) out.values = s.values
  if (s.scheme) out.scheme = s.scheme
  out.operators = s.operators.length ? s.operators : ['eq']
  if (s.allow_reference) out.allow_reference = true
  return out
}

// serializeOdrl prepares an OdrlProfileDoc for clean YAML emission.
function serializeOdrl(doc: OdrlProfileDoc): Record<string, unknown> {
  const profile: Record<string, unknown> = {
    id: doc.profile.id,
    version: doc.profile.version,
    title: cleanLangMap(doc.profile.title) ?? { en: '' },
  }
  const desc = cleanLangMap(doc.profile.description)
  if (desc && desc.en?.trim()) profile.description = desc
  profile.publisher = doc.profile.publisher
  profile.license = doc.profile.license
  profile.base_url = doc.profile.base_url

  const out: Record<string, unknown> = {
    dsl: 'alexandria.profiles/v1',
    kind: 'OdrlProfile',
    profile,
    namespaces: doc.namespaces,
  }

  const adoptedOut: Record<string, unknown[]> = {}
  for (const [cat, items] of Object.entries(doc.adopted)) {
    if (!items.length) continue
    adoptedOut[cat] = items.map((a) => {
      if (!a.signature && !a.enforcement) return a.term
      const entry: Record<string, unknown> = { term: a.term }
      if (a.signature) entry.signature = cleanSignature(a.signature)
      if (a.enforcement) entry.enforcement = a.enforcement
      return entry
    })
  }
  if (Object.keys(adoptedOut).length) out.adopted = adoptedOut

  if (doc.terms.length) {
    out.terms = doc.terms.map((t) => {
      const term: Record<string, unknown> = { id: t.id, kind: t.kind }
      if (t.kind === 'action' && t.included_in) term.included_in = t.included_in
      if (t.subclass_of && t.kind.endsWith('_subclass')) term.subclass_of = t.subclass_of
      if (t.status && t.status !== 'active') term.status = t.status
      if (t.replaced_by) term.replaced_by = t.replaced_by
      term.label = cleanLangMap(t.label) ?? { en: '' }
      term.definition = cleanLangMap(t.definition) ?? { en: '' }
      const note = cleanLangMap(t.note)
      if (note && note.en?.trim()) term.note = note
      if (t.kind === 'left_operand' && t.signature) term.signature = cleanSignature(t.signature)
      if (t.kind === 'left_operand' && t.enforcement) term.enforcement = t.enforcement
      if (t.extra_turtle) term.extra_turtle = t.extra_turtle
      return term
    })
  }

  if (doc.rules.actions === 'open' || doc.rules.left_operands === 'open') {
    out.rules = { actions: doc.rules.actions, left_operands: doc.rules.left_operands }
  }
  return out
}

// serializeSpace prepares a SpaceProfileDoc for clean YAML emission.
function serializeSpace(doc: SpaceProfileDoc): Record<string, unknown> {
  const space: Record<string, unknown> = {
    id: doc.space.id,
    version: doc.space.version,
    title: cleanLangMap(doc.space.title) ?? { en: '' },
  }
  const desc = cleanLangMap(doc.space.description)
  if (desc && desc.en?.trim()) space.description = desc
  space.operator = doc.space.operator
  space.base_url = doc.space.base_url

  const out: Record<string, unknown> = {
    dsl: 'alexandria.profiles/v1',
    kind: 'SpaceProfile',
    space,
  }
  if (doc.lifecycle && Object.keys(doc.lifecycle).length > 0) {
    out.lifecycle = doc.lifecycle
  }
  out.bindings = doc.bindings.map((b) => {
    const item: Record<string, unknown> = {
      slot: b.slot,
      profile: b.profile,
      source: b.source,
      version: b.version,
      obligation: b.obligation,
    }
    if (b.via?.length) item.via = b.via
    if (b.conformance.length) item.conformance = b.conformance.map((c) => ({ role: c.role, applies_to: c.applies_to }))
    return item
  })
  return out
}

// serializeDslBundle renders a DslBundle into a deterministic, human-friendly multi-document YAML stream.
export function serializeDslBundle(bundle: DslBundle): string {
  const docs: Record<string, unknown>[] = bundle.odrls.map(serializeOdrl)
  if (bundle.space) docs.push(serializeSpace(bundle.space))
  if (docs.length === 0) return ''
  return (
    docs
      .map((raw) => {
        const ydoc = new YAML.Document(raw)
        YAML.visit(ydoc, {
          Pair(_, pair) {
            const key = YAML.isScalar(pair.key) ? String(pair.key.value) : ''
            if ((key === 'operators' || key === 'values') && YAML.isSeq(pair.value) && pair.value.items.length <= 8) {
              pair.value.flow = true
            }
            if ((key === 'title' || key === 'label') && YAML.isMap(pair.value) && pair.value.items.length === 1) {
              const first = pair.value.items[0]
              if (YAML.isScalar(first?.value) && String(first.value.value).length <= 48) pair.value.flow = true
            }
            if (key === 'conformance' && YAML.isSeq(pair.value)) {
              for (const item of pair.value.items) {
                if (YAML.isMap(item)) item.flow = true
              }
            }
          },
        })
        return ydoc.toString({ lineWidth: 0 }).trimEnd()
      })
      .join('\n---\n') + '\n'
  )
}
