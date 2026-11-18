// Accordion-based form controls for authoring an alexandria.profiles/v1 SpaceProfile document.
// Organizes space identity, bindings and lifecycle overrides into collapsible scannable panels.
import { useState } from 'react'
import { ChevronDown, ChevronRight, Layers, Plus, Trash2, X } from 'lucide-react'
import {
  CLIENT_GATES,
  CONFORMANCE_ROLES,
  defaultSpaceProfile,
  GATES,
  SLOTS,
  type DslBinding,
  type DslConformance,
  type DslLifecycle,
  type OdrlProfileDoc,
  type SpaceProfileDoc,
} from '@/lib/dsl-bundle'
import { Accordion, AccordionItem } from '@/components/dsl-odrl-form'
import { Button, IconButton } from '@/components/ui/button'
import { Field, Input, Select, Textarea } from '@/components/ui/form'
import { Badge, Empty } from '@/components/ui/misc'

// ALL_GATE_PAIRS enumerates every valid (role, gate) conformance pair for the compact gate selector.
const ALL_GATE_PAIRS: DslConformance[] = CONFORMANCE_ROLES.flatMap((role) =>
  GATES.map((applies_to) => ({ role, applies_to })),
)

// slotTone maps a specification slot name to a subtle semantic badge tone.
function slotTone(slot: string): 'accent' | 'info' | 'purple' | 'success' | 'neutral' {
  if (slot === 'odrl') return 'accent'
  if (slot === 'dsp') return 'info'
  if (slot === 'dcat') return 'purple'
  if (slot === 'dcp') return 'success'
  return 'neutral'
}

// sameConf checks whether two conformance items target the same (role, gate) pair.
function sameConf(a: DslConformance, b: DslConformance): boolean {
  return a.role === b.role && a.applies_to === b.applies_to
}

// SpaceSectionId identifies one collapsible accordion panel in SpaceProfileForm.
type SpaceSectionId = 'space' | 'bindings' | 'lifecycle'

// SpaceProfileForm renders the accordion-based visual authoring surface for SpaceProfileDoc.
export function SpaceProfileForm({
  space,
  odrls,
  base,
  slug,
  onChange,
}: {
  space: SpaceProfileDoc | null
  odrls: OdrlProfileDoc[]
  base: string
  slug: string
  onChange: (next: SpaceProfileDoc) => void
}) {
  const [openSec, setOpenSec] = useState<Record<SpaceSectionId, boolean>>({
    space: true,
    bindings: true,
    lifecycle: false,
  })
  const [showDesc, setShowDesc] = useState(Boolean(space?.space.description?.en))
  const [openBinding, setOpenBinding] = useState<number | null>(null)

  const toggleSec = (id: SpaceSectionId) => setOpenSec((prev) => ({ ...prev, [id]: !prev[id] }))

  if (!space) {
    const firstOdrl = odrls[0]?.profile.id ?? `${base}/odrl/profile/${slug}`
    return (
      <Empty
        icon={<Layers />}
        title="No SpaceProfile in this stream"
        action={
          <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => onChange(defaultSpaceProfile(base, slug, firstOdrl))}>
            Initialize SpaceProfile
          </Button>
        }
      >
        Every hub release requires one SpaceProfile binding the governed ODRL profile and any vendored protocol specs.
      </Empty>
    )
  }

  const setMeta = (patch: Partial<SpaceProfileDoc['space']>) =>
    onChange({ ...space, space: { ...space.space, ...patch } })

  const usedSlots = new Set(space.bindings.map((b) => b.slot))
  const freeSlots = SLOTS.filter((s) => !usedSlots.has(s))

  const updateBinding = (idx: number, patch: Partial<DslBinding>) => {
    const next = space.bindings.map((b, i) => {
      if (i !== idx) return b
      const merged: DslBinding = { ...b, ...patch }
      // Internal source is restricted to the odrl slot by N2 validation.
      if (merged.source === 'internal' && merged.slot !== 'odrl') merged.slot = 'odrl'
      if (merged.source === 'internal') merged.via = undefined
      return merged
    })
    onChange({ ...space, bindings: next })
  }

  const removeBinding = (idx: number) => {
    if (openBinding === idx) setOpenBinding(null)
    onChange({ ...space, bindings: space.bindings.filter((_, i) => i !== idx) })
  }

  const addBinding = () => {
    const nextSlot = freeSlots[0]
    if (!nextSlot) return
    let created: DslBinding
    if (nextSlot === 'odrl') {
      const target = odrls[0]
      created = {
        slot: 'odrl',
        profile: target?.profile.id || `${base}/odrl/profile/${slug}`,
        source: 'internal',
        version: `=${target?.profile.version || '1.0.0'}`,
        obligation: 'mandatory',
        conformance: [
          { role: 'json-schema', applies_to: 'policy_intake' },
          { role: 'shacl', applies_to: 'policy_intake' },
        ],
      }
    } else if (nextSlot === 'dsp') {
      created = {
        slot: 'dsp',
        profile: 'https://w3id.org/dspace/2025/1',
        source: 'vendored',
        version: '=2025-1',
        obligation: 'mandatory',
        conformance: [{ role: 'json-schema', applies_to: 'dsp_message' }],
      }
    } else {
      const gate = nextSlot === 'dcat' ? 'catalog_entry' : 'credential_presentation'
      created = {
        slot: nextSlot,
        profile: nextSlot === 'dcat' ? 'https://www.w3.org/ns/dcat' : 'https://w3id.org/dspace-dcp/v1.0',
        source: 'vendored',
        version: '=1.0.0',
        obligation: 'optional',
        conformance: [{ role: 'json-schema', applies_to: gate }],
      }
    }
    setOpenSec((prev) => ({ ...prev, bindings: true }))
    setOpenBinding(space.bindings.length)
    onChange({ ...space, bindings: [...space.bindings, created] })
  }

  const hasLifecycle = Boolean(space.lifecycle && Object.keys(space.lifecycle).length > 0)
  const lc: DslLifecycle = space.lifecycle ?? {}

  const patchLifecycle = (patch: Partial<DslLifecycle>) => {
    const merged: DslLifecycle = { ...lc, ...patch }
    const cleaned: DslLifecycle = {}
    if (merged.propagation_window?.trim()) cleaned.propagation_window = merged.propagation_window.trim()
    if (merged.grace_window?.trim()) cleaned.grace_window = merged.grace_window.trim()
    if (merged.sync_interval?.trim()) cleaned.sync_interval = merged.sync_interval.trim()
    if (merged.clock_skew?.trim()) cleaned.clock_skew = merged.clock_skew.trim()
    if (merged.lease_margin?.trim()) cleaned.lease_margin = merged.lease_margin.trim()
    if (merged.max_concurrent_versions && merged.max_concurrent_versions >= 1) cleaned.max_concurrent_versions = merged.max_concurrent_versions
    if (merged.conformance_required) cleaned.conformance_required = merged.conformance_required
    if (merged.freshness?.ttl?.trim() || merged.freshness?.max_staleness?.trim()) {
      cleaned.freshness = {
        ...(merged.freshness?.ttl?.trim() ? { ttl: merged.freshness.ttl.trim() } : {}),
        ...(merged.freshness?.max_staleness?.trim() ? { max_staleness: merged.freshness.max_staleness.trim() } : {}),
      }
    }
    if (merged.degraded && Object.keys(merged.degraded).length > 0) cleaned.degraded = merged.degraded
    onChange({ ...space, lifecycle: Object.keys(cleaned).length > 0 ? cleaned : undefined })
  }

  return (
    <Accordion>
      <AccordionItem
        title="Space"
        summary={`${space.space.version}${space.space.title.en ? ` · ${space.space.title.en}` : ''}`}
        open={openSec.space}
        onToggle={() => toggleSec('space')}
      >
        <div className="space-y-3">
          <div className="grid grid-cols-[1fr_112px] gap-2.5">
            <Field label="Title">
              <Input
                value={space.space.title.en ?? ''}
                placeholder="Default data space"
                onChange={(e) => setMeta({ title: { ...space.space.title, en: e.target.value } })}
              />
            </Field>
            <Field label="Version">
              <Input mono className="text-xs" value={space.space.version} placeholder="1.0.0" onChange={(e) => setMeta({ version: e.target.value })} />
            </Field>
          </div>

          <Field label="Space IRI">
            <Input mono className="text-xs" value={space.space.id} placeholder="https://hub.example.com/spaces/default" onChange={(e) => setMeta({ id: e.target.value })} />
          </Field>

          <Field label="Operator IRI">
            <Input mono className="text-xs" value={space.space.operator} placeholder="https://hub.example.com/party/authority" onChange={(e) => setMeta({ operator: e.target.value })} />
          </Field>

          <Field label="Publication base URL">
            <Input mono className="text-xs" value={space.space.base_url} placeholder="https://hub.example.com/pub/default" onChange={(e) => setMeta({ base_url: e.target.value })} />
          </Field>

          {showDesc ? (
            <Field label="Description (optional)">
              <Textarea
                rows={2}
                value={space.space.description?.en ?? ''}
                placeholder="Governance scope and normative profile composition…"
                onChange={(e) => setMeta({ description: e.target.value ? { ...space.space.description, en: e.target.value } : undefined })}
              />
            </Field>
          ) : (
            <button type="button" onClick={() => setShowDesc(true)} className="text-xs text-muted hover:text-fg">
              + Description…
            </button>
          )}
        </div>
      </AccordionItem>

      <AccordionItem
        title="Bindings"
        summary={space.bindings.map((b) => b.slot).join(', ') || 'none'}
        open={openSec.bindings}
        onToggle={() => toggleSec('bindings')}
        action={
          <Button size="sm" variant="ghost" icon={<Plus className="size-3.5" />} disabled={freeSlots.length === 0} onClick={addBinding}>
            Add binding
          </Button>
        }
      >
        <div className="divide-y rounded-md border bg-raised">
          {space.bindings.map((b, idx) => {
            const open = openBinding === idx
            const missingEq = b.source === 'vendored' && b.version.length > 0 && !b.version.startsWith('=')
            const unlinkedOdrl = b.source === 'internal' && odrls.length > 0 && !odrls.some((o) => o.profile.id === b.profile)
            return (
              <div key={idx}>
                <div
                  role="button"
                  tabIndex={0}
                  onClick={() => setOpenBinding(open ? null : idx)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      setOpenBinding(open ? null : idx)
                    }
                  }}
                  className="flex h-9 cursor-pointer items-center gap-2 px-3 select-none hover:bg-hover/60"
                >
                  {open ? <ChevronDown className="size-3.5 shrink-0 text-subtle" /> : <ChevronRight className="size-3.5 shrink-0 text-subtle" />}
                  <Badge tone={slotTone(b.slot)}>{b.slot}</Badge>
                  <span className="font-mono text-xs text-muted">{b.version}</span>
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-subtle">{b.profile}</span>
                  <Badge tone={b.obligation === 'mandatory' ? 'neutral' : 'info'}>{b.obligation}</Badge>
                  {space.bindings.length > 1 ? (
                    <div onClick={(e) => e.stopPropagation()}>
                      <IconButton size="sm" label="Remove binding" onClick={() => removeBinding(idx)}>
                        <Trash2 className="size-3.5" />
                      </IconButton>
                    </div>
                  ) : null}
                </div>

                {open ? (
                  <div className="space-y-3 border-t bg-side/40 p-3">
                    <div className="grid grid-cols-2 gap-2.5">
                      <Field label="Slot">
                        <Select
                          value={b.slot}
                          onChange={(e) => {
                            const nextSlot = e.target.value
                            updateBinding(idx, {
                              slot: nextSlot,
                              ...(nextSlot !== 'odrl' && b.source === 'internal'
                                ? { source: 'vendored', version: b.version.startsWith('=') ? b.version : `=${b.version}` }
                                : {}),
                            })
                          }}
                        >
                          {SLOTS.map((s) => (
                            <option key={s} value={s} disabled={s !== b.slot && usedSlots.has(s)}>
                              {s}
                            </option>
                          ))}
                        </Select>
                      </Field>
                      <Field label="Source">
                        <Select
                          value={b.source}
                          onChange={(e) => {
                            const src = e.target.value as DslBinding['source']
                            updateBinding(idx, {
                              source: src,
                              ...(src === 'vendored' && !b.version.startsWith('=') ? { version: `=${b.version || '1.0.0'}` } : {}),
                            })
                          }}
                        >
                          <option value="internal" disabled={b.slot !== 'odrl'}>internal</option>
                          <option value="vendored">vendored</option>
                        </Select>
                      </Field>
                    </div>

                    <div className="grid grid-cols-2 gap-2.5">
                      <Field label="Obligation">
                        <Select value={b.obligation} onChange={(e) => updateBinding(idx, { obligation: e.target.value as DslBinding['obligation'] })}>
                          <option value="mandatory">mandatory</option>
                          <option value="recommended">recommended</option>
                          <option value="optional">optional</option>
                        </Select>
                      </Field>
                      <Field label="Version" error={missingEq ? 'Use =version' : undefined}>
                        <Input mono className="text-xs" value={b.version} placeholder="=1.0.0" onChange={(e) => updateBinding(idx, { version: e.target.value })} />
                      </Field>
                    </div>

                    <Field
                      label="Profile IRI"
                      hint={
                        unlinkedOdrl ? (
                          <button
                            type="button"
                            onClick={() => updateBinding(idx, { profile: odrls[0].profile.id, version: `=${odrls[0].profile.version}` })}
                            className="text-accent hover:underline"
                          >
                            Link to {odrls[0].profile.id}
                          </button>
                        ) : undefined
                      }
                    >
                      <Input mono className="text-xs" value={b.profile} placeholder="https://w3id.org/dspace/2025/1" onChange={(e) => updateBinding(idx, { profile: e.target.value })} />
                    </Field>

                    <div>
                      <div className="mb-1.5 flex items-center justify-between gap-2">
                        <span className="text-xs font-medium text-muted">Conformance gates</span>
                        <Select
                          className="w-44 shrink-0 [&>select]:h-7 [&>select]:text-xs"
                          value=""
                          onChange={(e) => {
                            if (!e.target.value) return
                            const [role, applies_to] = e.target.value.split(':')
                            updateBinding(idx, { conformance: [...b.conformance, { role, applies_to }] })
                          }}
                        >
                          <option value="">+ Add gate…</option>
                          {ALL_GATE_PAIRS.filter((p) => !b.conformance.some((c) => sameConf(c, p))).map((p) => (
                            <option key={`${p.role}:${p.applies_to}`} value={`${p.role}:${p.applies_to}`}>
                              {p.role} → {p.applies_to}
                            </option>
                          ))}
                        </Select>
                      </div>
                      {b.conformance.length === 0 ? (
                        <p className="text-xs text-danger">At least one gate is required when obligation is mandatory.</p>
                      ) : (
                        <div className="flex flex-wrap gap-1.5">
                          {b.conformance.map((c) => (
                            <span
                              key={`${c.role}:${c.applies_to}`}
                              className="inline-flex h-6 items-center gap-1 rounded-sm border border-line-strong bg-bg px-2 font-mono text-xs text-fg"
                            >
                              {c.role} → {c.applies_to}
                              <button
                                type="button"
                                onClick={() => updateBinding(idx, { conformance: b.conformance.filter((x) => !sameConf(x, c)) })}
                                className="text-subtle hover:text-danger"
                                aria-label="Remove gate"
                              >
                                <X className="size-3" />
                              </button>
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>
                ) : null}
              </div>
            )
          })}
        </div>
      </AccordionItem>

      <AccordionItem
        title="Lifecycle"
        summary={hasLifecycle ? 'custom' : 'defaults'}
        open={openSec.lifecycle}
        onToggle={() => toggleSec('lifecycle')}
        action={
          hasLifecycle ? (
            <Button size="sm" variant="ghost" onClick={() => onChange({ ...space, lifecycle: undefined })}>
              Use defaults
            </Button>
          ) : (
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setOpenSec((prev) => ({ ...prev, lifecycle: true }))
                onChange({ ...space, lifecycle: { grace_window: 'P30D', sync_interval: 'PT60S' } })
              }}
            >
              Customize
            </Button>
          )
        }
      >
        {!hasLifecycle ? (
          <p className="text-xs text-muted">
            Inherits propagation window, grace window and listing freshness from the namespace.
          </p>
        ) : (
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-2.5">
              <Field label="Grace window">
                <Input mono className="text-xs" value={lc.grace_window ?? ''} placeholder="P30D" onChange={(e) => patchLifecycle({ grace_window: e.target.value })} />
              </Field>
              <Field label="Propagation window">
                <Input mono className="text-xs" value={lc.propagation_window ?? ''} placeholder="PT5M" onChange={(e) => patchLifecycle({ propagation_window: e.target.value })} />
              </Field>
              <Field label="Sync interval">
                <Input mono className="text-xs" value={lc.sync_interval ?? ''} placeholder="PT60S" onChange={(e) => patchLifecycle({ sync_interval: e.target.value })} />
              </Field>
              <Field label="Freshness TTL">
                <Input
                  mono
                  className="text-xs"
                  value={lc.freshness?.ttl ?? ''}
                  placeholder="PT15M"
                  onChange={(e) => patchLifecycle({ freshness: { ...lc.freshness, ttl: e.target.value } })}
                />
              </Field>
              <Field label="Max staleness">
                <Input
                  mono
                  className="text-xs"
                  value={lc.freshness?.max_staleness ?? ''}
                  placeholder="PT2H"
                  onChange={(e) => patchLifecycle({ freshness: { ...lc.freshness, max_staleness: e.target.value } })}
                />
              </Field>
              <Field label="Max concurrent versions">
                <Input
                  mono
                  className="text-xs"
                  type="number"
                  min={1}
                  value={lc.max_concurrent_versions ?? ''}
                  placeholder="2"
                  onChange={(e) => patchLifecycle({ max_concurrent_versions: e.target.value ? Number(e.target.value) : undefined })}
                />
              </Field>
            </div>

            <div className="divide-y border-t pt-2">
              {CLIENT_GATES.slice(0, 4).map((gate) => (
                <div key={gate} className="flex items-center justify-between gap-3 py-1.5">
                  <span className="font-mono text-xs text-muted">{gate}</span>
                  <Select
                    className="w-32 shrink-0 [&>select]:h-7 [&>select]:text-xs"
                    value={lc.degraded?.[gate] ?? ''}
                    onChange={(e) => {
                      const nextDeg = { ...(lc.degraded ?? {}) }
                      if (!e.target.value) delete nextDeg[gate]
                      else nextDeg[gate] = e.target.value as 'continue' | 'reject'
                      patchLifecycle({ degraded: Object.keys(nextDeg).length ? nextDeg : undefined })
                    }}
                  >
                    <option value="">default</option>
                    <option value="continue">continue</option>
                    <option value="reject">reject</option>
                  </Select>
                </div>
              ))}
            </div>
          </div>
        )}
      </AccordionItem>
    </Accordion>
  )
}
