// CodeMirror wrapper used for every JSON, YAML and plain-text surface, editable or read-only.
// Syntax colors come from CSS variables so the editor follows the VS Code inspired light and dark palettes.
import { useMemo } from 'react'
import { json } from '@codemirror/lang-json'
import { yaml } from '@codemirror/lang-yaml'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { EditorView } from '@codemirror/view'
import { tags as t } from '@lezer/highlight'
import CodeMirror from '@uiw/react-codemirror'
import { cn } from '@/lib/cn'
import { useIsDark } from '@/lib/settings'
import { CopyButton } from '@/components/ui/misc'

const highlight = HighlightStyle.define([
  { tag: [t.propertyName, t.definition(t.propertyName)], color: 'var(--syn-key)' },
  { tag: [t.string, t.special(t.string)], color: 'var(--syn-string)' },
  { tag: [t.number, t.integer, t.float], color: 'var(--syn-number)' },
  { tag: [t.bool, t.null, t.keyword, t.atom], color: 'var(--syn-keyword)' },
  { tag: [t.comment, t.lineComment, t.meta], color: 'var(--syn-comment)' },
  { tag: [t.punctuation, t.separator, t.bracket, t.squareBracket, t.brace], color: 'var(--syn-punct)' },
])

// Language is the syntax mode of the editor.
export type Language = 'json' | 'yaml' | 'text'

// guessLanguage picks a syntax mode from a media type or the content itself.
export function guessLanguage(content: string, mediaType = ''): Language {
  if (mediaType.includes('json')) return 'json'
  if (mediaType.includes('yaml')) return 'yaml'
  const s = content.trimStart()
  if (s.startsWith('{') || s.startsWith('[')) return 'json'
  if (/^[\w-]+:\s/m.test(s) && !s.startsWith('@prefix')) return 'yaml'
  return 'text'
}

// CodeEditor renders an editable or read-only code surface that fills its container.
export function CodeEditor({
  value,
  onChange,
  language = 'json',
  readOnly,
  className,
  minHeight,
  maxHeight,
  placeholder,
  copy,
}: {
  value: string
  onChange?: (value: string) => void
  language?: Language
  readOnly?: boolean
  className?: string
  minHeight?: string
  maxHeight?: string
  placeholder?: string
  copy?: boolean
}) {
  const dark = useIsDark()
  const extensions = useMemo(
    () => [EditorView.theme({}, { dark }), syntaxHighlighting(highlight), EditorView.lineWrapping, ...(language === 'json' ? [json()] : language === 'yaml' ? [yaml()] : [])],
    [language, dark],
  )
  return (
    <div className={cn('group/code relative min-h-0 overflow-hidden rounded-md border bg-bg', className)}>
      <CodeMirror
        value={value}
        onChange={onChange}
        extensions={extensions}
        theme="none"
        readOnly={readOnly}
        editable={!readOnly}
        placeholder={placeholder}
        minHeight={minHeight}
        maxHeight={maxHeight}
        height={minHeight || maxHeight ? undefined : '100%'}
        style={{ height: minHeight || maxHeight ? undefined : '100%' }}
        basicSetup={{ foldGutter: true, highlightActiveLine: !readOnly, highlightActiveLineGutter: !readOnly, autocompletion: false, searchKeymap: true }}
      />
      {copy ? <CopyButton value={value} className="absolute top-1.5 right-1.5 bg-raised opacity-0 shadow-sm group-hover/code:opacity-100" /> : null}
    </div>
  )
}
