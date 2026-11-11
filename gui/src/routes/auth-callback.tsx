// OAuth 2.0 redirect endpoint of the console: redeems the authorization code and returns to the view that started sign-in.
// Rendered outside the application shell so a failed callback shows a focused error instead of a broken workspace.
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ShieldAlert } from 'lucide-react'
import { toast } from 'sonner'
import { completeLogin } from '@/lib/oidc'
import { Button } from '@/components/ui/button'
import { Empty, Spinner } from '@/components/ui/misc'

// AuthCallbackPage completes Authorization Code + PKCE exactly once.
export function AuthCallbackPage() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [error, setError] = useState<string | null>(null)
  const started = useRef(false)
  useEffect(() => {
    // Guards against the double effect invocation of StrictMode: a code can only be redeemed once.
    if (started.current) return
    started.current = true
    completeLogin(location.search)
      .then((returnTo) => {
        void qc.invalidateQueries({ queryKey: ['hub'] })
        toast.success('Signed in')
        void navigate({ to: returnTo, replace: true })
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
  }, [navigate, qc])
  return (
    <div className="flex h-full items-center justify-center bg-bg">
      {error ? (
        <Empty icon={<ShieldAlert />} title="Sign-in failed" action={<Button variant="primary" onClick={() => void navigate({ to: '/', replace: true })}>Back to the console</Button>}>
          {error}
        </Empty>
      ) : (
        <div className="flex items-center gap-2 text-sm text-muted"><Spinner /> Completing sign-in…</div>
      )}
    </div>
  )
}
