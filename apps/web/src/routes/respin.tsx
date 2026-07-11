import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useCallback, useEffect, useState } from 'react'
import { ArrowRight, ExternalLink, RefreshCw } from 'lucide-react'
import { SiteDraftRenderer } from '@/components/SiteDraftRenderer'
import { Button } from '@/components/ui/button'
import {
  APIError,
  claimRespin,
  getPreviewDraft,
  getRespinPreview,
  getRespinStatus,
  isTerminalRespinStatus,
  respinErrorCode,
  startAnonymousSession,
  startRespin,
  type RespinStatus,
  type SiteDraft,
} from '@/lib/api'
import { landingTheme } from '@/lib/landing-theme'
import { translator } from '@/lib/i18n'
import {
  DEFAULT_LOCALE,
  coerceLocale,
  ogLocale,
  resolveRequestLocale,
  useLocale,
} from '@/lib/locale'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/respin')({
  validateSearch: (search: Record<string, unknown>) => {
    const url =
      typeof search.url === 'string' && search.url.length > 0
        ? search.url
        : undefined
    const lang = coerceLocale(search.lang) ?? undefined
    return {
      ...(url ? { url } : {}),
      ...(lang ? { lang } : {}),
    }
  },
  loader: async () => ({ locale: await resolveRequestLocale() }),
  head: ({ loaderData }) => {
    const locale = coerceLocale(loaderData?.locale) ?? DEFAULT_LOCALE
    return {
      meta: [{ property: 'og:locale', content: ogLocale(locale) }],
    }
  },
  component: RespinDemo,
})

type Phase = 'input' | 'running' | 'after' | 'degraded' | 'error'

// The three visible pipeline steps a visitor watches (queued collapses into the
// first one). Maps the durable respin status to a progress index.
const PROGRESS_STEPS: Array<{ status: RespinStatus; key: ProgressKey }> = [
  { status: 'fetching', key: 'respin.progress.fetching' },
  { status: 'extracting', key: 'respin.progress.extracting' },
  { status: 'composing', key: 'respin.progress.composing' },
]

type ProgressKey =
  | 'respin.progress.fetching'
  | 'respin.progress.extracting'
  | 'respin.progress.composing'

const STATUS_ORDER: Record<RespinStatus, number> = {
  queued: 0,
  fetching: 1,
  extracting: 2,
  composing: 3,
  succeeded: 4,
  degraded: 4,
  failed: 4,
}

function RespinDemo() {
  const navigate = useNavigate()
  const locale = useLocale()
  const tr = translator(locale)
  const search = Route.useSearch()

  const [url, setUrl] = useState(search.url ?? '')
  const [phase, setPhase] = useState<Phase>('input')
  const [importId, setImportId] = useState('')
  const [status, setStatus] = useState<RespinStatus>('queued')
  const [errorMessage, setErrorMessage] = useState('')
  const [sourceUrl, setSourceUrl] = useState('')
  const [degraded, setDegraded] = useState(false)
  const [previewToken, setPreviewToken] = useState('')
  const [draft, setDraft] = useState<SiteDraft | null>(null)
  const [selectedPageId, setSelectedPageId] = useState<string | null>(null)
  const [draftError, setDraftError] = useState('')
  const [promptPrefill, setPromptPrefill] = useState('')

  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isContinuing, setIsContinuing] = useState(false)
  const [isClaimOpen, setIsClaimOpen] = useState(false)
  const [isClaiming, setIsClaiming] = useState(false)
  const [claimError, setClaimError] = useState('')

  // Resolve the terminal outcome: an "after" draft, a salvaged head-start
  // prompt, or a hard failure. Bound to the locale so the poll effect below has
  // a stable reference across status re-renders.
  const resolveTerminal = useCallback(
    async (id: string, terminalStatus: RespinStatus) => {
      const translate = translator(locale)
      if (terminalStatus === 'failed') {
        setErrorMessage(translate('respin.error.failed'))
        setPhase('error')
        return
      }
      try {
        const preview = await getRespinPreview(id)
        setSourceUrl(preview.source.url)
        setDegraded(preview.degraded)
        if (preview.after) {
          setPreviewToken(preview.after.previewToken)
          setPhase('after')
        } else {
          setPromptPrefill(preview.promptPrefill ?? '')
          setPhase('degraded')
        }
      } catch {
        setErrorMessage(translate('respin.error.generic'))
        setPhase('error')
      }
    },
    [locale],
  )

  // Poll the durable status endpoint while a run is in flight (Spec 21 demo UI:
  // the start POST and the watch are decoupled; the status endpoint is the
  // durable truth). Replaced-import / cache-hit runs land terminal on the first
  // poll and resolve immediately.
  useEffect(() => {
    if (phase !== 'running' || !importId) {
      return
    }
    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined

    async function poll() {
      try {
        const next = await getRespinStatus(importId)
        if (cancelled) {
          return
        }
        setStatus(next.status)
        if (isTerminalRespinStatus(next.status)) {
          resolveTerminal(importId, next.status)
          return
        }
      } catch {
        // Transient network/route error while the detached run continues:
        // keep polling on a slower cadence rather than failing the demo.
      }
      if (!cancelled) {
        timer = setTimeout(poll, 1500)
      }
    }

    poll()
    return () => {
      cancelled = true
      if (timer) {
        clearTimeout(timer)
      }
    }
  }, [phase, importId, resolveTerminal])

  // Load the generated draft once the after-view has a preview token.
  useEffect(() => {
    if (!previewToken) {
      return
    }
    let mounted = true
    getPreviewDraft(previewToken)
      .then((response) => {
        if (!mounted) {
          return
        }
        setDraft(response.draft)
        setSelectedPageId(resolvePreviewPageId(response.draft, null))
      })
      .catch(() => {
        if (mounted) {
          setDraftError(translator(locale)('respin.after.previewError'))
        }
      })
    return () => {
      mounted = false
    }
  }, [previewToken, locale])

  function resetToInput() {
    setPhase('input')
    setImportId('')
    setStatus('queued')
    setErrorMessage('')
    setSourceUrl('')
    setDegraded(false)
    setPreviewToken('')
    setDraft(null)
    setSelectedPageId(null)
    setDraftError('')
    setPromptPrefill('')
    setClaimError('')
    setIsClaimOpen(false)
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (!url.trim()) {
      return
    }
    setIsSubmitting(true)
    setErrorMessage('')
    try {
      const response = await startRespin({ url, locale })
      setImportId(response.importId)
      setStatus(response.status)
      setPhase('running')
    } catch (error) {
      setErrorMessage(messageForStartError(error, tr))
      setPhase('input')
    } finally {
      setIsSubmitting(false)
    }
  }

  // Degrade-to-prompt: drop the salvaged prefill into the ordinary homepage
  // prompt flow (Spec 21 graceful degradation) — a fresh trial session, the
  // same path the landing form takes.
  async function handleContinueWithPrefill() {
    setIsContinuing(true)
    setErrorMessage('')
    try {
      await startAnonymousSession({ freshIfBlocked: true, locale })
      await navigate({
        to: '/app',
        search: promptPrefill.trim() ? { prompt: promptPrefill.trim() } : {},
      })
    } catch (error) {
      setErrorMessage(messageForStartError(error, tr))
      setIsContinuing(false)
    }
  }

  async function handleClaim() {
    setIsClaiming(true)
    setClaimError('')
    try {
      const response = await claimRespin(importId)
      if (response.siteId) {
        await navigate({
          to: '/app/sites/$siteId',
          params: { siteId: response.siteId },
          search: { panel: undefined },
        })
      } else {
        await navigate({ to: '/app' })
      }
    } catch {
      setClaimError(tr('respin.claim.error'))
      setIsClaiming(false)
    }
  }

  return (
    <main
      className={cn(
        'min-h-screen bg-[var(--surface-0)] text-[var(--paper)] antialiased',
        '[font-family:"Be_Vietnam_Pro","Avenir_Next","Segoe_UI",sans-serif]',
      )}
      style={landingTheme}
    >
      <header className="mx-auto flex w-full max-w-[1180px] items-center justify-between gap-4 px-6 pt-7 md:px-8 md:pt-10">
        <Link
          to="/"
          className="inline-flex items-center gap-2.5 text-[15px] font-semibold tracking-tight text-[var(--paper)]"
        >
          <img src="/logo.png" alt="" className="size-7 object-contain" />
          snaelda
        </Link>
        <Link
          to="/"
          className="text-sm font-semibold text-[var(--paper-muted)] underline-offset-4 transition-colors hover:text-[var(--thread-gold)] hover:underline"
        >
          {tr('respin.nav.home')}
        </Link>
      </header>

      {phase === 'after' ? (
        <AfterView
          tr={tr}
          sourceUrl={sourceUrl}
          degraded={degraded}
          draft={draft}
          draftError={draftError}
          previewToken={previewToken}
          selectedPageId={selectedPageId}
          onNavigatePage={setSelectedPageId}
          onRespinAnother={resetToInput}
          onKeepIt={() => setIsClaimOpen(true)}
        />
      ) : (
        <section className="relative isolate mx-auto flex w-full max-w-[760px] flex-col items-center px-6 pb-24 pt-14 text-center md:pt-20">
          <div
            aria-hidden
            className="pointer-events-none absolute inset-x-0 -top-10 -z-10 h-[420px] bg-[radial-gradient(60%_80%_at_50%_0%,color-mix(in_oklch,var(--thread-mauve)_20%,transparent)_0%,transparent_72%)]"
          />
          <p className="text-xs font-bold uppercase tracking-[0.18em] text-[color-mix(in_oklch,var(--thread-mauve)_72%,var(--paper))]">
            {tr('respin.eyebrow')}
          </p>
          <h1 className='mt-4 max-w-2xl text-[clamp(2.2rem,5.4vw,3.4rem)] font-semibold leading-[1.02] tracking-[-0.02em] text-[color-mix(in_oklch,var(--thread-mauve)_70%,white)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
            {tr('respin.title')}
          </h1>
          <p className="mt-5 max-w-xl text-[1.02rem] leading-8 text-[var(--paper-muted)]">
            {tr('respin.subtitle')}
          </p>

          {phase === 'input' || phase === 'error' ? (
            <InputView
              tr={tr}
              url={url}
              onUrlChange={setUrl}
              onSubmit={handleSubmit}
              isSubmitting={isSubmitting}
              errorMessage={errorMessage}
            />
          ) : null}

          {phase === 'running' ? (
            <ProgressView tr={tr} status={status} sourceUrl={url} />
          ) : null}

          {phase === 'degraded' ? (
            <DegradedView
              tr={tr}
              sourceUrl={sourceUrl}
              onContinue={handleContinueWithPrefill}
              onRetry={resetToInput}
              isContinuing={isContinuing}
              errorMessage={errorMessage}
            />
          ) : null}
        </section>
      )}

      {isClaimOpen ? (
        <ClaimModal
          tr={tr}
          isClaiming={isClaiming}
          claimError={claimError}
          onConfirm={handleClaim}
          onCancel={() => {
            if (!isClaiming) {
              setIsClaimOpen(false)
              setClaimError('')
            }
          }}
        />
      ) : null}
    </main>
  )
}

type Translate = ReturnType<typeof translator>

function InputView({
  tr,
  url,
  onUrlChange,
  onSubmit,
  isSubmitting,
  errorMessage,
}: {
  tr: Translate
  url: string
  onUrlChange: (value: string) => void
  onSubmit: (event: FormEvent) => void
  isSubmitting: boolean
  errorMessage: string
}) {
  return (
    <>
      <form
        className="mt-10 flex w-full max-w-2xl flex-col items-stretch gap-3 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_56%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_88%,transparent)] p-3 shadow-[0_24px_70px_-22px_oklch(16%_0.05_336_/_0.6)] backdrop-blur-sm transition-colors duration-300 focus-within:border-[color-mix(in_oklch,var(--thread-teal)_70%,transparent)] md:flex-row md:items-center md:p-2.5 md:pl-3"
        onSubmit={onSubmit}
      >
        <input
          type="url"
          inputMode="url"
          autoComplete="url"
          value={url}
          onChange={(event) => onUrlChange(event.target.value)}
          placeholder={tr('respin.form.placeholder')}
          aria-label={tr('respin.form.ariaLabel')}
          className="min-h-14 w-full rounded-[14px] border border-transparent bg-transparent px-4 py-3.5 text-base text-[var(--paper)] outline-none placeholder:text-[color-mix(in_oklch,var(--paper-muted)_60%,transparent)] md:text-lg"
        />
        <Button
          type="submit"
          size="lg"
          variant="plain"
          disabled={isSubmitting || !url.trim()}
          className="min-h-14 shrink-0 rounded-[14px] bg-[var(--thread-gold)] px-7 text-[var(--ink)] shadow-[0_10px_24px_-8px_oklch(78%_0.11_68_/_0.55)] transition-transform duration-200 hover:-translate-y-px hover:bg-[color-mix(in_oklch,var(--thread-gold)_84%,white)] disabled:opacity-70"
        >
          <RefreshCw className="size-4.5" />
          {isSubmitting ? tr('respin.form.submitBusy') : tr('respin.form.submit')}
        </Button>
      </form>

      {errorMessage ? (
        <p className="mt-4 max-w-xl text-sm text-[var(--thread-coral)]" role="alert">
          {errorMessage}
        </p>
      ) : null}

      <p className="mt-4 max-w-xl text-sm text-[color-mix(in_oklch,var(--paper-muted)_76%,transparent)]">
        {tr('respin.form.helper')}
      </p>
    </>
  )
}

function ProgressView({
  tr,
  status,
  sourceUrl,
}: {
  tr: Translate
  status: RespinStatus
  sourceUrl: string
}) {
  const current = STATUS_ORDER[status]
  return (
    <div className="mt-12 flex w-full max-w-md flex-col items-stretch gap-6">
      <div className="flex flex-col items-center gap-2">
        <img
          src="/logo.png"
          alt=""
          aria-hidden
          className="size-16 object-contain animate-[spin_3s_linear_infinite] motion-reduce:animate-none"
        />
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--thread-mauve)]">
          {tr('respin.progress.eyebrow')}
        </p>
        <p className='font-serif text-[clamp(1.3rem,3vw,1.7rem)] font-semibold text-[var(--paper)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
          {tr('respin.progress.title')}
        </p>
        {sourceUrl ? (
          <p className="max-w-full truncate text-sm text-[var(--paper-muted)]">
            <span className="text-[color-mix(in_oklch,var(--paper-muted)_70%,transparent)]">
              {tr('respin.progress.sourceLabel')}{' '}
            </span>
            {sourceUrl}
          </p>
        ) : null}
      </div>

      <ol className="flex flex-col gap-3 text-left">
        {PROGRESS_STEPS.map((step) => {
          const stepIndex = STATUS_ORDER[step.status]
          const state =
            current > stepIndex
              ? 'done'
              : current === stepIndex
                ? 'active'
                : 'pending'
          return (
            <li
              key={step.status}
              className={cn(
                'flex items-center gap-3 rounded-[12px] border px-4 py-3 text-sm font-semibold transition-colors',
                state === 'done' &&
                  'border-[color-mix(in_oklch,var(--thread-teal)_44%,var(--border))] bg-[color-mix(in_oklch,var(--thread-teal)_10%,var(--surface-1))] text-[var(--paper)]',
                state === 'active' &&
                  'border-[color-mix(in_oklch,var(--thread-gold)_54%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_12%,var(--surface-1))] text-[var(--paper)]',
                state === 'pending' &&
                  'border-[color-mix(in_oklch,var(--border)_50%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_70%,transparent)] text-[var(--paper-muted)]',
              )}
            >
              <span
                aria-hidden
                className={cn(
                  'grid size-6 shrink-0 place-items-center rounded-full text-xs',
                  state === 'done' &&
                    'bg-[var(--thread-teal)] text-[var(--ink)]',
                  state === 'active' &&
                    'bg-[var(--thread-gold)] text-[var(--ink)] motion-safe:animate-pulse',
                  state === 'pending' &&
                    'border border-[var(--border)] text-[var(--paper-muted)]',
                )}
              >
                {state === 'done' ? '✓' : PROGRESS_STEPS.indexOf(step) + 1}
              </span>
              {tr(step.key)}
            </li>
          )
        })}
      </ol>
    </div>
  )
}

function DegradedView({
  tr,
  sourceUrl,
  onContinue,
  onRetry,
  isContinuing,
  errorMessage,
}: {
  tr: Translate
  sourceUrl: string
  onContinue: () => void
  onRetry: () => void
  isContinuing: boolean
  errorMessage: string
}) {
  return (
    <div className="mt-12 flex w-full max-w-lg flex-col items-center gap-5 rounded-[18px] border border-[color-mix(in_oklch,var(--thread-gold)_38%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_8%,var(--surface-1))] px-6 py-8 text-center">
      <p className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--thread-gold)]">
        {tr('respin.degraded.eyebrow')}
      </p>
      <p className='font-serif text-[clamp(1.4rem,3vw,1.9rem)] font-semibold text-[var(--paper)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
        {tr('respin.degraded.title')}
      </p>
      <p className="max-w-md text-sm leading-7 text-[var(--paper-muted)]">
        {tr('respin.degraded.body')}
      </p>
      {sourceUrl ? (
        <a
          href={sourceUrl}
          target="_blank"
          rel="noopener noreferrer nofollow"
          className="inline-flex items-center gap-1.5 text-sm font-semibold text-[var(--thread-teal)] underline-offset-4 hover:underline"
        >
          {sourceUrl}
          <ExternalLink aria-hidden className="size-3.5" />
        </a>
      ) : null}
      {errorMessage ? (
        <p className="text-sm text-[var(--thread-coral)]" role="alert">
          {errorMessage}
        </p>
      ) : null}
      <div className="flex flex-col gap-3 sm:flex-row">
        <Button
          type="button"
          size="lg"
          variant="plain"
          disabled={isContinuing}
          onClick={onContinue}
          className="min-h-12 rounded-full bg-[var(--thread-gold)] px-6 text-[var(--ink)] transition-transform hover:-translate-y-px hover:bg-[color-mix(in_oklch,var(--thread-gold)_84%,white)] disabled:opacity-70"
        >
          {isContinuing
            ? tr('respin.degraded.busy')
            : tr('respin.degraded.cta')}
          <ArrowRight aria-hidden className="size-4" />
        </Button>
        <button
          type="button"
          onClick={onRetry}
          className="min-h-12 rounded-full border border-[color-mix(in_oklch,var(--border)_60%,transparent)] px-6 text-sm font-semibold text-[var(--paper-muted)] transition-colors hover:border-[var(--thread-teal)] hover:text-[var(--paper)]"
        >
          {tr('respin.degraded.retry')}
        </button>
      </div>
    </div>
  )
}

function AfterView({
  tr,
  sourceUrl,
  degraded,
  draft,
  draftError,
  previewToken,
  selectedPageId,
  onNavigatePage,
  onRespinAnother,
  onKeepIt,
}: {
  tr: Translate
  sourceUrl: string
  degraded: boolean
  draft: SiteDraft | null
  draftError: string
  previewToken: string
  selectedPageId: string | null
  onNavigatePage: (pageId: string) => void
  onRespinAnother: () => void
  onKeepIt: () => void
}) {
  return (
    <section className="relative mt-8 w-full">
      <div className="sticky top-0 z-40 border-y border-[color-mix(in_oklch,var(--border)_40%,transparent)] bg-[color-mix(in_oklch,var(--surface-0)_92%,transparent)] backdrop-blur-md">
        <div className="mx-auto flex w-full max-w-[1180px] flex-col gap-3 px-6 py-3 md:flex-row md:items-center md:justify-between md:px-8">
          <div className="flex min-w-0 flex-col gap-1">
            <p className="text-xs font-bold uppercase tracking-[0.14em] text-[var(--thread-mauve)]">
              {tr('respin.after.eyebrow')}
            </p>
            {sourceUrl ? (
              <a
                href={sourceUrl}
                target="_blank"
                rel="noopener noreferrer nofollow"
                className="inline-flex items-center gap-1.5 truncate text-sm font-semibold text-[var(--thread-teal)] underline-offset-4 hover:underline"
              >
                {tr('respin.after.sourceLink')}
                <ExternalLink aria-hidden className="size-3.5 shrink-0" />
              </a>
            ) : null}
          </div>
          <div className="flex shrink-0 items-center gap-2.5">
            <button
              type="button"
              onClick={onRespinAnother}
              className="inline-flex min-h-11 items-center gap-2 rounded-full border border-[color-mix(in_oklch,var(--border)_60%,transparent)] px-4 text-sm font-semibold text-[var(--paper-muted)] transition-colors hover:border-[var(--thread-teal)] hover:text-[var(--paper)]"
            >
              <RefreshCw aria-hidden className="size-4" />
              {tr('respin.after.respinAnother')}
            </button>
            <Button
              type="button"
              size="lg"
              variant="plain"
              onClick={onKeepIt}
              className="min-h-11 rounded-full bg-[var(--thread-gold)] px-5 text-[var(--ink)] transition-transform hover:-translate-y-px hover:bg-[color-mix(in_oklch,var(--thread-gold)_84%,white)]"
            >
              {tr('respin.after.keepIt')}
              <ArrowRight aria-hidden className="size-4" />
            </Button>
          </div>
        </div>
        <div className="mx-auto w-full max-w-[1180px] px-6 pb-3 md:px-8">
          <p className="text-xs text-[color-mix(in_oklch,var(--paper-muted)_82%,transparent)]">
            {degraded ? tr('respin.after.degradedNote') : tr('respin.after.note')}
          </p>
        </div>
      </div>

      <div className="relative min-h-[60vh] w-full">
        {draftError ? (
          <div
            role="alert"
            className="grid min-h-[60vh] place-items-center px-6 text-center"
          >
            <p className="max-w-[44ch] font-serif text-[clamp(1.3rem,2.6vw,1.8rem)] font-bold text-[var(--paper)]">
              {draftError}
            </p>
          </div>
        ) : draft ? (
          <>
            <SiteDraftRenderer
              site={draft}
              eyebrow={tr('respin.after.badge')}
              showPageMeta={false}
              selectedPageId={selectedPageId ?? undefined}
              previewToken={previewToken}
              onNavigatePage={onNavigatePage}
            />
            <div
              aria-label={tr('respin.after.badge')}
              className="pointer-events-none fixed bottom-5 right-5 z-50 rounded-full border border-[color-mix(in_oklch,var(--thread-gold)_42%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_88%,transparent)] px-3.5 py-1.5 text-[0.7rem] font-bold uppercase tracking-[0.16em] text-[var(--thread-gold)] backdrop-blur-md"
            >
              {tr('respin.after.badge')}
            </div>
          </>
        ) : (
          <div
            aria-busy="true"
            className="grid min-h-[60vh] place-items-center text-[var(--paper-muted)]"
          >
            <p className="text-sm">{tr('respin.after.loadingPreview')}</p>
          </div>
        )}
      </div>
    </section>
  )
}

function ClaimModal({
  tr,
  isClaiming,
  claimError,
  onConfirm,
  onCancel,
}: {
  tr: Translate
  isClaiming: boolean
  claimError: string
  onConfirm: () => void
  onCancel: () => void
}) {
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={tr('respin.claim.title')}
      className="fixed inset-0 z-[60] grid place-items-center bg-[color-mix(in_oklch,black_62%,transparent)] p-6 backdrop-blur-sm"
      onClick={onCancel}
    >
      <div
        className="w-full max-w-md rounded-[20px] border border-[color-mix(in_oklch,var(--border)_60%,transparent)] bg-[var(--surface-1)] p-7 text-center shadow-[0_30px_80px_-20px_oklch(10%_0.04_336_/_0.7)]"
        onClick={(event) => event.stopPropagation()}
      >
        <p className='font-serif text-[1.5rem] font-semibold text-[var(--paper)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
          {tr('respin.claim.title')}
        </p>
        <p className="mt-3 text-sm leading-7 text-[var(--paper-muted)]">
          {tr('respin.claim.body')}
        </p>
        {claimError ? (
          <p className="mt-4 text-sm text-[var(--thread-coral)]" role="alert">
            {claimError}
          </p>
        ) : null}
        <div className="mt-6 flex flex-col gap-3">
          <Button
            type="button"
            size="lg"
            variant="plain"
            disabled={isClaiming}
            onClick={onConfirm}
            className="min-h-12 rounded-full bg-[var(--thread-gold)] px-6 text-[var(--ink)] transition-transform hover:-translate-y-px hover:bg-[color-mix(in_oklch,var(--thread-gold)_84%,white)] disabled:opacity-70"
          >
            {isClaiming ? tr('respin.claim.busy') : tr('respin.claim.confirm')}
            <ArrowRight aria-hidden className="size-4" />
          </Button>
          <button
            type="button"
            onClick={onCancel}
            disabled={isClaiming}
            className="min-h-11 rounded-full px-6 text-sm font-semibold text-[var(--paper-muted)] transition-colors hover:text-[var(--paper)] disabled:opacity-60"
          >
            {tr('respin.claim.cancel')}
          </button>
        </div>
      </div>
    </div>
  )
}

function messageForStartError(error: unknown, tr: Translate): string {
  const code = respinErrorCode(error)
  switch (code) {
    case 'invalid_url':
      return tr('respin.error.invalidUrl')
    case 'rate_limited':
      return tr('respin.error.rateLimited')
    case 'respin_busy':
      return tr('respin.error.busy')
    default:
      break
  }
  if (error instanceof APIError && error.message) {
    return error.message
  }
  return tr('respin.error.generic')
}

function resolvePreviewPageId(
  draft: SiteDraft,
  preferredPageId: string | null,
) {
  if (
    preferredPageId &&
    draft.pages.some((page) => page.id === preferredPageId)
  ) {
    return preferredPageId
  }
  return (
    draft.pages.find((page) => page.slug === '/')?.id ??
    draft.pages[0]?.id ??
    null
  )
}
