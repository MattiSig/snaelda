import { Link, createFileRoute } from '@tanstack/react-router'
import { ArrowUpRight, Loader2, RefreshCw, ShieldCheck } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  APIError,
  getAdminOverview,
  getCurrentSession,
  listAdminGenerationJobs,
  listAdminSites,
  type AdminGenerationJob,
  type AdminOverview,
  type AdminSite,
  type BuilderSession,
} from '@/lib/api'
import { paddedPanel, statGrid, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/app/admin/')({
  component: ControlRoomPage,
})

function ControlRoomPage() {
  const [session, setSession] = useState<BuilderSession | null>(null)
  const [overview, setOverview] = useState<AdminOverview | null>(null)
  const [jobs, setJobs] = useState<AdminGenerationJob[]>([])
  const [sites, setSites] = useState<AdminSite[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  const loadData = useCallback(async () => {
    const [overviewResponse, jobsResponse, sitesResponse] = await Promise.all([
      getAdminOverview(),
      listAdminGenerationJobs(20),
      listAdminSites(20),
    ])
    setOverview(overviewResponse.overview)
    setJobs(jobsResponse.jobs)
    setSites(sitesResponse.sites)
  }, [])

  useEffect(() => {
    let isMounted = true

    getCurrentSession()
      .then((nextSession) => {
        if (!isMounted) {
          return
        }
        setSession(nextSession)
        if (!nextSession.isOperator) {
          setErrorMessage('Operator access is required to view the control room.')
          setIsLoading(false)
          return
        }
        return loadData()
          .catch((error) => {
            if (!isMounted) {
              return
            }
            setErrorMessage(
              error instanceof APIError
                ? error.message
                : 'Could not load the control room.',
            )
          })
          .finally(() => {
            if (isMounted) {
              setIsLoading(false)
            }
          })
      })
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setErrorMessage(
          error instanceof APIError ? error.message : 'Could not load your session.',
        )
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [loadData])

  async function handleRefresh() {
    setIsRefreshing(true)
    setErrorMessage('')
    try {
      await loadData()
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not refresh the control room.',
      )
    } finally {
      setIsRefreshing(false)
    }
  }

  if (isLoading) {
    return (
      <section className={paddedPanel}>
        <p className={text.p}>Loading the control room…</p>
      </section>
    )
  }

  if (!session?.isOperator) {
    return (
      <section className={paddedPanel}>
        <p className={text.error}>{errorMessage || 'Operator access is required.'}</p>
      </section>
    )
  }

  const jobsTrend = overview
    ? overview.generationJobs.last24Hours - overview.generationJobs.previous24Hours
    : 0

  return (
    <div className="grid gap-4">
      <section className={paddedPanel}>
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="grid gap-2">
            <p className={text.eyebrow}>Operator control room</p>
            <h1 className={text.sectionTitle}>The workshop floor</h1>
            <p className={text.p}>
              A quiet look at what is being spun, published, and signed up across
              every workspace.
            </p>
          </div>
          <div className="flex items-center gap-3">
            {overview ? (
              <p className="text-sm text-[var(--paper-muted)]">
                As of {formatTime(overview.generatedAt)}
              </p>
            ) : null}
            <Button
              type="button"
              variant="outline"
              onClick={handleRefresh}
              disabled={isRefreshing}
            >
              {isRefreshing ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <RefreshCw className="size-4" />
              )}
              Refresh
            </Button>
          </div>
        </div>
        {errorMessage ? <p className={cn(text.error, 'mt-4')}>{errorMessage}</p> : null}

        {overview ? (
          <div className={cn(statGrid.list, 'pb-0')}>
            <StatTile
              label="Spun in the last 24h"
              value={overview.generationJobs.last24Hours}
              detail={trendLabel(jobsTrend)}
              highlight={overview.generationJobs.last24Hours > 0}
            />
            <StatTile
              label="Generations, 7 days"
              value={overview.generationJobs.last7Days}
              detail={`${overview.generationJobs.total} all time`}
            />
            <StatTile
              label="Publishes, 24h"
              value={overview.publishes.last24Hours}
              detail={`${overview.publishes.last7Days} in 7 days`}
            />
            <StatTile
              label="Sites"
              value={overview.sites.total}
              detail={`${overview.sites.published} published · ${overview.sites.last7Days} new this week`}
            />
            <StatTile
              label="Users"
              value={overview.users.total}
              detail={`${overview.users.last7Days} new this week`}
            />
            <StatTile
              label="Form submissions, 24h"
              value={overview.forms.last24Hours}
            />
          </div>
        ) : null}
      </section>

      <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
        <section className={paddedPanel}>
          <div className="grid gap-1.5">
            <p className={text.eyebrow}>Spinning activity</p>
            <h2 className={text.sectionTitle}>Recent generation jobs</h2>
          </div>
          {jobs.length === 0 ? (
            <p className={cn(text.p, 'mt-4')}>
              Nothing on the spindle yet. Jobs land here the moment someone
              generates a site.
            </p>
          ) : (
            <ul className="mt-4 grid gap-1.5">
              {jobs.map((job) => (
                <li
                  key={job.id}
                  className="grid gap-1.5 rounded-[12px] border border-border bg-[var(--surface-2)] px-4 py-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <StatusPill status={job.status} hasError={job.hasError} />
                    <span className="text-xs text-[var(--paper-muted)]">
                      {formatTime(job.createdAt)}
                    </span>
                  </div>
                  <p className="truncate text-sm font-semibold text-[var(--paper)]" title={job.prompt}>
                    {job.prompt || 'No prompt recorded'}
                  </p>
                  <p className="text-xs text-[var(--paper-muted)]">
                    {[job.siteName, job.workspaceName, job.createdByEmail]
                      .filter(Boolean)
                      .join(' · ') || 'No site attached'}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </section>

        <div className="grid gap-4">
          <section className={paddedPanel}>
            <div className="grid gap-1.5">
              <p className={text.eyebrow}>Inventory</p>
              <h2 className={text.sectionTitle}>Newest sites</h2>
            </div>
            {sites.length === 0 ? (
              <p className={cn(text.p, 'mt-4')}>No sites yet.</p>
            ) : (
              <ul className="mt-4 grid gap-1.5">
                {sites.map((site) => (
                  <li
                    key={site.id}
                    className="flex items-center justify-between gap-3 rounded-[12px] border border-border bg-[var(--surface-2)] px-4 py-3"
                  >
                    <div className="min-w-0 grid gap-0.5">
                      <p className="truncate text-sm font-semibold text-[var(--paper)]">
                        {site.name || site.slug}
                      </p>
                      <p className="truncate text-xs text-[var(--paper-muted)]">
                        {[site.workspaceName, formatTime(site.createdAt)]
                          .filter(Boolean)
                          .join(' · ')}
                      </p>
                    </div>
                    <span
                      className={cn(
                        'shrink-0 rounded-full border px-2.5 py-1 text-[0.65rem] font-bold uppercase tracking-[0.08em]',
                        site.published
                          ? 'border-[var(--thread-teal)] text-[var(--thread-teal)]'
                          : 'border-border text-[var(--paper-muted)]',
                      )}
                    >
                      {site.published ? 'Published' : site.status}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className={paddedPanel}>
            <div className="grid gap-1.5">
              <p className={text.eyebrow}>Support tools</p>
              <h2 className={text.sectionTitle}>Workbench</h2>
              <p className={text.p}>
                Hands-on tools live here as they are built.
              </p>
            </div>
            <div className="mt-4 grid gap-2">
              <Link
                to="/app/admin/once-over"
                className="flex items-center justify-between gap-3 rounded-[12px] border border-border bg-[var(--surface-2)] px-4 py-3 transition-[border-color,background] hover:border-[var(--thread-teal)] hover:bg-[var(--surface-3)]"
              >
                <span className="flex items-center gap-2.5 text-sm font-bold text-[var(--paper)]">
                  <ShieldCheck className="size-4" />
                  Once-over queue
                </span>
                <ArrowUpRight className="size-4 text-[var(--paper-muted)]" />
              </Link>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

function StatTile({
  label,
  value,
  detail,
  highlight,
}: {
  label: string
  value: number
  detail?: string
  highlight?: boolean
}) {
  return (
    <div className={statGrid.item}>
      <p className={text.label}>{label}</p>
      <p
        className={cn(
          statGrid.value,
          highlight && 'text-[var(--thread-teal)]',
        )}
      >
        {value}
      </p>
      {detail ? (
        <p className="mt-1 text-xs text-[var(--paper-muted)]">{detail}</p>
      ) : null}
    </div>
  )
}

function StatusPill({ status, hasError }: { status: string; hasError: boolean }) {
  const isDone = status === 'completed' || status === 'succeeded'
  const isFailed = status === 'failed' || hasError
  const isActive = status === 'running' || status === 'pending' || status === 'queued'
  return (
    <span
      className={cn(
        'rounded-full border px-2.5 py-1 text-[0.65rem] font-bold uppercase tracking-[0.08em]',
        isFailed
          ? 'border-destructive text-destructive'
          : isDone
            ? 'border-[var(--thread-teal)] text-[var(--thread-teal)]'
            : isActive
              ? 'border-[var(--thread-gold)] text-[var(--thread-gold)]'
              : 'border-border text-[var(--paper-muted)]',
      )}
    >
      {status}
    </span>
  )
}

function formatTime(value: string) {
  const date = new Date(value)
  const deltaMs = Date.now() - date.getTime()
  const minutes = Math.round(deltaMs / 60000)
  if (minutes < 1) {
    return 'just now'
  }
  if (minutes < 60) {
    return `${minutes}m ago`
  }
  const hours = Math.round(minutes / 60)
  if (hours < 24) {
    return `${hours}h ago`
  }
  return date.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function trendLabel(delta: number) {
  if (delta > 0) {
    return `up ${delta} vs the day before`
  }
  if (delta < 0) {
    return `down ${Math.abs(delta)} vs the day before`
  }
  return 'level with the day before'
}
