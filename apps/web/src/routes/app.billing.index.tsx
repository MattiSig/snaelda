import { createFileRoute } from '@tanstack/react-router'
import { ArrowUpRight, Check, CreditCard, Mail, Minus } from 'lucide-react'
import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  APIError,
  createBillingCheckout,
  createBillingPortal,
  getBillingState,
  getCurrentSession,
  updateOnceOver,
  type BillingPlan,
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
  const [isStartingCheckout, setIsStartingCheckout] = useState<'site' | 'pro' | ''>('')
  const [isStartingOnceOverCheckout, setIsStartingOnceOverCheckout] = useState(false)
  const [isOpeningPortal, setIsOpeningPortal] = useState(false)
  const [onceOverBusiness, setOnceOverBusiness] = useState('')
  const [onceOverVisitor, setOnceOverVisitor] = useState('')
  const [onceOverOutcome, setOnceOverOutcome] = useState('')
  const [onceOverStuckOn, setOnceOverStuckOn] = useState('')
  const [onceOverErrorMessage, setOnceOverErrorMessage] = useState('')
  const [onceOverStatusMessage, setOnceOverStatusMessage] = useState('')
  const [isSavingOnceOver, setIsSavingOnceOver] = useState(false)
  const [claimEmail, setClaimEmail] = useState('')

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
        setClaimEmail(nextSession.user?.email ?? '')
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

  async function handleCheckout(plan: 'site' | 'pro') {
    setIsStartingCheckout(plan)
    setErrorMessage('')
    setStatusMessage('')

    try {
      const response = await createBillingCheckout({ plan, email: claimEmail.trim() || undefined })
      window.location.assign(response.url)
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
      window.location.assign(response.url)
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
      const response = await createBillingCheckout({
        purchaseType: 'once_over',
        email: claimEmail.trim() || undefined,
      })
      window.location.assign(response.url)
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
  const { catalog } = billingState
  const onceOver = billingState.onceOver
  const trialEndsLabel = session.trialExpiresAt
    ? new Date(session.trialExpiresAt).toLocaleDateString()
    : ''
  const hasClaimedEmail = Boolean(session.user?.email)
  const needsEmailToCheckout = !hasClaimedEmail
  const isEmailValid = isLikelyEmail(claimEmail)
  const checkoutBlockedByEmail = needsEmailToCheckout && !isEmailValid
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

        {needsEmailToCheckout ? (
          <div className="mt-6 grid gap-3 rounded-[16px] border border-[color-mix(in_oklch,var(--thread-mauve)_38%,var(--border))] bg-[color-mix(in_oklch,var(--thread-mauve)_8%,var(--surface-2))] p-5">
            <div className="flex items-start gap-3">
              <span className="mt-1 inline-flex size-9 shrink-0 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--thread-mauve)_22%,var(--surface-1))] text-[var(--thread-mauve)]">
                <Mail aria-hidden className="size-4" />
              </span>
              <div className="grid gap-1">
                <p className={text.label}>Save your workspace before you pay</p>
                <p className="text-sm text-[var(--paper-muted)]">
                  Receipts go to this address, and you sign in from any browser with
                  a magic link to it.
                </p>
              </div>
            </div>
            <Input
              type="email"
              autoComplete="email"
              inputMode="email"
              value={claimEmail}
              onChange={(event) => setClaimEmail(event.target.value)}
              placeholder="you@example.com"
              aria-label="Email for receipts and login"
              aria-invalid={claimEmail.length > 0 && !isEmailValid}
              disabled={isStartingCheckout !== '' || isStartingOnceOverCheckout}
            />
            {claimEmail.length > 0 && !isEmailValid ? (
              <p className={text.error}>That doesn’t look like a valid email yet.</p>
            ) : null}
          </div>
        ) : null}

        <div className="mt-6 grid gap-4 lg:grid-cols-2">
          {catalog.plans.map((plan) => {
            const isCurrent = entitlement.plan === plan.id && entitlement.subscriptionLive
            const isLoading = isStartingCheckout === plan.id
            return (
              <PlanCard
                key={plan.id}
                plan={plan}
                ctaLabel={
                  isCurrent
                    ? 'Current plan'
                    : isLoading
                      ? 'Opening checkout…'
                      : checkoutBlockedByEmail
                        ? 'Add an email above'
                        : `Choose ${plan.name}`
                }
                disabled={isCurrent || isStartingCheckout !== '' || checkoutBlockedByEmail}
                onSelect={() => handleCheckout(plan.id)}
              />
            )
          })}
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
              value={entitlement.customDomainsEnabled ? 'Included' : 'Upgrade to unlock'}
              action={
                !entitlement.customDomainsEnabled ? (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => handleCheckout('site')}
                    disabled={isStartingCheckout !== '' || checkoutBlockedByEmail}
                  >
                    Upgrade
                  </Button>
                ) : undefined
              }
            />
          </div>

          {entitlement.subscriptionLive ? (
            <Button type="button" variant="outline" onClick={handlePortal} disabled={isOpeningPortal}>
              <CreditCard className="size-4" />
              {isOpeningPortal ? 'Opening portal…' : 'Manage billing'}
            </Button>
          ) : hasClaimedEmail ? (
            <div className={emptyState}>
              <p className={text.p}>
                Subscribing as <span className="font-semibold text-[var(--paper)]">{session.user?.email}</span>.
                Receipts and login go here.
              </p>
            </div>
          ) : null}
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
                label="Price"
                value={formatPrice(catalog.onceOverPrices)}
                hint="One-time charge. Buy as many passes as you like."
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

            <div className="rounded-[12px] border border-border bg-[var(--surface-2)] px-4 py-3">
              <p className="flex items-center justify-between gap-3 text-sm">
                <span className={text.label}>Status</span>
                <span className="font-semibold text-[var(--paper)]">
                  {humanOnceOverStatus(onceOver.status)}
                  {onceOverSubmittedLabel
                    ? ` · ready since ${onceOverSubmittedLabel}`
                    : onceOverPaidLabel
                      ? ` · purchased ${onceOverPaidLabel}`
                      : ''}
                </span>
              </p>
            </div>

            {onceOver.status === 'none' || onceOver.status === 'delivered' ? (
              <div className={emptyState}>
                <p className={text.p}>
                  {onceOver.status === 'delivered'
                    ? 'This workspace already has a delivered pass. You can buy another if you want a fresh review after new edits.'
                    : 'Fill in the intake on the right to see what reviewers will read. Buy when you are ready.'}
                </p>
                <div className="mt-4">
                  <Button
                    type="button"
                    onClick={handleOnceOverCheckout}
                    disabled={isStartingOnceOverCheckout || checkoutBlockedByEmail}
                  >
                    {isStartingOnceOverCheckout
                      ? 'Opening checkout…'
                      : checkoutBlockedByEmail
                        ? 'Add an email above'
                        : onceOver.status === 'delivered'
                          ? `Buy another once-over · ${formatPrice(catalog.onceOverPrices)}`
                          : `Buy once-over · ${formatPrice(catalog.onceOverPrices)}`}
                    <ArrowUpRight className="size-4" />
                  </Button>
                </div>
              </div>
            ) : null}

            {(onceOver.status === 'pending' || onceOver.status === 'delivered') && onceOverRequest?.videoUrl ? (
              <div className="rounded-[14px] border border-border bg-[var(--surface-2)] p-4">
                <p className={text.label}>
                  {onceOver.status === 'delivered' ? 'Your walkthrough' : 'Current delivery link'}
                </p>
                <a
                  className="mt-2 inline-flex text-sm font-semibold text-[var(--thread-gold)] underline underline-offset-4"
                  href={onceOverRequest.videoUrl}
                  target="_blank"
                  rel="noreferrer"
                >
                  Open walkthrough
                </a>
                {onceOverRequest.deliveryNextSteps?.length ? (
                  <ol className="mt-4 grid gap-2 pl-5 text-sm text-[var(--paper-muted)]">
                    {onceOverRequest.deliveryNextSteps.map((step, index) => (
                      <li key={`${step}-${index}`} className="list-decimal">
                        {step}
                      </li>
                    ))}
                  </ol>
                ) : null}
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
                disabled={isSavingOnceOver}
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
                disabled={isSavingOnceOver}
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
                disabled={isSavingOnceOver}
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
                disabled={isSavingOnceOver}
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

            {onceOver.status === 'none' || onceOver.status === 'delivered' ? (
              <p className={cn(form.hint, 'mt-1')}>
                Saving is enabled after you buy a once-over. Use the field above to preview what the reviewer will read.
              </p>
            ) : null}
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
  plan,
  ctaLabel,
  disabled,
  onSelect,
}: {
  plan: BillingPlan
  ctaLabel: string
  disabled: boolean
  onSelect: () => void
}) {
  const isRecommended = plan.id === 'site'
  return (
    <article
      className={cn(
        'grid gap-5 rounded-[16px] border bg-[var(--surface-2)] p-6',
        isRecommended
          ? 'border-[color-mix(in_oklch,var(--thread-gold)_55%,var(--border))] shadow-[0_0_0_1px_color-mix(in_oklch,var(--thread-gold)_22%,transparent)]'
          : 'border-border',
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="grid gap-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-lg font-black text-[var(--paper)]">{plan.name}</p>
            {isRecommended ? (
              <span className="rounded-full border border-[color-mix(in_oklch,var(--thread-gold)_70%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_18%,var(--surface-1))] px-2 py-0.5 text-[10px] font-bold uppercase tracking-[0.1em] text-[var(--paper)]">
                Recommended
              </span>
            ) : null}
          </div>
          <p className="text-sm text-[var(--paper-muted)]">{planTagline(plan)}</p>
        </div>
      </div>
      <div className="grid gap-1">
        <p className="flex items-baseline gap-2">
          <span className="text-[2.4rem] font-black leading-none tabular-nums text-[var(--paper)]">
            {formatPrice(plan.prices)}
          </span>
          <span className="text-sm text-[var(--paper-muted)]">/ month</span>
        </p>
      </div>
      <ul className="grid gap-2 text-sm">
        {planFeatures(plan).map((feature) => (
          <li key={feature.label} className="flex items-center gap-2">
            {feature.included ? (
              <Check
                className={cn(
                  'size-4 shrink-0',
                  plan.id === 'pro' ? 'text-[var(--thread-teal)]' : 'text-[var(--thread-gold)]',
                )}
                aria-hidden="true"
              />
            ) : (
              <Minus
                className="size-4 shrink-0 text-[var(--paper-muted)]"
                aria-hidden="true"
              />
            )}
            <span
              className={cn(
                feature.included
                  ? 'text-[var(--paper)]'
                  : 'text-[var(--paper-muted)] line-through',
              )}
            >
              {feature.label}
            </span>
          </li>
        ))}
      </ul>
      <Button
        type="button"
        onClick={onSelect}
        disabled={disabled}
        variant={isRecommended ? 'default' : 'outline'}
      >
        {ctaLabel}
        <ArrowUpRight className="size-4" />
      </Button>
    </article>
  )
}

function UsageRow({
  label,
  value,
  action,
}: {
  label: string
  value: string
  action?: ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-[12px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_42%,transparent)] px-4 py-3">
      <p className={text.label}>{label}</p>
      <div className="flex items-center gap-3">
        <p className="text-right text-sm font-semibold text-[var(--paper)]">{value}</p>
        {action}
      </div>
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

function isLikelyEmail(value: string) {
  const trimmed = value.trim()
  if (trimmed.length < 5) {
    return false
  }
  const at = trimmed.indexOf('@')
  if (at <= 0 || at !== trimmed.lastIndexOf('@')) {
    return false
  }
  const dot = trimmed.indexOf('.', at)
  return dot > at + 1 && dot < trimmed.length - 1
}

function humanPlan(plan: string) {
  switch (plan) {
    case 'pro':
      return 'Pro'
    case 'site':
      return 'Site'
    default:
      return 'Trial'
  }
}

// resolvePlanPrice picks the price to display from a per-currency map, preferring
// ISK (the Phase 0 currency) and falling back to whatever single currency the
// catalog carries.
function resolvePlanPrice(
  prices: Record<string, number> | null | undefined,
): { currency: string; amount: number } | null {
  // Tolerate a missing map so an older API payload degrades to "—" instead of
  // crashing the billing page during a deploy skew.
  if (!prices) {
    return null
  }
  if (prices.ISK !== undefined) {
    return { currency: 'ISK', amount: prices.ISK }
  }
  const [currency] = Object.keys(prices)
  if (!currency) {
    return null
  }
  return { currency, amount: prices[currency] }
}

function formatCurrency(currency: string, amount: number) {
  try {
    return new Intl.NumberFormat('is-IS', {
      style: 'currency',
      currency,
      maximumFractionDigits: currency === 'ISK' ? 0 : 2,
    }).format(amount)
  } catch {
    return `${amount} ${currency}`
  }
}

function formatPrice(prices: Record<string, number> | null | undefined) {
  const resolved = resolvePlanPrice(prices)
  return resolved ? formatCurrency(resolved.currency, resolved.amount) : '—'
}

function planFeatures(plan: BillingPlan) {
  return [
    { label: `${plan.monthlyPromptLimit} prompts / month`, included: true },
    { label: `${plan.activeSiteLimit} active sites`, included: true },
    { label: `${formatBytes(plan.assetStorageLimitBytes)} asset storage`, included: true },
    { label: `${plan.collectionLimit} collections`, included: true },
    { label: `${plan.collectionEntryLimit} collection entries / detail URLs`, included: true },
    { label: 'Custom domains', included: plan.customDomainsEnabled },
    { label: 'Priority once-over slots', included: plan.priorityOnceOver },
  ]
}

function planTagline(plan: BillingPlan) {
  switch (plan.id) {
    case 'pro':
      return 'Multiple sites or heavier iteration on one.'
    default:
      return 'A small business site and its next rounds of edits.'
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
