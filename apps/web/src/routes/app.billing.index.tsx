import { createFileRoute } from '@tanstack/react-router'
import { ArrowUpRight, CreditCard, Sparkles } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  APIError,
  createBillingCheckout,
  createBillingPortal,
  getBillingState,
  getCurrentSession,
  updateOnceOver,
  type BillingState,
  type BuilderSession,
} from '@/lib/api'
import { emptyState, form, paddedPanel, text } from '@/lib/styles'
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
  const [isStartingOnceOverCheckout, setIsStartingOnceOverCheckout] = useState(false)
  const [isOpeningPortal, setIsOpeningPortal] = useState(false)
  const [onceOverBusiness, setOnceOverBusiness] = useState('')
  const [onceOverVisitor, setOnceOverVisitor] = useState('')
  const [onceOverOutcome, setOnceOverOutcome] = useState('')
  const [onceOverStuckOn, setOnceOverStuckOn] = useState('')
  const [onceOverErrorMessage, setOnceOverErrorMessage] = useState('')
  const [onceOverStatusMessage, setOnceOverStatusMessage] = useState('')
  const [isSavingOnceOver, setIsSavingOnceOver] = useState(false)

  function applyOnceOverForm(nextBillingState: BillingState) {
    setOnceOverBusiness(nextBillingState.onceOver.request?.intakeBusiness ?? '')
    setOnceOverVisitor(nextBillingState.onceOver.request?.intakeVisitor ?? '')
    setOnceOverOutcome(nextBillingState.onceOver.request?.intakeOutcome ?? '')
    setOnceOverStuckOn(nextBillingState.onceOver.request?.intakeStuckOn ?? '')
  }

  useEffect(() => {
    let isMounted = true

    Promise.all([getCurrentSession(), getBillingState()])
      .then(([nextSession, nextBillingState]) => {
        if (!isMounted) {
          return
        }
        setSession(nextSession)
        setBillingState(nextBillingState)
        applyOnceOverForm(nextBillingState)
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
      const response = await createBillingCheckout({ plan })
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

  async function handleOnceOverCheckout() {
    setIsStartingOnceOverCheckout(true)
    setOnceOverErrorMessage('')
    setOnceOverStatusMessage('')

    try {
      const response = await createBillingCheckout({ purchaseType: 'once_over' })
      window.location.href = response.url
    } catch (error) {
      setOnceOverErrorMessage(error instanceof APIError ? error.message : 'Could not start the once-over checkout')
      setIsStartingOnceOverCheckout(false)
    }
  }

  async function handleSaveOnceOver(readyForReview: boolean) {
    setIsSavingOnceOver(true)
    setOnceOverErrorMessage('')
    setOnceOverStatusMessage('')

    try {
      const response = await updateOnceOver({
        intakeBusiness: onceOverBusiness,
        intakeVisitor: onceOverVisitor,
        intakeOutcome: onceOverOutcome,
        intakeStuckOn: onceOverStuckOn,
        readyForReview,
      })
      setBillingState((currentState) =>
        currentState
          ? {
              ...currentState,
              onceOver: response.onceOver,
            }
          : currentState,
      )
      if (billingState) {
        applyOnceOverForm({
          ...billingState,
          onceOver: response.onceOver,
        })
      }
      setOnceOverStatusMessage(readyForReview ? 'Once-over request marked ready for review.' : 'Once-over intake saved.')
    } catch (error) {
      setOnceOverErrorMessage(error instanceof APIError ? error.message : 'Could not save the once-over intake')
    } finally {
      setIsSavingOnceOver(false)
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
  const onceOver = billingState.onceOver
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
  const onceOverRequest = onceOver.request
  const onceOverPaidLabel = onceOverRequest?.paidAt
    ? new Date(onceOverRequest.paidAt).toLocaleDateString()
    : ''
  const onceOverSubmittedLabel = onceOverRequest?.intakeSubmittedAt
    ? new Date(onceOverRequest.intakeSubmittedAt).toLocaleDateString()
    : ''

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

      <section className={cn(paddedPanel, 'rounded-[14px] xl:col-span-2')}>
        <div className="grid gap-5 lg:grid-cols-[minmax(0,0.92fr)_minmax(340px,1.08fr)]">
          <div className="grid gap-4">
            <div>
              <p className={text.eyebrow}>Once-over</p>
              <h2 className={text.sectionTitle}>Bring in a human pass when the draft is close</h2>
              <p className={text.p}>
                A one-time review adds a short walkthrough plus a few builder edits from the
                maker. Buy it when you want an outside eye on the first live version.
              </p>
            </div>

            <div className="grid gap-3 md:grid-cols-3">
              <StatusTile
                label="Status"
                value={humanOnceOverStatus(onceOver.status)}
                hint={
                  onceOverSubmittedLabel
                    ? `Ready since ${onceOverSubmittedLabel}`
                    : onceOverPaidLabel
                      ? `Purchased ${onceOverPaidLabel}`
                      : 'No request yet'
                }
              />
              <StatusTile
                label="Scope"
                value="One pass"
                hint="A recorded walkthrough plus 3 to 5 builder edits."
              />
              <StatusTile
                label="Turnaround"
                value="3 business days"
                hint="Starts once the intake is marked ready for review."
              />
            </div>

            {onceOver.status === 'none' || onceOver.status === 'delivered' ? (
              <div className={emptyState}>
                <p className={text.p}>
                  {onceOver.status === 'delivered'
                    ? 'This workspace already has a delivered pass. You can buy another if you want a fresh review after new edits.'
                    : 'No once-over is attached to this workspace yet.'}
                </p>
                <div className="mt-4">
                  <Button
                    type="button"
                    onClick={handleOnceOverCheckout}
                    disabled={isStartingOnceOverCheckout}
                  >
                    {isStartingOnceOverCheckout ? 'Opening checkout…' : onceOver.status === 'delivered' ? 'Buy another once-over' : 'Buy a once-over'}
                    <ArrowUpRight className="size-4" />
                  </Button>
                </div>
              </div>
            ) : null}

            {onceOver.status === 'pending' && onceOverRequest?.videoUrl ? (
              <div className="rounded-[14px] border border-border bg-[var(--surface-2)] p-4">
                <p className={text.label}>Current delivery link</p>
                <a
                  className="mt-2 inline-flex text-sm font-semibold text-[var(--thread-gold)] underline underline-offset-4"
                  href={onceOverRequest.videoUrl}
                  target="_blank"
                  rel="noreferrer"
                >
                  Open walkthrough
                </a>
              </div>
            ) : null}
          </div>

          <div className="grid gap-3 rounded-[16px] border border-border bg-[var(--surface-2)] p-5">
            <div>
              <p className={text.label}>Intake</p>
              <p className="mt-2 text-sm text-[var(--paper-muted)]">
                Tell the reviewer what the business does, who the visitor is, and what one
                outcome matters most.
              </p>
            </div>

            <div className={form.field}>
              <label htmlFor="once-over-business" className={text.label}>
                What does the business do?
              </label>
              <Textarea
                id="once-over-business"
                rows={3}
                value={onceOverBusiness}
                onChange={(event) => setOnceOverBusiness(event.target.value)}
                placeholder="Hand-dyed yarn for knitters who want richer color in their projects."
                disabled={onceOver.status === 'none' || onceOver.status === 'delivered'}
              />
            </div>

            <div className={form.field}>
              <label htmlFor="once-over-visitor" className={text.label}>
                Who is the visitor?
              </label>
              <Textarea
                id="once-over-visitor"
                rows={3}
                value={onceOverVisitor}
                onChange={(event) => setOnceOverVisitor(event.target.value)}
                placeholder="A knitter deciding whether this is the right indie dye studio to trust."
                disabled={onceOver.status === 'none' || onceOver.status === 'delivered'}
              />
            </div>

            <div className={form.field}>
              <label htmlFor="once-over-outcome" className={text.label}>
                What is the main outcome?
              </label>
              <Textarea
                id="once-over-outcome"
                rows={2}
                value={onceOverOutcome}
                onChange={(event) => setOnceOverOutcome(event.target.value)}
                placeholder="Get the first yarn order."
                disabled={onceOver.status === 'none' || onceOver.status === 'delivered'}
              />
            </div>

            <div className={form.field}>
              <label htmlFor="once-over-stuck" className={text.label}>
                What still feels stuck?
              </label>
              <Textarea
                id="once-over-stuck"
                rows={3}
                value={onceOverStuckOn}
                onChange={(event) => setOnceOverStuckOn(event.target.value)}
                placeholder="The proof is buried and the homepage still feels generic."
                disabled={onceOver.status === 'none' || onceOver.status === 'delivered'}
              />
            </div>

            {onceOverErrorMessage ? <p className={text.error}>{onceOverErrorMessage}</p> : null}
            {onceOverStatusMessage ? <p className={text.success}>{onceOverStatusMessage}</p> : null}

            <div className="flex flex-wrap gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() => handleSaveOnceOver(false)}
                disabled={isSavingOnceOver || onceOver.status === 'none' || onceOver.status === 'delivered'}
              >
                {isSavingOnceOver ? 'Saving intake…' : 'Save intake'}
              </Button>
              <Button
                type="button"
                onClick={() => handleSaveOnceOver(true)}
                disabled={isSavingOnceOver || onceOver.status === 'none' || onceOver.status === 'delivered'}
              >
                {isSavingOnceOver ? 'Saving intake…' : onceOver.status === 'pending' ? 'Update ready request' : 'Ready for review'}
              </Button>
            </div>

            {onceOver.status === 'awaiting_intake' ? (
              <p className={cn(form.hint, 'mt-1')}>
                Buying the once-over reserves the pass. The review clock starts after you mark the intake ready.
              </p>
            ) : null}
            {onceOver.status === 'pending' ? (
              <p className={cn(form.hint, 'mt-1')}>
                The request is in the review queue. You can still refine the draft while you wait.
              </p>
            ) : null}
          </div>
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

function StatusTile({ label, value, hint }: { label: string; value: string; hint: string }) {
  return (
    <div className="rounded-[12px] border border-border bg-[var(--surface-2)] p-4">
      <p className={text.label}>{label}</p>
      <p className="mt-2 text-2xl font-black text-[var(--paper)]">{value}</p>
      <p className="mt-2 text-sm text-[var(--paper-muted)]">{hint}</p>
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

function humanOnceOverStatus(status: BillingState['onceOver']['status']) {
  switch (status) {
    case 'awaiting_intake':
      return 'Awaiting intake'
    case 'pending':
      return 'Pending'
    case 'delivered':
      return 'Delivered'
    default:
      return 'Not purchased'
  }
}
