// Accordion-based form controls for authoring an alexandria.profiles/v1 OdrlProfile document.
// Keeps sections collapsed to scannable summaries until expanded so the studio stays calm and legible.
import { useState, type ReactNode } from 'react'
import { ChevronDown, ChevronRight, Plus, Trash2, X } from 'lucide-react'
import { cn } from '@/lib/cn'
import {
  ALL_ODRL_ACTIONS,
  ALL_ODRL_LEFT_OPERANDS,
  DEFAULT_SUBCLASS_OF,
  IRI_OPS,
  ODRL_OPERATORS,
  ORDERED_DATATYPES,
  ORDERED_OPS,
  presetSignatureFor,
  RESERVED_PREFIXES,
  TERM_KINDS,
  XSD_DATATYPES,
  type DslAdopted,
  type DslSignature,
  type DslTerm,
  type OdrlProfileDoc,
} from '@/lib/dsl-bundle'
import { Button, IconButton } from '@/components/ui/button'
import { Field, Input, Select, Textarea } from '@/components/ui/form'
import { Badge } from '@/components/ui/misc'

// Accordion groups top-level collapsible sections inside a single clean bordered surface.
export function Accordion({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('divide-y rounded-lg border bg-raised', className)}>{children}</div>
}

// AccordionItem renders one collapsible section with a quiet right-aligned summary and optional action.
export function AccordionItem({
  title,
  summary,
  open,
  onToggle,
  action,
  children,
}: {
  title: string
  summary?: ReactNode
  open: boolean
  onToggle: () => void
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <div>
      <div
        role="button"
        tabIndex={0}
        aria-expanded={open}
        onClick={onToggle}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            onToggle()
          }
        }}
        className="flex h-10 cursor-pointer items-center gap-2.5 px-4 select-none hover:bg-hover/50"
      >
        <ChevronRight className={cn('size-3.5 shrink-0 text-subtle transition-transform duration-150', open && 'rotate-90 text-fg')} />
        <span className="text-sm font-medium text-fg">{title}</span>
        <div className="ml-auto flex min-w-0 items-center gap-2">
          {!open && summary ? <span className="truncate font-mono text-xs text-subtle">{summary}</span> : null}
          {open && action ? <div onClick={(e) => e.stopPropagation()}>{action}</div> : null}
        </div>
      </div>
      {open ? <div className="border-t bg-bg/40 px-4 py-3.5">{children}</div> : null}
    </div>
  )
}

// Segmented renders a minimal mutually-exclusive switch aligned to the console rhythm.
export function Segmented<T extends string>({
  value,
  options,
  onChange,
  className,
}: {
  value: T
  options: { id: T; label: string }[]
  onChange: (next: T) => void
  className?: string
}) {
  return (
    <div className={cn('inline-flex h-7 shrink-0 items-center rounded-md border border-line-strong bg-bg p-0.5', className)}>
      {options.map((o) => (
        <button
          key={o.id}
          type="button"
          onClick={() => onChange(o.id)}
          className={cn(
            'h-6 rounded-sm px-2 text-xs font-medium transition-colors',
            value === o.id ? 'bg-accent-soft text-accent' : 'text-muted hover:text-fg',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

// SignatureEditor renders a compact datatype + operator selector for left_operand constraints.
export function SignatureEditor({
  signature,
  onChange,
  rightOperandCuries,
}: {
  signature: DslSignature | undefined
  onChange: (next: DslSignature) => void
  rightOperandCuries: string[]
}) {
  const [newVal, setNewVal] = useState('')
  const [addingValue, setAddingValue] = useState(false)
  const sig: DslSignature = signature ?? { datatype: 'xsd:string', operators: ['eq'] }
  const isIri = sig.value_kind === 'iri' && !sig.datatype
  const selectValue = isIri ? '__iri__' : sig.datatype || 'xsd:string'
  const canOrder = !isIri && ORDERED_DATATYPES.has(selectValue)

  const handleTypeChange = (val: string) => {
    if (val === '__iri__') {
      const kept = sig.operators.filter((op) => !ORDERED_OPS.has(op))
      onChange({
        value_kind: 'iri',
        operators: kept.length ? kept : ['eq', 'isAnyOf'],
        values: sig.values,
        scheme: sig.scheme,
        allow_reference: sig.allow_reference,
      })
      return
    }
    const ordered = ORDERED_DATATYPES.has(val)
    const kept = sig.operators.filter((op) => (ordered || !ORDERED_OPS.has(op)) && !IRI_OPS.has(op))
    onChange({
      datatype: val,
      operators: kept.length ? kept : ordered ? ['lteq', 'eq'] : ['eq'],
      values: sig.values,
      allow_reference: sig.allow_reference,
    })
  }

  const toggleOp = (op: string) => {
    const next = sig.operators.includes(op) ? sig.operators.filter((x) => x !== op) : [...sig.operators, op]
    if (next.length > 0) onChange({ ...sig, operators: next })
  }

  const toggleValue = (val: string) => {
    const cur = sig.values ?? []
    const next = cur.includes(val) ? cur.filter((x) => x !== val) : [...cur, val]
    onChange({ ...sig, values: next.length ? next : undefined, scheme: next.length ? undefined : sig.scheme })
  }

  const commitNewVal = () => {
    const v = newVal.trim()
    if (!v) return
    if (!(sig.values ?? []).includes(v)) onChange({ ...sig, values: [...(sig.values ?? []), v], scheme: undefined })
    setNewVal('')
  }

  return (
    <div className="space-y-3 border-t pt-3">
      <Field label="Value type">
        <Select className="[&>select]:font-mono [&>select]:text-xs" value={selectValue} onChange={(e) => handleTypeChange(e.target.value)}>
          <optgroup label="XSD Literal">
            {XSD_DATATYPES.map((dt) => (
              <option key={dt} value={dt}>{dt}</option>
            ))}
            {!isIri && !XSD_DATATYPES.includes(selectValue as (typeof XSD_DATATYPES)[number]) ? (
              <option value={selectValue}>{selectValue}</option>
            ) : null}
          </optgroup>
          <optgroup label="Identifier">
            <option value="__iri__">IRI / Concept (value_kind: iri)</option>
          </optgroup>
        </Select>
      </Field>

      <div>
        <span className="mb-1.5 block text-xs font-medium text-muted">Operators</span>
        <div className="flex flex-wrap gap-1">
          {ODRL_OPERATORS.map((op) => {
            const active = sig.operators.includes(op)
            const disabled = (ORDERED_OPS.has(op) && !canOrder) || (IRI_OPS.has(op) && !isIri)
            return (
              <button
                key={op}
                type="button"
                disabled={disabled && !active}
                onClick={() => toggleOp(op)}
                className={cn(
                  'h-6 rounded-sm border px-2 font-mono text-xs transition-colors',
                  active
                    ? 'border-accent/50 bg-accent-soft font-medium text-accent'
                    : 'border-line-strong bg-bg text-muted hover:text-fg disabled:opacity-30',
                )}
              >
                {op}
              </button>
            )
          })}
        </div>
      </div>

      {isIri && rightOperandCuries.length > 0 ? (
        <div>
          <span className="mb-1 block text-xs font-medium text-muted">Allowed right operands</span>
          <div className="flex flex-wrap gap-1">
            {rightOperandCuries.map((curie) => {
              const picked = sig.values?.includes(curie)
              return (
                <button
                  key={curie}
                  type="button"
                  onClick={() => toggleValue(curie)}
                  className={cn(
                    'h-6 rounded-sm border px-2 font-mono text-xs transition-colors',
                    picked ? 'border-accent/50 bg-accent-soft font-medium text-accent' : 'border-line-strong bg-bg text-muted hover:text-fg',
                  )}
                >
                  {curie}
                </button>
              )
            })}
          </div>
        </div>
      ) : null}

      {sig.values?.length || addingValue ? (
        <div className="space-y-1.5">
          <span className="block text-xs font-medium text-muted">Allowed values</span>
          {sig.values?.length ? (
            <div className="flex flex-wrap gap-1">
              {sig.values.map((v) => (
                <span key={v} className="inline-flex h-6 items-center gap-1 rounded-sm border border-line-strong bg-bg px-2 font-mono text-xs">
                  {v}
                  <button type="button" onClick={() => toggleValue(v)} className="text-subtle hover:text-danger" aria-label={`Remove ${v}`}>
                    <X className="size-3" />
                  </button>
                </span>
              ))}
            </div>
          ) : null}
          <div className="flex gap-1.5">
            <Input
              mono
              className="text-xs"
              value={newVal}
              placeholder={isIri ? 'ex:researchUse' : 'Allowed literal value…'}
              onChange={(e) => setNewVal(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  commitNewVal()
                }
              }}
            />
            <Button size="sm" className="h-8" disabled={!newVal.trim()} onClick={commitNewVal}>
              Add
            </Button>
          </div>
        </div>
      ) : (
        <button type="button" onClick={() => setAddingValue(true)} className="text-xs text-muted hover:text-fg">
          + Restrict to specific values…
        </button>
      )}
    </div>
  )
}

// uniqueLocalId generates a non-colliding valid LocalId for a newly created term.
function uniqueLocalId(existing: DslTerm[], baseName: string): string {
  const taken = new Set(existing.map((t) => t.id))
  if (!taken.has(baseName)) return baseName
  let n = 2
  while (taken.has(`${baseName}${n}`)) n++
  return `${baseName}${n}`
}

// uniquePrefix generates a non-colliding namespace prefix when adding a new row.
function uniquePrefix(namespaces: Record<string, string>): string {
  let n = 2
  while (`ex${n}` in namespaces) n++
  return `ex${n}`
}

// kindTone assigns a subtle semantic badge tone to each ODRL term kind.
function kindTone(kind: string): 'accent' | 'info' | 'purple' | 'success' | 'neutral' {
  if (kind === 'action') return 'accent'
  if (kind === 'left_operand') return 'info'
  if (kind === 'right_operand') return 'purple'
  if (kind.endsWith('_subclass')) return 'success'
  return 'neutral'
}

// OdrlSectionId identifies one collapsible accordion panel in OdrlProfileForm.
type OdrlSectionId = 'profile' | 'namespaces' | 'adopted' | 'terms'

// OdrlProfileForm renders the accordion-based visual authoring form for one OdrlProfileDoc.
export function OdrlProfileForm({
  doc,
  onChange,
}: {
  doc: OdrlProfileDoc
  onChange: (next: OdrlProfileDoc) => void
}) {
  const [openSec, setOpenSec] = useState<Record<OdrlSectionId, boolean>>({
    profile: true,
    namespaces: false,
    adopted: false,
    terms: true,
  })
  const [showMoreMeta, setShowMoreMeta] = useState(
    Boolean(doc.profile.description?.en || (doc.profile.base_url && doc.profile.base_url !== doc.profile.id)),
  )
  const [openOperand, setOpenOperand] = useState<string | null>(null)
  const [openTerm, setOpenTerm] = useState<number | null>(null)

  const toggleSec = (id: OdrlSectionId) => setOpenSec((prev) => ({ ...prev, [id]: !prev[id] }))

  const nsEntries = Object.entries(doc.namespaces)
  const primaryPrefix = nsEntries[0]?.[0] || 'ex'
  const rightOperandCuries = doc.terms.filter((t) => t.kind === 'right_operand' && t.id).map((t) => `${primaryPrefix}:${t.id}`)

  const setProfile = (patch: Partial<OdrlProfileDoc['profile']>) =>
    onChange({ ...doc, profile: { ...doc.profile, ...patch } })

  const adoptedActions = doc.adopted.actions ?? []
  const adoptedLeftOps = doc.adopted.left_operands ?? []
  const adoptedActionSet = new Set(adoptedActions.map((a) => a.term))
  const adoptedLeftOpSet = new Set(adoptedLeftOps.map((a) => a.term))

  const addAdoptedAction = (localName: string) => {
    if (!localName) return
    const curie = `odrl:${localName}`
    if (adoptedActionSet.has(curie)) return
    onChange({ ...doc, adopted: { ...doc.adopted, actions: [...adoptedActions, { term: curie }] } })
  }

  const removeAdoptedAction = (curie: string) => {
    onChange({ ...doc, adopted: { ...doc.adopted, actions: adoptedActions.filter((a) => a.term !== curie) } })
  }

  const addAdoptedLeftOp = (localName: string) => {
    if (!localName) return
    const curie = `odrl:${localName}`
    if (adoptedLeftOpSet.has(curie)) return
    const next: DslAdopted[] = [...adoptedLeftOps, { term: curie, signature: presetSignatureFor(curie) }]
    setOpenOperand(curie)
    onChange({ ...doc, adopted: { ...doc.adopted, left_operands: next } })
  }

  const updateAdoptedLeftOp = (idx: number, nextItem: DslAdopted) => {
    onChange({
      ...doc,
      adopted: { ...doc.adopted, left_operands: adoptedLeftOps.map((item, i) => (i === idx ? nextItem : item)) },
    })
  }

  const removeAdoptedLeftOp = (idx: number) => {
    onChange({ ...doc, adopted: { ...doc.adopted, left_operands: adoptedLeftOps.filter((_, i) => i !== idx) } })
  }

  const addTerm = () => {
    const id = uniqueLocalId(doc.terms, 'newTerm')
    const next: DslTerm = {
      id,
      kind: 'action',
      included_in: 'odrl:use',
      label: { en: 'New term' },
      definition: { en: 'Governed action defined by this profile.' },
    }
    setOpenSec((prev) => ({ ...prev, terms: true }))
    setOpenTerm(doc.terms.length)
    onChange({ ...doc, terms: [...doc.terms, next] })
  }

  const updateTerm = (idx: number, patch: Partial<DslTerm>) => {
    const next = doc.terms.map((t, i) => {
      if (i !== idx) return t
      const merged: DslTerm = { ...t, ...patch }
      if (patch.kind) {
        if (patch.kind === 'action' && !merged.included_in) merged.included_in = 'odrl:use'
        if (patch.kind === 'left_operand' && !merged.signature) merged.signature = { datatype: 'xsd:string', operators: ['eq'] }
        if (DEFAULT_SUBCLASS_OF[patch.kind] && !merged.subclass_of) merged.subclass_of = DEFAULT_SUBCLASS_OF[patch.kind]
      }
      return merged
    })
    onChange({ ...doc, terms: next })
  }

  const removeTerm = (idx: number) => {
    if (openTerm === idx) setOpenTerm(null)
    onChange({ ...doc, terms: doc.terms.filter((_, i) => i !== idx) })
  }

  const addPrefix = () => {
    const p = uniquePrefix(doc.namespaces)
    const base = doc.profile.id.replace(/\/odrl\/profile\/.*$/, '') || 'https://example.org'
    onChange({ ...doc, namespaces: { ...doc.namespaces, [p]: `${base}/ns/${p}#` } })
  }

  const adoptedTotal = adoptedActions.length + adoptedLeftOps.length

  return (
    <Accordion>
      <AccordionItem
        title="Profile"
        summary={`${doc.profile.version}${doc.profile.title.en ? ` · ${doc.profile.title.en}` : ''}`}
        open={openSec.profile}
        onToggle={() => toggleSec('profile')}
      >
        <div className="space-y-3">
          <div className="grid grid-cols-[1fr_112px] gap-2.5">
            <Field label="Title">
              <Input
                value={doc.profile.title.en ?? ''}
                placeholder="Default ODRL profile"
                onChange={(e) => setProfile({ title: { ...doc.profile.title, en: e.target.value } })}
              />
            </Field>
            <Field label="Version">
              <Input mono className="text-xs" value={doc.profile.version} placeholder="1.0.0" onChange={(e) => setProfile({ version: e.target.value })} />
            </Field>
          </div>

          <Field label="Profile IRI">
            <Input
              mono
              className="text-xs"
              value={doc.profile.id}
              placeholder="https://hub.example.com/odrl/profile/default"
              onChange={(e) => {
                const nextId = e.target.value
                const syncBase = !doc.profile.base_url || doc.profile.base_url === doc.profile.id
                setProfile({ id: nextId, ...(syncBase ? { base_url: nextId } : {}) })
              }}
            />
          </Field>

          <Field label="Publisher">
            <Input value={doc.profile.publisher} placeholder="Data Space Authority" onChange={(e) => setProfile({ publisher: e.target.value })} />
          </Field>

          <Field label="License IRI">
            <Input mono className="text-xs" value={doc.profile.license} placeholder="https://creativecommons.org/licenses/by/4.0/" onChange={(e) => setProfile({ license: e.target.value })} />
          </Field>

          {showMoreMeta ? (
            <div className="space-y-3 border-t pt-3">
              <Field label="Publication base URL">
                <Input mono className="text-xs" value={doc.profile.base_url} placeholder={doc.profile.id} onChange={(e) => setProfile({ base_url: e.target.value })} />
              </Field>
              <Field label="Description (optional)">
                <Textarea
                  rows={2}
                  value={doc.profile.description?.en ?? ''}
                  placeholder="Scope and governance rules of this profile…"
                  onChange={(e) => setProfile({ description: e.target.value ? { ...doc.profile.description, en: e.target.value } : undefined })}
                />
              </Field>
            </div>
          ) : (
            <button type="button" onClick={() => setShowMoreMeta(true)} className="text-xs text-muted hover:text-fg">
              + Base URL &amp; description…
            </button>
          )}
        </div>
      </AccordionItem>

      <AccordionItem
        title="Namespaces &amp; closure"
        summary={`${nsEntries.map(([p]) => p).join(', ') || 'none'} · ${doc.rules.actions}`}
        open={openSec.namespaces}
        onToggle={() => toggleSec('namespaces')}
        action={
          <Button size="sm" variant="ghost" icon={<Plus className="size-3.5" />} onClick={addPrefix}>
            Add prefix
          </Button>
        }
      >
        <div className="space-y-3">
          <div className="space-y-2">
            {nsEntries.map(([prefix, iri], idx) => {
              const badSuffix = iri.length > 0 && !iri.endsWith('#') && !iri.endsWith('/')
              const reserved = RESERVED_PREFIXES.has(prefix)
              return (
                <div key={idx} className="space-y-1">
                  <div className="grid grid-cols-[84px_1fr_auto] items-center gap-2">
                    <Input
                      mono
                      className="text-xs"
                      value={prefix}
                      placeholder="ex"
                      onChange={(e) => {
                        const np = e.target.value.trim()
                        const rebuilt: Record<string, string> = {}
                        nsEntries.forEach(([k, v], i) => {
                          rebuilt[i === idx ? np : k] = v
                        })
                        onChange({ ...doc, namespaces: rebuilt })
                      }}
                    />
                    <Input
                      mono
                      className="text-xs"
                      value={iri}
                      placeholder="https://hub.example.com/ns/default#"
                      onChange={(e) => onChange({ ...doc, namespaces: { ...doc.namespaces, [prefix]: e.target.value } })}
                    />
                    {nsEntries.length > 1 ? (
                      <IconButton
                        size="sm"
                        label="Remove prefix"
                        onClick={() => {
                          const next = { ...doc.namespaces }
                          delete next[prefix]
                          onChange({ ...doc, namespaces: next })
                        }}
                      >
                        <Trash2 className="size-3.5" />
                      </IconButton>
                    ) : (
                      <span className="w-1" />
                    )}
                  </div>
                  {reserved ? <p className="text-xs text-danger">Prefix “{prefix}” is reserved by Alexandria.</p> : null}
                  {badSuffix ? (
                    <p className="text-xs text-warning">
                      Namespace IRI must end in <code>#</code> or <code>/</code> ·{' '}
                      <button
                        type="button"
                        className="underline"
                        onClick={() => onChange({ ...doc, namespaces: { ...doc.namespaces, [prefix]: `${iri}#` } })}
                      >
                        append #
                      </button>
                    </p>
                  ) : null}
                </div>
              )
            })}
          </div>

          <div className="divide-y border-t pt-1">
            <div className="flex items-center justify-between gap-4 py-2">
              <span className="text-sm text-fg">Actions vocabulary</span>
              <Segmented
                value={doc.rules.actions}
                onChange={(v) => onChange({ ...doc, rules: { ...doc.rules, actions: v } })}
                options={[
                  { id: 'closed', label: 'Closed' },
                  { id: 'open', label: 'Open' },
                ]}
              />
            </div>
            <div className="flex items-center justify-between gap-4 pt-2">
              <span className="text-sm text-fg">Left operands vocabulary</span>
              <Segmented
                value={doc.rules.left_operands}
                onChange={(v) => onChange({ ...doc, rules: { ...doc.rules, left_operands: v } })}
                options={[
                  { id: 'closed', label: 'Closed' },
                  { id: 'open', label: 'Open' },
                ]}
              />
            </div>
          </div>
        </div>
      </AccordionItem>

      <AccordionItem
        title="Adopted ODRL 2.2"
        summary={`${adoptedTotal} ${adoptedTotal === 1 ? 'term' : 'terms'}`}
        open={openSec.adopted}
        onToggle={() => toggleSec('adopted')}
      >
        <div className="space-y-4">
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-muted">Standard actions</span>
              <Select className="w-44 shrink-0 [&>select]:h-7 [&>select]:text-xs" value="" onChange={(e) => addAdoptedAction(e.target.value)}>
                <option value="">+ Adopt action…</option>
                {ALL_ODRL_ACTIONS.filter((a) => !adoptedActionSet.has(`odrl:${a}`)).map((a) => (
                  <option key={a} value={a}>odrl:{a}</option>
                ))}
              </Select>
            </div>
            {adoptedActions.length === 0 ? (
              <p className="text-xs text-subtle">None adopted — only profile-defined actions are admitted when closed.</p>
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {adoptedActions.map((a) => (
                  <span key={a.term} className="inline-flex h-6 items-center gap-1 rounded-sm border border-line-strong bg-raised px-2 font-mono text-xs text-fg">
                    {a.term}
                    <button type="button" onClick={() => removeAdoptedAction(a.term)} className="text-subtle hover:text-danger" aria-label={`Remove ${a.term}`}>
                      <X className="size-3" />
                    </button>
                  </span>
                ))}
              </div>
            )}
          </div>

          <div className="space-y-2 border-t pt-3">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-muted">Standard left operands</span>
              <Select className="w-44 shrink-0 [&>select]:h-7 [&>select]:text-xs" value="" onChange={(e) => addAdoptedLeftOp(e.target.value)}>
                <option value="">+ Adopt operand…</option>
                {ALL_ODRL_LEFT_OPERANDS.filter((lo) => !adoptedLeftOpSet.has(`odrl:${lo}`)).map((lo) => (
                  <option key={lo} value={lo}>odrl:{lo}</option>
                ))}
              </Select>
            </div>

            {adoptedLeftOps.length === 0 ? (
              <p className="text-xs text-subtle">None adopted.</p>
            ) : (
              <div className="divide-y rounded-md border bg-raised">
                {adoptedLeftOps.map((item, idx) => {
                  const open = openOperand === item.term
                  return (
                    <div key={item.term}>
                      <div
                        role="button"
                        tabIndex={0}
                        onClick={() => setOpenOperand(open ? null : item.term)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.preventDefault()
                            setOpenOperand(open ? null : item.term)
                          }
                        }}
                        className="flex h-9 cursor-pointer items-center gap-2 px-3 select-none hover:bg-hover/60"
                      >
                        {open ? <ChevronDown className="size-3.5 shrink-0 text-subtle" /> : <ChevronRight className="size-3.5 shrink-0 text-subtle" />}
                        <span className="font-mono text-xs font-medium text-fg">{item.term}</span>
                        <span className="min-w-0 flex-1 truncate font-mono text-xs text-subtle">
                          {item.signature?.datatype ?? item.signature?.value_kind ?? 'any'} · {(item.signature?.operators ?? []).join(', ')}
                        </span>
                        <div onClick={(e) => e.stopPropagation()}>
                          <IconButton size="sm" label={`Remove ${item.term}`} onClick={() => removeAdoptedLeftOp(idx)}>
                            <Trash2 className="size-3.5" />
                          </IconButton>
                        </div>
                      </div>
                      {open ? (
                        <div className="px-3 pb-3">
                          <SignatureEditor
                            signature={item.signature}
                            rightOperandCuries={rightOperandCuries}
                            onChange={(sig) => updateAdoptedLeftOp(idx, { ...item, signature: sig })}
                          />
                        </div>
                      ) : null}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      </AccordionItem>

      <AccordionItem
        title="Terms"
        summary={`${doc.terms.length} ${doc.terms.length === 1 ? 'term' : 'terms'}`}
        open={openSec.terms}
        onToggle={() => toggleSec('terms')}
        action={
          <Button size="sm" variant="ghost" icon={<Plus className="size-3.5" />} onClick={addTerm}>
            Add term
          </Button>
        }
      >
        {doc.terms.length === 0 ? (
          <p className="text-xs text-subtle">No profile-specific terms defined yet.</p>
        ) : (
          <div className="divide-y rounded-md border bg-raised">
            {doc.terms.map((term, idx) => {
              const open = openTerm === idx
              const invalidId = term.id.length > 0 && !/^[A-Za-z][A-Za-z0-9_]*$/.test(term.id)
              return (
                <div key={idx}>
                  <div
                    role="button"
                    tabIndex={0}
                    onClick={() => setOpenTerm(open ? null : idx)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        setOpenTerm(open ? null : idx)
                      }
                    }}
                    className="flex h-9 cursor-pointer items-center gap-2 px-3 select-none hover:bg-hover/60"
                  >
                    {open ? <ChevronDown className="size-3.5 shrink-0 text-subtle" /> : <ChevronRight className="size-3.5 shrink-0 text-subtle" />}
                    <span className="font-mono text-xs font-medium text-fg">
                      {primaryPrefix}:{term.id || '…'}
                    </span>
                    <Badge tone={kindTone(term.kind)}>{term.kind.replace('_', ' ')}</Badge>
                    {term.label.en ? <span className="min-w-0 flex-1 truncate text-xs text-muted">{term.label.en}</span> : <span className="flex-1" />}
                    {term.status && term.status !== 'active' ? <Badge tone="warning">{term.status}</Badge> : null}
                    <div onClick={(e) => e.stopPropagation()}>
                      <IconButton size="sm" label="Delete term" onClick={() => removeTerm(idx)}>
                        <Trash2 className="size-3.5" />
                      </IconButton>
                    </div>
                  </div>

                  {open ? (
                    <div className="space-y-3 border-t bg-side/40 p-3">
                      <div className="grid grid-cols-2 gap-2.5">
                        <Field label="Local ID" error={invalidId ? 'Letters, digits and _ only' : undefined}>
                          <Input mono className="text-xs" value={term.id} placeholder="access" onChange={(e) => updateTerm(idx, { id: e.target.value })} />
                        </Field>
                        <Field label="Kind">
                          <Select value={term.kind} onChange={(e) => updateTerm(idx, { kind: e.target.value })}>
                            {TERM_KINDS.map((k) => (
                              <option key={k} value={k}>{k}</option>
                            ))}
                          </Select>
                        </Field>
                      </div>

                      <div className="grid grid-cols-2 gap-2.5">
                        <Field label="Label">
                          <Input
                            value={term.label.en ?? ''}
                            placeholder="Access"
                            onChange={(e) => updateTerm(idx, { label: { ...term.label, en: e.target.value } })}
                          />
                        </Field>
                        {term.kind === 'action' ? (
                          <Field label="Refines (included_in)">
                            <Input
                              mono
                              className="text-xs"
                              value={term.included_in ?? ''}
                              placeholder="odrl:use"
                              onChange={(e) => updateTerm(idx, { included_in: e.target.value.trim() || undefined })}
                            />
                          </Field>
                        ) : term.kind.endsWith('_subclass') ? (
                          <Field label="Subclass of">
                            <Input
                              mono
                              className="text-xs"
                              value={term.subclass_of ?? ''}
                              placeholder={DEFAULT_SUBCLASS_OF[term.kind] ?? 'odrl:Rule'}
                              onChange={(e) => updateTerm(idx, { subclass_of: e.target.value.trim() || undefined })}
                            />
                          </Field>
                        ) : (
                          <Field label="Status">
                            <Select
                              value={term.status ?? 'active'}
                              onChange={(e) => updateTerm(idx, { status: e.target.value === 'active' ? undefined : (e.target.value as DslTerm['status']) })}
                            >
                              <option value="active">active</option>
                              <option value="deprecated">deprecated</option>
                              <option value="retired">retired</option>
                            </Select>
                          </Field>
                        )}
                      </div>

                      {term.kind === 'action' || term.kind.endsWith('_subclass') ? (
                        <div className="grid grid-cols-2 gap-2.5">
                          <Field label="Status">
                            <Select
                              value={term.status ?? 'active'}
                              onChange={(e) => updateTerm(idx, { status: e.target.value === 'active' ? undefined : (e.target.value as DslTerm['status']) })}
                            >
                              <option value="active">active</option>
                              <option value="deprecated">deprecated</option>
                              <option value="retired">retired</option>
                            </Select>
                          </Field>
                          {term.status && term.status !== 'active' ? (
                            <Field label="Replaced by (CURIE)">
                              <Input mono className="text-xs" value={term.replaced_by ?? ''} placeholder={`${primaryPrefix}:newTerm`} onChange={(e) => updateTerm(idx, { replaced_by: e.target.value.trim() || undefined })} />
                            </Field>
                          ) : null}
                        </div>
                      ) : term.status && term.status !== 'active' ? (
                        <Field label="Replaced by (CURIE)">
                          <Input mono className="text-xs" value={term.replaced_by ?? ''} placeholder={`${primaryPrefix}:newTerm`} onChange={(e) => updateTerm(idx, { replaced_by: e.target.value.trim() || undefined })} />
                        </Field>
                      ) : null}

                      <Field label="Definition">
                        <Textarea
                          rows={2}
                          value={term.definition.en ?? ''}
                          placeholder="Normative meaning of this term…"
                          onChange={(e) => updateTerm(idx, { definition: { ...term.definition, en: e.target.value } })}
                        />
                      </Field>

                      {term.kind === 'left_operand' ? (
                        <SignatureEditor
                          signature={term.signature}
                          rightOperandCuries={rightOperandCuries}
                          onChange={(sig) => updateTerm(idx, { signature: sig })}
                        />
                      ) : null}
                    </div>
                  ) : null}
                </div>
              )
            })}
          </div>
        )}
      </AccordionItem>
    </Accordion>
  )
}
