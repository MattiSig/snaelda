import { Link, createFileRoute } from '@tanstack/react-router'
import { ArrowRight, Mail, ShieldCheck } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { getCurrentSession, type BuilderSession } from '@/lib/api'
import { paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/app/billing/success')({
  component: BillingSuccessPage,
})

function BillingSuccessPage() {
  const [session, setSession] = useState<BuilderSession | null>(null)
  const [stillConfirming, setStillConfirming] = useState(true)
  const stopRef = useRef(false)

  useEffect(() => {
    stopRef.current = false

    let attempt = 0
    const maxAttempts = 12 // ~14s of polling with 1.2s base interval

    async function poll() {
      if (stopRef.current) {
        return
      }
      try {
        const next = await getCurrentSession({ retryOnUnauthorized: false })
        if (stopRef.current) {
          return
        }
        setSession(next)
        if (next.user?.email) {
          setStillConfirming(false)
          return
        }
      } catch {
        // Webhook hasn't claimed yet; keep polling.
      }
      attempt += 1
      if (attempt >= maxAttempts) {
        setStillConfirming(false)
        return
      }
      window.setTimeout(poll, 1200)
    }

    void poll()

    return () => {
      stopRef.current = true
    }
  }, [])

  const claimedEmail = session?.user?.email
  const isClaimed = Boolean(claimedEmail)

  return (
    <section className={cn(paddedPanel, 'rounded-[14px] grid gap-6')}>
      <div className="grid gap-3">
        <p className={text.eyebrow}>Checkout complete</p>
        <h1 className={text.sectionTitle}>
          {isClaimed ? 'Your workshop is yours.' : 'Stripe has handed the session back.'}
        </h1>
      </div>

      {isClaimed ? (
        <div className="grid gap-4 rounded-[16px] border border-[color-mix(in_oklch,var(--thread-teal)_42%,var(--border))] bg-[color-mix(in_oklch,var(--thread-teal)_10%,var(--surface-2))] p-5">
          <div className="flex items-start gap-3">
            <span className="mt-0.5 inline-flex size-9 shrink-0 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--thread-teal)_22%,var(--surface-1))] text-[var(--thread-teal)]">
              <ShieldCheck aria-hidden className="size-4" />
            </span>
            <div className="grid gap-1">
              <p className={text.label}>Workspace saved under</p>
              <p className="break-all text-lg font-black text-[var(--paper)]">
                {claimedEmail}
              </p>
            </div>
          </div>
          <div className="grid gap-2 text-sm text-[var(--paper-muted)]">
            <p className="flex items-start gap-2">
              <Mail aria-hidden className="mt-0.5 size-4 shrink-0 text-[var(--thread-mauve)]" />
              <span>
                We sent a confirmation. Next time you visit, use a magic link to
                that address.
              </span>
            </p>
          </div>
        </div>
      ) : stillConfirming ? (
        <div className="rounded-[16px] border border-border bg-[var(--surface-2)] p-5">
          <p className={text.p}>
            Finalizing your subscription with Stripe. This usually takes a
            moment.
          </p>
          <p className={cn(text.p, 'mt-2 text-[var(--paper-muted)]')}>
            Keep this tab open. We’ll show your account email here as soon as
            it’s saved.
          </p>
        </div>
      ) : (
        <div className="rounded-[16px] border border-border bg-[var(--surface-2)] p-5">
          <p className={text.p}>
            Stripe accepted your payment. Snaelda is still catching up on the
            confirmation. Refresh this page in a moment, or open the builder
            and check back here.
          </p>
        </div>
      )}

      <div className="flex flex-wrap gap-3">
        <Button asChild>
          <Link to="/app">
            Open builder
            <ArrowRight className="size-4" />
          </Link>
        </Button>
        <Button asChild variant="outline">
          <Link to="/app/billing">Open billing</Link>
        </Button>
        {isClaimed ? (
          <Button asChild variant="plain">
            <Link to="/login">How to sign in next time</Link>
          </Button>
        ) : null}
      </div>
    </section>
  )
}
