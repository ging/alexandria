// Entry point of the Alexandria console: mounts React with the query client, router, tooltips and toasts.
// Query defaults favor a live operations tool: short staleness, refetch on focus and no retry on client errors.
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { Tooltip } from 'radix-ui'
import { Toaster } from 'sonner'
import { ApiError } from '@/lib/http'
import { applyTheme, getSettings, useIsDark } from '@/lib/settings'
import { router } from '@/router'
import './styles.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5_000,
      retry: (count, error) => !(error instanceof ApiError && (error.status === 0 || (error.status >= 400 && error.status < 500))) && count < 2,
    },
  },
})

applyTheme(getSettings().theme)

// App wires the global providers.
function App() {
  const dark = useIsDark()
  return (
    <QueryClientProvider client={queryClient}>
      <Tooltip.Provider delayDuration={250}>
        <RouterProvider router={router} />
        <Toaster theme={dark ? 'dark' : 'light'} position="bottom-right" offset={16} toastOptions={{ style: { fontSize: 13, fontFamily: 'var(--font-sans)', background: 'var(--bg-raised)', color: 'var(--fg)', border: '1px solid var(--border-strong)' } }} />
      </Tooltip.Provider>
    </QueryClientProvider>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
