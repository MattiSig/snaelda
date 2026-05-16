import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { APIError, restoreWorkspace } from '@/lib/api'
import { cn } from '@/lib/utils'
import { layout, paddedPanel, text } from '@/lib/styles'

export const Route = createFileRoute('/restore')({
  validateSearch: (search: Record<string, unknown>) => ({
    k: typeof search.k === 'string' ? search.k : '',
  }),
  component: RestoreRoute,
})

function RestoreRoute() {
  const navigate = useNavigate()
  const search = Route.useSearch()
  const [message, setMessage] = useState('')

  useEffect(() => {
    if (!search.k) {
      return
    }

    restoreWorkspace(search.k)
      .then(() => navigate({ to: '/app' }))
      .catch((error) => {
        setMessage(
          error instanceof APIError ? error.message : 'Could not restore that workspace',
        )
      })
  }, [navigate, search.k])

  const visibleMessage = message || (search.k
    ? 'Restoring your workspace...'
    : 'Workspace recovery link is missing a key.')

  return (
    <main className={cn(layout.pageShell, layout.narrowShell, 'pt-14')}>
      <section className={paddedPanel}>
        <p className={text.eyebrow}>Workspace recovery</p>
        <h1 className={cn(text.h2, 'mt-2')}>{visibleMessage}</h1>
      </section>
    </main>
  )
}
