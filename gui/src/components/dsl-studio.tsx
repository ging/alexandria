// Bidirectional authoring studio for alexandria.profiles/v1 OdrlProfile and SpaceProfile bundles.
// Supports Form-only, Split and Code-only layouts while keeping structured accordions and YAML in sync.
import { useEffect, useRef, useState } from 'react'
import { AlertTriangle, BookOpen, Code2, Columns2, Plus, SlidersHorizontal } from 'lucide-react'
import { cn } from '@/lib/cn'
import { hasOpenDialog, isEditableTarget, useWindowKeydown } from '@/lib/hotkeys'
import {
  defaultOdrlProfile,
  parseDslBundle,
  serializeDslBundle,
  type DslBundle,
  type OdrlProfileDoc,
  type SpaceProfileDoc,
} from '@/lib/dsl-bundle'
import { CodeEditor } from '@/components/code-editor'
import { OdrlProfileForm } from '@/components/dsl-odrl-form'
import { SpaceProfileForm } from '@/components/dsl-space-form'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/form'
import { Badge, Empty, Tabs } from '@/components/ui/misc'

// ActiveDoc selects which profile kind is focused in the form pane.
type ActiveDoc = 'odrl' | 'space'

// StudioMode selects Form-only, Split or Code-only layout.
type StudioMode = 'form' | 'split' | 'code'

// ModeSwitch renders the compact Form / Split / Code layout selector.
function ModeSwitch({ value, onChange }: { value: StudioMode; onChange: (next: StudioMode) => void }) {
  const items: { id: StudioMode; label: string; icon: typeof Columns2 }[] = [
    { id: 'form', label: 'Form', icon: SlidersHorizontal },
    { id: 'split', label: 'Split', icon: Columns2 },
    { id: 'code', label: 'Code', icon: Code2 },
  ]
  return (
    <div className="inline-flex h-6 shrink-0 items-center rounded-md border border-line-strong bg-bg p-0.5">
      {items.map(({ id, label, icon: Icon }) => (
        <button
          key={id}
          type="button"
          title={`${label} view (${id === 'form' ? 'f' : id === 'split' ? 's' : 'y'})`}
          onClick={() => onChange(id)}
          className={cn(
            'inline-flex h-5 items-center gap-1 rounded-sm px-1.5 text-2xs font-medium transition-colors',
            value === id ? 'bg-accent-soft text-accent' : 'text-muted hover:text-fg',
          )}
        >
          <Icon className="size-3" />
          {label}
        </button>
      ))}
    </div>
  )
}

// DslStudio renders the synchronized form + YAML authoring workspace.
export function DslStudio({
  value,
  onChange,
  base,
  slug,
  className,
}: {
  value: string
  onChange: (next: string) => void
  base: string
  slug: string
  className?: string
}) {
  const effectiveBase = base || 'https://hub.example.com'
  const lastSeenRef = useRef<string>(value)
  const initialParsed = parseDslBundle(value)

  const [bundle, setBundle] = useState<DslBundle>(
    () => initialParsed.bundle ?? { odrls: [defaultOdrlProfile(effectiveBase, slug)], space: null },
  )
  const [parseError, setParseError] = useState<string | null>(initialParsed.error)
  const [docTab, setDocTab] = useState<ActiveDoc>('odrl')
  const [odrlIdx, setOdrlIdx] = useState(0)
  const [mode, setMode] = useState<StudioMode>('split')

  useWindowKeydown((e) => {
    if (hasOpenDialog() || isEditableTarget(e.target) || e.metaKey || e.ctrlKey || e.altKey) return
    if (e.key === '1') { e.preventDefault(); setDocTab('odrl'); if (mode === 'code') setMode('split') }
    else if (e.key === '2') { e.preventDefault(); setDocTab('space'); if (mode === 'code') setMode('split') }
    else if (e.key === 'f') { e.preventDefault(); setMode('form') }
    else if (e.key === 's') { e.preventDefault(); setMode('split') }
    else if (e.key === 'y') { e.preventDefault(); setMode('code') }
  })

  useEffect(() => {
    if (value === lastSeenRef.current) return
    lastSeenRef.current = value
    const res = parseDslBundle(value)
    if (res.bundle) {
      setBundle(res.bundle)
      setParseError(null)
    } else {
      setParseError(res.error)
    }
  }, [value])

  const commitBundle = (next: DslBundle) => {
    const yaml = serializeDslBundle(next)
    lastSeenRef.current = yaml
    setBundle(next)
    setParseError(null)
    onChange(yaml)
  }

  const handleCodeChange = (nextYaml: string) => {
    lastSeenRef.current = nextYaml
    const res = parseDslBundle(nextYaml)
    if (res.bundle) {
      setBundle(res.bundle)
      setParseError(null)
    } else {
      setParseError(res.error)
    }
    onChange(nextYaml)
  }

  const clampedOdrlIdx = Math.min(odrlIdx, Math.max(0, bundle.odrls.length - 1))
  const currentOdrl: OdrlProfileDoc | undefined = bundle.odrls[clampedOdrlIdx]

  const updateOdrl = (idx: number, nextDoc: OdrlProfileDoc) => {
    const prevDoc = bundle.odrls[idx]
    const nextOdrls = bundle.odrls.map((d, i) => (i === idx ? nextDoc : d))
    let nextSpace: SpaceProfileDoc | null = bundle.space
    // Keep internal ODRL bindings in SpaceProfile aligned when the ODRL profile ID or SemVer changes.
    if (nextSpace && prevDoc && (prevDoc.profile.id !== nextDoc.profile.id || prevDoc.profile.version !== nextDoc.profile.version)) {
      nextSpace = {
        ...nextSpace,
        bindings: nextSpace.bindings.map((b) => {
          if (b.source !== 'internal' || b.profile !== prevDoc.profile.id) return b
          return {
            ...b,
            profile: nextDoc.profile.id,
            version: b.version === `=${prevDoc.profile.version}` ? `=${nextDoc.profile.version}` : b.version,
          }
        }),
      }
    }
    commitBundle({ odrls: nextOdrls, space: nextSpace })
  }

  const addOdrl = () => {
    const created = defaultOdrlProfile(effectiveBase, slug, bundle.odrls.length + 1)
    const nextOdrls = [...bundle.odrls, created]
    setOdrlIdx(nextOdrls.length - 1)
    setDocTab('odrl')
    commitBundle({ ...bundle, odrls: nextOdrls })
  }

  return (
    <div
      className={cn(
        'grid h-full min-h-0 overflow-hidden rounded-md border bg-bg',
        mode === 'split' ? 'grid-cols-1 md:grid-cols-2' : 'grid-cols-1',
        className,
      )}
    >
      {mode !== 'code' ? (
        <div className={cn('flex min-h-0 min-w-0 flex-col', mode === 'split' && 'border-b md:border-r md:border-b-0')}>
          <Tabs
            className="px-4"
            value={docTab}
            onChange={setDocTab}
            tabs={[
              { id: 'odrl', label: 'ODRL Profile' },
              { id: 'space', label: 'Space Profile' },
            ]}
            trailing={
              <>
                {docTab === 'odrl' && bundle.odrls.length > 1 ? (
                  <Select
                    className="w-32 [&>select]:h-7 [&>select]:font-mono [&>select]:text-xs"
                    value={String(clampedOdrlIdx)}
                    onChange={(e) => setOdrlIdx(Number(e.target.value))}
                  >
                    {bundle.odrls.map((od, i) => (
                      <option key={i} value={i}>
                        {od.profile.id.split('/').pop() || `odrl-${i + 1}`} ({od.profile.version})
                      </option>
                    ))}
                  </Select>
                ) : null}
                {mode === 'form' ? <ModeSwitch value={mode} onChange={setMode} /> : null}
              </>
            }
          />

          <div className="min-h-0 flex-1 overflow-y-auto p-4">
            <div className={cn(mode === 'form' && 'mx-auto max-w-2xl')}>
              {parseError ? (
                <div className="mb-3 flex items-start gap-2 rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-xs text-danger">
                  <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
                  <span className="font-mono">{parseError}</span>
                </div>
              ) : null}

              {docTab === 'odrl' ? (
                currentOdrl ? (
                  <OdrlProfileForm doc={currentOdrl} onChange={(nextDoc) => updateOdrl(clampedOdrlIdx, nextDoc)} />
                ) : (
                  <Empty
                    icon={<BookOpen />}
                    title="No OdrlProfile in this stream"
                    action={
                      <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={addOdrl}>
                        Initialize OdrlProfile
                      </Button>
                    }
                  >
                    Add an internal ODRL 2.2 profile to govern actions, left operands and custom vocabulary terms.
                  </Empty>
                )
              ) : (
                <SpaceProfileForm
                  space={bundle.space}
                  odrls={bundle.odrls}
                  base={effectiveBase}
                  slug={slug}
                  onChange={(nextSpace) => commitBundle({ ...bundle, space: nextSpace })}
                />
              )}
            </div>
          </div>
        </div>
      ) : null}

      {mode !== 'form' ? (
        <div className="flex min-h-0 min-w-0 flex-col">
          <div className="flex h-9 shrink-0 items-center justify-between gap-2 border-b px-4">
            <div className="flex min-w-0 items-center gap-2">
              <span className="label-caps">DSL</span>
              {parseError ? (
                <Badge tone="danger" dot>Syntax error</Badge>
              ) : (
                <span className="truncate font-mono text-xs text-subtle">alexandria.profiles/v1</span>
              )}
            </div>
            <ModeSwitch value={mode} onChange={setMode} />
          </div>
          <div className="min-h-0 flex-1 p-3">
            <CodeEditor value={value} onChange={handleCodeChange} language="yaml" copy className="h-full" />
          </div>
        </div>
      ) : null}
    </div>
  )
}
