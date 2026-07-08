import {
  Link,
  Outlet,
  createFileRoute,
  useRouterState,
} from '@tanstack/react-router'
import { BarChart3, ChevronDown, Eye, Home, Link2, LogOut, PencilLine, ShieldCheck } from 'lucide-react'
import type { FormEvent } from 'react'
import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  APIError,
  claimWorkspace,
  createRecoveryKey,
  getBillingState,
  type BuilderSession,
  type BillingState,
  getCurrentSession,
  getSiteDraft,
  logout,
} from '@/lib/api'
import { actions, form, layout, paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'
import { setWorkspaceLocale } from '@/lib/workspace-locale'

export const Route = createFileRoute('/app')({
  component: AppLayout,
})

function AppLayout() {
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const [session, setSession] = useState<BuilderSession | null>(null)
  const [billingState, setBillingState] = useState<BillingState | null>(null)
  const [errorMessage, setErrorMessage] = useState('')
  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false)
  const [currentSiteName, setCurrentSiteName] = useState('')
  const [isSavePanelOpen, setIsSavePanelOpen] = useState(false)
  const [claimEmail, setClaimEmail] = useState('')
  const [claimName, setClaimName] = useState('')
  const [saveStatusMessage, setSaveStatusMessage] = useState('')
  const [saveErrorMessage, setSaveErrorMessage] = useState('')
  const [recoveryURL, setRecoveryURL] = useState('')
  const [isSavingWorkspace, setIsSavingWorkspace] = useState(false)
  const accountMenuRef = useRef<HTMLDivElement | null>(null)
  const siteMatch = pathname.match(/^\/app\/sites\/([^/]+)/)
  const siteId = siteMatch?.[1] ?? ''
  const isBuilderRoute = /^\/app\/sites\/[^/]+\/?$/.test(pathname)
  const isPreviewRoute = /^\/app\/sites\/[^/]+\/preview\/?$/.test(pathname)
  const isAnalyticsRoute = /^\/app\/sites\/[^/]+\/analytics\/?$/.test(pathname)

  useEffect(() => {
    let isMounted = true

    getCurrentSession()
      .then((nextSession) => {
        if (isMounted) {
          setSession(nextSession)
          setWorkspaceLocale(nextSession.workspaceLocale)
          setClaimEmail(nextSession.user?.email ?? '')
          setClaimName(nextSession.user?.name ?? '')
        }
      })
      .catch((error) => {
        if (error instanceof APIError && error.status === 401) {
          window.location.href = '/'
          return
        }
        if (isMounted) {
          setErrorMessage('Could not load your session')
        }
      })

    return () => {
      isMounted = false
    }
  }, [pathname])

  useEffect(() => {
    let isMounted = true

    getBillingState()
      .then((nextState) => {
        if (isMounted) {
          setBillingState(nextState)
        }
      })
      .catch(() => {
        if (isMounted) {
          setBillingState(null)
        }
      })

    return () => {
      isMounted = false
    }
  }, [pathname])

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
    window.location.href = '/'
  }

  async function handleClaimWorkspace(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSavingWorkspace(true)
    setSaveErrorMessage('')
    setSaveStatusMessage('')

    try {
      const response = await claimWorkspace(claimEmail, claimName)
      setSession(response.session)
      setRecoveryURL('')
      setSaveStatusMessage('Magic link sent. Open the email to finish securing this workspace.')
    } catch (error) {
      setSaveErrorMessage(
        error instanceof APIError ? error.message : 'Could not save this workspace',
      )
    } finally {
      setIsSavingWorkspace(false)
    }
  }

  async function handleCreateRecoveryKey() {
    setIsSavingWorkspace(true)
    setSaveErrorMessage('')
    setSaveStatusMessage('')

    try {
      const response = await createRecoveryKey()
      setRecoveryURL(response.recoveryUrl)
      setSaveStatusMessage('Recovery link ready. Treat it like a password.')
    } catch (error) {
      setSaveErrorMessage(
        error instanceof APIError ? error.message : 'Could not create a recovery link',
      )
    } finally {
      setIsSavingWorkspace(false)
    }
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

  const displayName =
    session.user?.name || session.user?.email || (session.kind === 'trial' ? 'Trial workspace' : 'Snaelda')
  const currentPlan = billingState?.entitlement.plan || (session.subscriptionLive ? 'site' : 'trial')
  const planBadgeLabel = currentPlan === 'pro' ? 'Pro' : currentPlan === 'site' ? 'Site' : 'Trial'
  const visibleSiteName = siteId ? currentSiteName : ''
  const initials = displayName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('')
  const isTrialWorkspace = Boolean(session.trialStartedAt && !session.subscriptionLive)
  const isClaimed = Boolean(session.claimedByUserId)
  const trialEndLabel = session.trialExpiresAt
    ? new Date(session.trialExpiresAt).toLocaleDateString()
    : ''

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
              <Link
                to="/app/billing"
                className="inline-flex min-h-10 items-center gap-2 rounded-full px-3 py-2 text-sm font-bold text-[var(--paper-muted)] transition-[background,color] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]"
                activeProps={{ className: 'bg-[var(--surface-2)] text-[var(--paper)]' }}
              >
                Billing
              </Link>
              {session.isOperator ? (
                <Link
                  to="/app/admin"
                  className="inline-flex min-h-10 items-center gap-2 rounded-full px-3 py-2 text-sm font-bold text-[var(--paper-muted)] transition-[background,color] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]"
                  activeProps={{ className: 'bg-[var(--surface-2)] text-[var(--paper)]' }}
                >
                  <ShieldCheck className="size-4" />
                  Control room
                </Link>
              ) : null}
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
            <Link
              to="/app/billing"
              className="hidden min-h-10 items-center rounded-full border border-border bg-[var(--surface-2)] px-3 py-2 text-sm font-bold text-[var(--paper)] transition-[border-color,background] hover:border-[var(--thread-gold)] hover:bg-[var(--surface-3)] md:inline-flex"
            >
              {planBadgeLabel}
            </Link>
            {siteId ? (
              <>
                <Link
                  to="/app/sites/$siteId/analytics"
                  params={{ siteId }}
                  search={{ window: '7d' as const }}
                  className={cn(
                    actions.inlineLink,
                    isAnalyticsRoute && 'border-[var(--thread-teal)] bg-[var(--surface-3)]',
                  )}
                >
                  <BarChart3 className="size-4" />
                  Analytics
                </Link>
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
                  search={{ panel: undefined }}
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
                    {session.user?.workspaceRole || session.workspaceRole}
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
                      {session.user?.email || 'Cookie-bound builder session'}
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

      {isTrialWorkspace ? (
        <section className="border-b border-border bg-[linear-gradient(90deg,color-mix(in_oklch,var(--thread-mauve)_18%,var(--surface-1)),color-mix(in_oklch,var(--thread-teal)_12%,var(--surface-1)))]">
          <div className="mx-auto grid w-full max-w-[1760px] gap-4 px-5 py-4 max-sm:px-3.5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="grid gap-1">
                <p className="inline-flex items-center gap-2 text-sm font-bold text-[var(--paper)]">
                  <ShieldCheck className="size-4 text-[var(--thread-gold)]" />
                  Trial workspace
                </p>
                <p className="text-sm text-[var(--paper-muted)]">
                  {session.promptsUsed ?? 0}/{session.promptLimit ?? 25} prompts used
                  {trialEndLabel ? `, edits pause after ${trialEndLabel}` : ''}.
                  {!isClaimed ? ' Claim this workspace before you publish.' : ' Your workspace is now linked to email login.'}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  className="border-[var(--border)] bg-transparent hover:border-[var(--thread-gold)] hover:bg-transparent hover:text-[var(--thread-gold)]"
                  onClick={() => setIsSavePanelOpen((value) => !value)}
                >
                  Save your workspace
                </Button>
                {!isClaimed ? (
                  <Button type="button" variant="plain" onClick={handleCreateRecoveryKey} disabled={isSavingWorkspace}>
                    <Link2 className="size-4" />
                    Copy workspace link
                  </Button>
                ) : null}
              </div>
            </div>

            {isSavePanelOpen || recoveryURL ? (
              <div className="grid gap-4 rounded-[14px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_92%,black)] p-4">
                {!isClaimed ? (
                  <form className={cn(form.grid, 'gap-3')} onSubmit={handleClaimWorkspace}>
                    <div className="grid gap-1">
                      <p className={text.label}>Add an email</p>
                      <p className="text-sm text-[var(--paper-muted)]">
                        We’ll send a magic link so this workspace is recoverable from any browser.
                      </p>
                    </div>
                    <Input
                      type="email"
                      value={claimEmail}
                      onChange={(event) => setClaimEmail(event.target.value)}
                      placeholder="you@example.com"
                      required
                    />
                    <Input
                      type="text"
                      value={claimName}
                      onChange={(event) => setClaimName(event.target.value)}
                      placeholder="Your name"
                    />
                    <div className="flex flex-wrap gap-2">
                      <Button type="submit" disabled={isSavingWorkspace}>
                        {isSavingWorkspace ? 'Sending magic link...' : 'Send magic link'}
                      </Button>
                    </div>
                  </form>
                ) : null}

                {recoveryURL ? (
                  <div className="grid gap-2">
                    <p className={text.label}>Workspace link</p>
                    <Input readOnly value={recoveryURL} />
                  </div>
                ) : null}

                {saveErrorMessage ? <p className={text.error}>{saveErrorMessage}</p> : null}
                {saveStatusMessage ? <p className={text.success}>{saveStatusMessage}</p> : null}
              </div>
            ) : null}
          </div>
        </section>
      ) : null}

      <main className={layout.appShell}>
        <section className={layout.appContent}>
          <Outlet />
        </section>
      </main>
    </>
  )
}
