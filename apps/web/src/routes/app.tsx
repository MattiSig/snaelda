import { Link, Outlet, createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  APIError,
  type AuthSession,
  getCurrentSession,
  logout,
} from '@/lib/api'
import { cn } from '@/lib/utils'
import { layout, paddedPanel, text } from '@/lib/styles'

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
      <main className={cn(layout.pageShell, layout.narrowShell, 'pt-14')}>
        <section className={paddedPanel}>
          <p className={text.eyebrow}>Builder unavailable</p>
          <h1 className={cn(text.h2, 'mt-2')}>{errorMessage}</h1>
          <p className={cn(text.p, 'mt-3')}>
            Start the local API, then refresh this page to open the builder.
          </p>
        </section>
      </main>
    )
  }

  if (!session) {
    return (
      <main className={cn(layout.pageShell, layout.narrowShell, 'pt-14')}>
        <section className={paddedPanel}>
          <p className={text.p}>Loading your builder session...</p>
        </section>
      </main>
    )
  }

  return (
    <main className={layout.appShell}>
      <aside className="flex flex-col gap-3 rounded-[18px] border border-border bg-[var(--surface-1)] p-4 shadow-[var(--shadow-tight)]">
        <div className="mb-1 grid gap-3 border-b border-border pb-4">
          <div className="flex items-center gap-3">
            <img src="/logo.png" alt="" className="size-9 rounded-full object-contain" />
            <span className="font-black text-[var(--paper)]">
              {session.user.name || session.user.email}
            </span>
          </div>
          <small className="text-xs font-bold uppercase tracking-[0.1em] text-[var(--paper-muted)]">
            {session.user.workspaceRole}
          </small>
        </div>
        <Link
          to="/app"
          className="rounded-[14px] border border-transparent px-3.5 py-3 font-bold text-[var(--paper-muted)] transition-[background,border-color,color,transform] hover:-translate-y-px hover:border-border hover:bg-[var(--surface-2)] hover:text-[var(--paper)]"
        >
          Sites
        </Link>
        <p className={cn(text.p, 'mx-0.5 mt-1 text-sm')}>
          Drafts stay editable until you publish an immutable version.
        </p>
        <Button
          type="button"
          variant="plain"
          className="rounded-[14px] border border-transparent px-3.5 py-3 font-bold text-[var(--paper-muted)] transition-[background,border-color,color,transform] hover:-translate-y-px hover:border-border hover:bg-[var(--surface-2)] hover:text-[var(--paper)]"
          onClick={handleSignOut}
        >
          Sign out
        </Button>
      </aside>
      <section className={layout.appContent}>
        <Outlet />
      </section>
    </main>
  )
}
