import { Link, Outlet, createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import {
  APIError,
  type AuthSession,
  getCurrentSession,
  logout,
} from '@/lib/api'

export const Route = createFileRoute('/app')({
  component: AppLayout,
})

function AppLayout() {
  const [session, setSession] = useState<AuthSession | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let isMounted = true

    getCurrentSession()
      .then((nextSession) => {
        if (isMounted) {
          setSession(nextSession)
        }
      })
      .catch((error) => {
        if (error instanceof APIError && error.status === 401) {
          const redirect = encodeURIComponent(window.location.pathname)
          window.location.href = `/login?redirect=${redirect}`
          return
        }
        if (isMounted) {
          setErrorMessage('Could not load your session')
        }
      })

    return () => {
      isMounted = false
    }
  }, [])

  async function handleSignOut() {
    await logout()
    window.location.href = '/login'
  }

  if (errorMessage) {
    return (
      <main className="app-shell">
        <section className="app-content">
          <div className="builder-panel">
            <p className="form-error">{errorMessage}</p>
          </div>
        </section>
      </main>
    )
  }

  if (!session) {
    return (
      <main className="app-shell">
        <section className="app-content">
          <div className="builder-panel">
            <p>Loading session...</p>
          </div>
        </section>
      </main>
    )
  }

  return (
    <main className="app-shell">
      <aside className="app-sidebar">
        <div className="app-user">
          <span>{session.user.name || session.user.email}</span>
          <small>{session.user.workspaceRole}</small>
        </div>
        <Link to="/app">Sites</Link>
        <Link to="/app/sites/$siteId" params={{ siteId: 'draft' }}>
          Draft
        </Link>
        <Link to="/app/sites/$siteId/preview" params={{ siteId: 'draft' }}>
          Preview
        </Link>
        <button type="button" onClick={handleSignOut}>
          Sign out
        </button>
      </aside>
      <section className="app-content">
        <Outlet />
      </section>
    </main>
  )
}
