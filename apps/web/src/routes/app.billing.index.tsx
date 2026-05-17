import { createFileRoute } from '@tanstack/react-router'
import { ArrowUpRight, CreditCard, Sparkles } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  APIError,
  createBillingCheckout,
  createBillingPortal,
  getBillingState,
  getCurrentSession,
  type BillingState,
  type BuilderSession,
} from '@/lib/api'
import { emptyState, paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/app/billing/')({
  component: BillingPage,
})

function BillingPage() {
  const [session, setSession] = useState<BuilderSession | null>(null)
  const [billingState, setBillingState] = useState<BillingState | null>(null)
  const [errorMessage, setErrorMessage] = useState('')
  const [statusMessage, setStatusMessage] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isStartingCheckout, setIsStartingCheckout] = useState<'basic' | 'pro' | ''>('')
  const [isOpeningPortal, setIsOpeningPortal] = useState(false)

  useEffect(() => {
    let isMounted = true

    Promise.all([getCurrentSession(), getBillingState()])
      .then(([nextSession, nextBillingState]) => {
        if (!isMounted) {
          return
        }
        setSession(nextSession)
        setBillingState(nextBillingState)
        setIsLoading(false)
      })
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setErrorMessage(error instanceof APIError ? error.message : 'Could not load billing')
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [])

  async function handleCheckout(plan: 'basic' | 'pro') {
    setIsStartingCheckout(plan)
    setErrorMessage('')
    setStatusMessage('')

    try {
      const response = await createBillingCheckout(plan)
      window.location.href = response.url
    } catch (error) {
      setErrorMessage(error instanceof APIError ? error.message : 'Could not start checkout')
      setIsStartingCheckout('')
    }
  }

  async function handlePortal() {
    setIsOpeningPortal(true)
    setErrorMessage('')
    setStatusMessage('')

    try {
      const response = await createBillingPortal()
      window.location.href = response.url
    } catch (error) {
      setErrorMessage(error instanceof APIError ? error.message : 'Could not open billing portal')
      setIsOpeningPortal(false)
    }
  }

  if (isLoading) {
    return (
      <section className={cn(paddedPanel, 'rounded-[14px]')}>
        <p className={text.p}>Loading billing…</p>
      </section>
    )
  }

  if (!session || !billingState) {
    return (
      <section className={cn(paddedPanel, 'rounded-[14px]')}>
        <p className={text.error}>{errorMessage || 'Billing is unavailable right now.'}</p>
      </section>
    )
  }

  const { entitlement, usage } = billingState
  const trialEndsLabel = session.trialExpiresAt
    ? new Date(session.trialExpiresAt).toLocaleDateString()
    : ''
  const promptLabel = entitlement.subscriptionLive
    ? entitlement.monthlyPromptLimit
      ? `${usage.periodPromptCount}/${entitlement.monthlyPromptLimit} prompts this period`
      : `${usage.periodPromptCount} prompts this period`
    : `${session.promptsUsed ?? 0}/${session.promptLimit ?? 25} trial prompts used`
  const siteLabel = entitlement.activeSiteLimit
    ? `${usage.activeSiteCount}/${entitlement.activeSiteLimit} sites`
    : `${usage.activeSiteCount} sites`
  const storageLabel = entitlement.assetStorageLimitBytes
    ? `${formatBytes(usage.uploadedAssetBytes)} of ${formatBytes(entitlement.assetStorageLimitBytes)} used`
    : `${formatBytes(usage.uploadedAssetBytes)} uploaded`

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]">
      <section className={cn(paddedPanel, 'rounded-[14px]')}>
        <div className="grid gap-3">
          <p className={text.eyebrow}>Billing</p>
          <h1 className={text.sectionTitle}>Keep the builder live</h1>
          <p className={text.p}>
            Trials are generous enough to get a real site out. Paid plans lift the time gate,
            keep prompting open, and unlock custom domains.
          </p>
        </div>

        <div className="mt-6 grid gap-3 lg:grid-cols-2">
          <PlanCard
            name="Basic"
            accent="var(--thread-gold)"
            description="The production path for a small business site and its next rounds of edits."
            ctaLabel={isStartingCheckout === 'basic' ? 'Opening checkout…' : 'Choose Basic'}
            disabled={isStartingCheckout !== ''}
            onSelect={() => handleCheckout('basic')}
          />
          <PlanCard
            name="Pro"
            accent="var(--thread-teal)"
            description="A roomier plan when you expect multiple sites or heavier iteration."
            ctaLabel={isStartingCheckout === 'pro' ? 'Opening checkout…' : 'Choose Pro'}
            disabled={isStartingCheckout !== ''}
            onSelect={() => handleCheckout('pro')}
          />
        </div>

        {errorMessage ? <p className={cn(text.error, 'mt-4')}>{errorMessage}</p> : null}
        {statusMessage ? <p className={cn(text.success, 'mt-4')}>{statusMessage}</p> : null}
      </section>

      <section className={cn(paddedPanel, 'rounded-[14px]')}>
        <div className="grid gap-4">
          <div className="rounded-[14px] border border-border bg-[var(--surface-2)] p-4">
            <p className={text.label}>Current plan</p>
            <div className="mt-3 flex items-center justify-between gap-3">
              <div>
                <p className="text-2xl font-black text-[var(--paper)]">
                  {humanPlan(entitlement.plan)}
                </p>
                <p className="mt-1 text-sm text-[var(--paper-muted)]">
                  {entitlement.subscriptionLive
                    ? 'Subscription is live'
                    : trialEndsLabel
                      ? `Trial access changes after ${trialEndsLabel}`
                      : 'Trial workspace'}
                </p>
              </div>
              <span className="inline-flex min-h-8 items-center rounded-full border border-border bg-[var(--surface-1)] px-3 text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper)]">
                {entitlement.status}
              </span>
            </div>
          </div>

          <div className="grid gap-3">
            <UsageRow label="Prompt allowance" value={promptLabel} />
            <UsageRow label="Site count" value={siteLabel} />
            <UsageRow label="Asset storage" value={storageLabel} />
            <UsageRow
              label="Custom domains"
              value={entitlement.customDomainsEnabled ? 'Included' : 'Upgrade required'}
            />
          </div>

          {entitlement.subscriptionLive ? (
            <Button type="button" variant="outline" onClick={handlePortal} disabled={isOpeningPortal}>
              <CreditCard className="size-4" />
              {isOpeningPortal ? 'Opening portal…' : 'Manage billing'}
            </Button>
          ) : (
            <div className={emptyState}>
              <p className={text.p}>
                Checkout will also claim the workspace if it is still trial-only.
              </p>
            </div>
          )}
        </div>
      </section>
    </div>
  )
}

function PlanCard({
  name,
  accent,
  description,
  ctaLabel,
  disabled,
  onSelect,
}: {
  name: string
  accent: string
  description: string
  ctaLabel: string
  disabled: boolean
  onSelect: () => void
}) {
  return (
    <article className="grid gap-4 rounded-[16px] border border-border bg-[var(--surface-2)] p-5">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-lg font-black text-[var(--paper)]">{name}</p>
          <p className="mt-1 text-sm text-[var(--paper-muted)]">{description}</p>
        </div>
        <span
          className="flex size-11 items-center justify-center rounded-full"
          style={{ background: `color-mix(in oklch, ${accent} 22%, var(--surface-1))` }}
        >
          <Sparkles className="size-5 text-[var(--paper)]" />
        </span>
      </div>
      <Button type="button" onClick={onSelect} disabled={disabled}>
        {ctaLabel}
        <ArrowUpRight className="size-4" />
      </Button>
    </article>
  )
}

function UsageRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-[12px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_42%,transparent)] px-4 py-3">
      <p className={text.label}>{label}</p>
      <p className="text-right text-sm font-semibold text-[var(--paper)]">{value}</p>
    </div>
  )
}

function humanPlan(plan: string) {
  switch (plan) {
    case 'pro':
      return 'Pro'
    case 'basic':
      return 'Basic'
    default:
      return 'Trial'
  }
}

function formatBytes(value: number) {
  if (!value) {
    return '0 B'
  }
  const units = ['B', 'KB', 'MB', 'GB']
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  const precision = size >= 10 || index === 0 ? 0 : 1
  return `${size.toFixed(precision)} ${units[index]}`
}
