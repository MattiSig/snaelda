import {
  Link,
  Outlet,
  createFileRoute,
  useRouterState,
} from '@tanstack/react-router'
import { ChevronDown, Eye, Home, LogOut, PencilLine } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import {
  APIError,
  type AuthSession,
  getCurrentSession,
  getSiteDraft,
  logout,
} from '@/lib/api'
import { actions, layout, paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/app')({
  component: AppLayout,
})

function AppLayout() {
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const [session, setSession] = useState<AuthSession | null>(null)
  const [errorMessage, setErrorMessage] = useState('')
  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false)
  const [currentSiteName, setCurrentSiteName] = useState('')
  const accountMenuRef = useRef<HTMLDivElement | null>(null)
  const siteMatch = pathname.match(/^\/app\/sites\/([^/]+)/)
  const siteId = siteMatch?.[1] ?? ''
  const isBuilderRoute = /^\/app\/sites\/[^/]+\/?$/.test(pathname)
  const isPreviewRoute = /^\/app\/sites\/[^/]+\/preview\/?$/.test(pathname)

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

  useEffect(() => {
    let isMounted = true

    if (!siteId) return

    getSiteDraft(siteId)
      .then((response) => {
        if (isMounted) {
          setCurrentSiteName(response.draft.site.name)
        }
      })
      .catch(() => {
        if (isMounted) {
          setCurrentSiteName('')
        }
      })

    return () => {
      isMounted = false
    }
  }, [siteId])

  useEffect(() => {
    if (!isAccountMenuOpen) {
      return
    }

    function handlePointerDown(event: PointerEvent) {
      if (!accountMenuRef.current?.contains(event.target as Node)) {
        setIsAccountMenuOpen(false)
      }
    }

    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsAccountMenuOpen(false)
      }
    }

    window.addEventListener('pointerdown', handlePointerDown)
    window.addEventListener('keydown', handleEscape)

    return () => {
      window.removeEventListener('pointerdown', handlePointerDown)
      window.removeEventListener('keydown', handleEscape)
    }
  }, [isAccountMenuOpen])

  async function handleSignOut() {
    setIsAccountMenuOpen(false)
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

  const displayName = session.user.name || session.user.email
  const visibleSiteName = siteId ? currentSiteName : ''
  const initials = displayName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('')

  return (
    <>
      <header className="sticky top-0 z-30 border-b border-border bg-[color-mix(in_oklch,var(--surface-1)_94%,transparent)] backdrop-blur-[18px]">
        <div className="mx-auto flex min-h-[72px] w-full max-w-[1760px] items-center justify-between gap-4 px-5 py-3 max-sm:grid max-sm:gap-3 max-sm:px-3.5">
          <div className="flex min-w-0 items-center gap-3">
            <Link
              to="/"
              className="flex size-10 items-center justify-center rounded-[12px] border border-border bg-[var(--surface-2)]"
              aria-label="Go to home"
            >
              <img src="/logo.png" alt="" className="size-6 object-contain" />
            </Link>

            <nav aria-label="App navigation" className="flex items-center gap-1.5">
              <Link
                to="/app"
                className="inline-flex min-h-10 items-center gap-2 rounded-full px-3 py-2 text-sm font-bold text-[var(--paper-muted)] transition-[background,color] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]"
                activeProps={{ className: 'bg-[var(--surface-2)] text-[var(--paper)]' }}
              >
                <Home className="size-4" />
                Sites
              </Link>
            </nav>

            {visibleSiteName ? (
              <div className="hidden min-w-0 border-l border-border pl-3 md:block">
                <p className={text.eyebrow}>Current site</p>
                <p className="truncate text-sm font-bold text-[var(--paper)]">
                  {visibleSiteName}
                </p>
              </div>
            ) : null}
          </div>

          <div className="flex items-center justify-end gap-2">
            {siteId ? (
              <>
                <Link
                  to="/app/sites/$siteId/preview"
                  params={{ siteId }}
                  className={cn(
                    actions.inlineLink,
                    isPreviewRoute && 'border-[var(--thread-teal)] bg-[var(--surface-3)]',
                  )}
                >
                  <Eye className="size-4" />
                  Preview
                </Link>
                <Link
                  to="/app/sites/$siteId"
                  params={{ siteId }}
                  className={cn(
                    actions.inlineLink,
                    isBuilderRoute && 'border-[var(--thread-teal)] bg-[var(--surface-3)]',
                  )}
                >
                  <PencilLine className="size-4" />
                  Edit
                </Link>
              </>
            ) : null}

            <div ref={accountMenuRef} className="relative">
              <button
                type="button"
                className="flex min-h-12 items-center gap-3 rounded-[12px] border border-border bg-[var(--surface-2)] px-3 py-2.5 text-left transition-[border-color,background] hover:border-[var(--thread-gold)] hover:bg-[var(--surface-3)]"
                onClick={() => setIsAccountMenuOpen((value) => !value)}
                aria-haspopup="menu"
                aria-expanded={isAccountMenuOpen}
              >
                <span className="flex size-9 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--thread-violet)_24%,var(--surface-1))] text-sm font-black text-[var(--paper)]">
                  {initials || 'S'}
                </span>
                <span className="grid min-w-0 gap-0.5">
                  <span className="truncate text-sm font-bold text-[var(--paper)]">
                    {displayName}
                  </span>
                  <span className="truncate text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                    {session.user.workspaceRole}
                  </span>
                </span>
                <ChevronDown className="size-4 text-[var(--paper-muted)]" />
              </button>

              {isAccountMenuOpen ? (
                <div
                  role="menu"
                  className="absolute right-0 top-[calc(100%+12px)] z-20 grid min-w-[240px] gap-1 rounded-[12px] border border-border bg-[var(--surface-1)] p-2 shadow-[var(--shadow-soft)]"
                >
                  <div className="rounded-[10px] bg-[var(--surface-2)] px-3 py-2.5">
                    <p className="truncate text-sm font-bold text-[var(--paper)]">
                      {displayName}
                    </p>
                    <p className="mt-1 truncate text-xs text-[var(--paper-muted)]">
                      {session.user.email}
                    </p>
                  </div>
                  <button
                    type="button"
                    role="menuitem"
                    className="flex min-h-11 items-center gap-2 rounded-[10px] px-3 py-2.5 text-sm font-bold text-[var(--paper-muted)] transition-[background,color] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]"
                    onClick={handleSignOut}
                  >
                    <LogOut className="size-4" />
                    Sign out
                  </button>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </header>

      <main className={layout.appShell}>
        <section className={layout.appContent}>
          <Outlet />
        </section>
      </main>
    </>
  )
}
