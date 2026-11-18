// Shared keyboard-navigation helpers and hooks for safe, conflict-free console shortcuts.
// Prevents single-key hotkeys from firing while typing in inputs, selects or CodeMirror editors, or behind modal dialogs.
import { useEffect, useRef } from 'react'

// isEditableTarget reports whether an event target accepts text input or owns CodeMirror focus.
export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false
  if (target.closest('.cm-editor')) return true
  const tag = target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return target instanceof HTMLElement && target.isContentEditable
}

// hasOpenDialog reports whether a Radix modal dialog is currently mounted over the workspace.
export function hasOpenDialog(): boolean {
  return document.querySelector('[role="dialog"]') !== null
}

// useWindowKeydown attaches a stable window keydown listener that always sees the latest handler.
export function useWindowKeydown(handler: (e: KeyboardEvent) => void): void {
  const ref = useRef(handler)
  ref.current = handler
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => ref.current(e)
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}
