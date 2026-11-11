// Small React hooks shared by routed views: namespace route parameter, ticking clock and mutation feedback.
// They keep page components focused on layout and data wiring.
import { useEffect, useState } from 'react'
import { useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query'
import { useParams } from '@tanstack/react-router'
import { toast } from 'sonner'
import { toastError } from '@/components/problem'

// useNs returns the namespace slug of the current route.
export function useNs(): string {
  return (useParams({ strict: false }) as { ns?: string }).ns ?? ''
}

// useNow returns the current time, refreshed on an interval for relative labels.
export function useNow(intervalMs = 5000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
  return now
}

// useAction wraps a mutation with success toast, error toast and query invalidation.
export function useAction<V, R>(fn: (vars: V) => Promise<R>, o: { success?: string | ((r: R) => string); invalidate?: QueryKey[]; onSuccess?: (r: R, vars: V) => void } = {}) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: (r, vars) => {
      for (const key of o.invalidate ?? []) void qc.invalidateQueries({ queryKey: key })
      if (o.success) toast.success(typeof o.success === 'function' ? o.success(r) : o.success)
      o.onSuccess?.(r, vars)
    },
    onError: toastError,
  })
}
