import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { CSSProperties, FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { ArrowRight, Globe, Sparkles } from 'lucide-react'
import { HeroDemo } from '@/components/HeroDemo'
import { HeroScrollCue } from '@/components/HeroScrollCue'
import { PublishedSitePage } from '@/components/PublishedSitePage'
import {
  APIError,
  getCurrentSession,
  getPublishedSiteByHostname,
  restoreWorkspace,
  startAnonymousSession,
  type BuilderSession,
} from '@/lib/api'
import {
  buildPublishedPageHead,
  loadPublishedSitePageData,
} from '@/lib/published-site'
import { getHostedPublicSiteContext } from '@/lib/public-site'
import { translator, type Locale } from '@/lib/i18n'
import {
  DEFAULT_LOCALE,
  coerceLocale,
  ogLocale,
  resolveRequestLocale,
  useLocale,
} from '@/lib/locale'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/')({
  validateSearch: (search: Record<string, unknown>) => {
    const restore =
      typeof search.restore === 'string' && search.restore.length > 0
        ? search.restore
        : undefined
    const lang = coerceLocale(search.lang) ?? undefined
    return {
      ...(restore ? { restore } : {}),
      ...(lang ? { lang } : {}),
    }
  },
  loader: async () => {
    const [hostedPublic, locale] = await Promise.all([
      getHostedPublicSiteContext(),
      resolveRequestLocale(),
    ])
    return {
      locale,
      hostedPublic,
      published: hostedPublic.isHostedPublic
        ? await loadPublishedSitePageData(() =>
            getPublishedSiteByHostname(
              hostedPublic.hostname,
              hostedPublic.pagePath,
            ),
          )
        : { site: null, errorMessage: '' },
    }
  },
  head: ({ loaderData }) => {
    const base = buildPublishedPageHead(loaderData?.published.site)
    // A hosted public site owns its own head (including its content locale);
    // only the marketing landing announces the visitor-resolved locale.
    if (loaderData?.hostedPublic.isHostedPublic) {
      return base
    }
    const locale = coerceLocale(loaderData?.locale) ?? DEFAULT_LOCALE
    return {
      ...base,
      meta: [
        ...(base.meta ?? []),
        { property: 'og:locale', content: ogLocale(locale) },
      ],
    }
  },
  component: Home,
})

const HERO_DEMO_ENABLED =
  (import.meta as ImportMeta & { env: { VITE_HERO_DEMO_ENABLED?: string } }).env
    .VITE_HERO_DEMO_ENABLED === 'true'

const heroDemoSources = [
  {
    src: '/media/hero-demo/hero-768.webm',
    type: 'video/webm' as const,
    media: '(max-width: 640px)',
  },
  {
    src: '/media/hero-demo/hero-768.mp4',
    type: 'video/mp4' as const,
    media: '(max-width: 640px)',
  },
  { src: '/media/hero-demo/hero-1280.webm', type: 'video/webm' as const },
  { src: '/media/hero-demo/hero-1280.mp4', type: 'video/mp4' as const },
]

const landingTheme = {
  backgroundColor: '#131411',
  color: '#e4e2dd',
  '--background': '#131411',
  '--surface-0': '#131411',
  '--surface-1': '#1f201d',
  '--surface-2': '#2a2a27',
  '--surface-3': '#343532',
  '--paper': '#e4e2dd',
  '--paper-muted': '#cfc3ca',
  '--ink': '#131411',
  '--border': '#4c454a',
  '--thread-mauve': '#dabed6',
  '--thread-gold': '#f4a261',
  '--thread-teal': '#6fd8c8',
  '--thread-coral': '#f07a98',
  '--thread-violet': '#b58ad0',
} as CSSProperties

function Home() {
  const navigate = useNavigate()
  const locale = useLocale()
  const tr = translator(locale)
  const { hostedPublic, published } = Route.useLoaderData()
  const search = Route.useSearch()
  const [prompt, setPrompt] = useState('')
  const [restoreMessage, setRestoreMessage] = useState('')
  const [isStartingWorkspace, setIsStartingWorkspace] = useState(false)
  const [currentSession, setCurrentSession] = useState<BuilderSession | null>(null)

  const spunFor: Array<{ id: string; label: string; tone: string }> = [
    { id: 'shops', label: tr('landing.madeFor.shops'), tone: 'var(--thread-mauve)' },
    { id: 'studios', label: tr('landing.madeFor.studios'), tone: 'var(--thread-gold)' },
    { id: 'services', label: tr('landing.madeFor.services'), tone: 'var(--thread-teal)' },
    {
      id: 'contractors',
      label: tr('landing.madeFor.contractors'),
      tone: 'var(--thread-coral)',
    },
    {
      id: 'side-projects',
      label: tr('landing.madeFor.sideProjects'),
      tone: 'var(--thread-violet)',
    },
    {
      id: 'everything-else',
      label: tr('landing.madeFor.everythingElse'),
      tone: 'var(--thread-mauve)',
    },
  ]

  const landingPromptChips = [
    tr('landing.chips.warmLocal'),
    tr('landing.chips.booking'),
    tr('landing.chips.simple'),
    tr('landing.chips.premium'),
  ]

  useEffect(() => {
    if (hostedPublic.isHostedPublic) {
      return
    }

    let isMounted = true

    getCurrentSession({ retryOnUnauthorized: false })
      .then((session) => {
        if (isMounted) {
          setCurrentSession(session)
        }
      })
      .catch(() => {
        // No active session is the normal first-visit state.
      })

    return () => {
      isMounted = false
    }
  }, [hostedPublic.isHostedPublic])

  useEffect(() => {
    const incomingRestoreKey = extractRecoveryKey(search.restore || '')
    if (!incomingRestoreKey) {
      return
    }

    restoreWorkspace(incomingRestoreKey)
      .then(() => navigate({ to: '/app' }))
      .catch((error) => {
        setRestoreMessage(
          error instanceof APIError ? error.message : tr('landing.error.restore'),
        )
      })
  }, [navigate, search.restore, tr])

  async function handleGuestStart(promptOverride?: string) {
    setIsStartingWorkspace(true)
    setRestoreMessage('')
    try {
      await startAnonymousSession({ freshIfBlocked: true, locale })
      await navigate({
        to: '/app',
        search: promptOverride?.trim() ? { prompt: promptOverride.trim() } : {},
      })
    } catch (error) {
      setRestoreMessage(
        error instanceof APIError ? error.message : tr('landing.error.start'),
      )
      setIsStartingWorkspace(false)
    }
  }

  if (hostedPublic.isHostedPublic) {
    return (
      <PublishedSitePage
        site={published.site}
        errorMessage={published.errorMessage}
      />
    )
  }

  return (
    <main
      className={cn(
        'min-h-screen bg-[var(--surface-0)] text-[var(--paper)] antialiased',
        '[font-family:"Be_Vietnam_Pro","Avenir_Next","Segoe_UI",sans-serif]',
      )}
      style={landingTheme}
    >
      <section className="relative isolate flex min-h-screen flex-col overflow-hidden">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-[radial-gradient(70%_85%_at_50%_-8%,color-mix(in_oklch,var(--thread-mauve)_24%,var(--surface-0))_0%,transparent_72%)]"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute -right-[18%] bottom-[-22%] h-[68vh] w-[68vh] rounded-full bg-[radial-gradient(circle,color-mix(in_oklch,var(--thread-gold)_22%,transparent)_0%,transparent_70%)] blur-3xl"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute left-[-22%] top-[36%] h-[52vh] w-[52vh] rounded-full bg-[radial-gradient(circle,color-mix(in_oklch,var(--thread-teal)_14%,transparent)_0%,transparent_70%)] blur-3xl"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 bottom-0 h-40 bg-[linear-gradient(to_bottom,transparent,var(--surface-0))]"
        />

        <div className="relative z-10 mx-auto flex w-full max-w-[1180px] items-center justify-between gap-4 px-6 pt-7 md:px-8 md:pt-10">
          <Link
            to="/"
            className="inline-flex items-center gap-2.5 text-[15px] font-semibold tracking-tight text-[var(--paper)]"
          >
            <img src="/logo.png" alt="" className="size-7 object-contain" />
            snaelda
          </Link>
          {currentSession ? (
            <Link
              to="/app"
              className="inline-flex min-h-11 items-center gap-2 rounded-full border border-[color-mix(in_oklch,var(--thread-teal)_52%,var(--border))] bg-[color-mix(in_oklch,var(--surface-2)_76%,transparent)] px-4 text-sm font-bold text-[var(--paper)] transition-[background,border-color,color,transform] hover:-translate-y-px hover:border-[var(--thread-teal)] hover:bg-[var(--surface-2)]"
            >
              {tr('landing.nav.openWorkspace')}
              <ArrowRight aria-hidden className="size-4" />
            </Link>
          ) : (
            <Link
              to="/login"
              className="text-sm font-semibold text-[var(--paper-muted)] underline-offset-4 transition-colors hover:text-[var(--thread-gold)] hover:underline"
            >
              {tr('landing.nav.login')}
            </Link>
          )}
        </div>

        <div className="relative z-10 mx-auto flex w-full max-w-[1180px] flex-1 flex-col items-center justify-center px-6 pb-14 pt-10 text-center md:px-8 md:pb-20 md:pt-14">
          <img
            src="/logo.png"
            alt=""
            aria-hidden
            className="mb-7 h-[clamp(160px,24vw,260px)] w-[clamp(160px,24vw,260px)] object-contain drop-shadow-[0_22px_60px_color-mix(in_oklch,var(--thread-mauve)_22%,transparent)] animate-[spin_90s_linear_infinite] motion-reduce:animate-none"
          />

          <h1 className='max-w-4xl text-[clamp(3rem,8vw,5.6rem)] font-semibold leading-[0.95] tracking-[-0.025em] text-[color-mix(in_oklch,var(--thread-mauve)_70%,white)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
            {tr('landing.hero.titleLead')}{' '}
            <span className="italic font-normal text-[var(--paper-muted)]">
              {tr('landing.hero.titleAccent')}
            </span>
          </h1>
          <p className="mt-6 max-w-xl text-[1.05rem] leading-8 text-[var(--paper-muted)] md:text-[1.125rem]">
            {tr('landing.hero.subtitle')}
          </p>

          {restoreMessage ? (
            <p className="mt-6 max-w-2xl text-sm text-[var(--thread-gold)]" role="alert">
              {restoreMessage}
            </p>
          ) : null}

          {currentSession ? (
            <ReturningWorkspacePrompt session={currentSession} locale={locale} />
          ) : null}

          <form
            className={cn(
              'group relative flex w-full max-w-2xl flex-col items-stretch gap-3 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_56%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_88%,transparent)] p-3 shadow-[0_24px_70px_-22px_oklch(16%_0.05_336_/_0.6)] backdrop-blur-sm transition-colors duration-300 focus-within:border-[color-mix(in_oklch,var(--thread-teal)_70%,transparent)] md:flex-row md:items-center md:p-2.5 md:pl-3',
              currentSession ? 'mt-5' : 'mt-10',
            )}
            onSubmit={async (event: FormEvent) => {
              event.preventDefault()
              await handleGuestStart(prompt)
            }}
          >
            <Sparkles
              aria-hidden
              className="pointer-events-none absolute left-7 top-7 size-5 text-[color-mix(in_oklch,var(--thread-mauve)_70%,var(--paper))] md:top-1/2 md:-translate-y-1/2"
            />
            <input
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={tr('landing.form.placeholder')}
              aria-label={tr('landing.form.ariaLabel')}
              className="min-h-14 w-full rounded-[14px] border border-transparent bg-transparent py-3.5 pl-12 pr-4 text-base text-[var(--paper)] outline-none placeholder:text-[color-mix(in_oklch,var(--paper-muted)_60%,transparent)] md:text-lg"
            />
            <Button
              type="submit"
              size="lg"
              variant="plain"
              disabled={isStartingWorkspace}
              className="min-h-14 shrink-0 rounded-[14px] bg-[var(--thread-gold)] px-7 text-[var(--ink)] shadow-[0_10px_24px_-8px_oklch(78%_0.11_68_/_0.55)] transition-transform duration-200 hover:-translate-y-px hover:bg-[color-mix(in_oklch,var(--thread-gold)_84%,white)]"
            >
              <Globe className="size-4.5" />
              {isStartingWorkspace
                ? tr('landing.form.submitOpening')
                : currentSession
                  ? tr('landing.form.submitAnother')
                  : tr('landing.form.submitStart')}
            </Button>
          </form>

          <div className="mt-4 flex w-full max-w-2xl flex-wrap items-center justify-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-[0.14em] text-[color-mix(in_oklch,var(--paper-muted)_72%,transparent)]">
              {tr('landing.chips.label')}
            </span>
            {landingPromptChips.map((chip) => (
              <button
                key={chip}
                type="button"
                className="rounded-full border border-[color-mix(in_oklch,var(--border)_62%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_70%,transparent)] px-3 py-1.5 text-sm font-semibold text-[var(--paper-muted)] transition-[background,border-color,color,transform] hover:-translate-y-px hover:border-[color-mix(in_oklch,var(--thread-teal)_70%,transparent)] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]"
                onClick={() =>
                  setPrompt((current) =>
                    current.trim() ? `${current.trim()}. ${chip}.` : chip,
                  )
                }
              >
                {chip}
              </button>
            ))}
          </div>

          <p className="mt-4 max-w-xl text-sm text-[color-mix(in_oklch,var(--paper-muted)_76%,transparent)]">
            {tr('landing.form.helper')}
          </p>

          <p className="mt-5 text-xs text-[color-mix(in_oklch,var(--paper-muted)_70%,transparent)]">
            {tr('landing.consent.prefix')}{' '}
            <Link
              to="/terms"
              className="font-semibold text-[var(--paper-muted)] underline underline-offset-4 hover:text-[var(--paper)]"
            >
              {tr('landing.consent.terms')}
            </Link>{' '}
            {tr('landing.consent.and')}{' '}
            <Link
              to="/privacy"
              className="font-semibold text-[var(--paper-muted)] underline underline-offset-4 hover:text-[var(--paper)]"
            >
              {tr('landing.consent.privacy')}
            </Link>
            .
          </p>

          {HERO_DEMO_ENABLED ? (
            <div className="mt-8 md:mt-10">
              <HeroScrollCue targetId="hero-demo" />
            </div>
          ) : null}
        </div>
      </section>

      {HERO_DEMO_ENABLED ? (
        <section className="relative z-10 mx-auto w-full max-w-[1180px] px-6 pt-20 pb-8 md:px-8 md:pt-28 md:pb-12">
          <HeroDemo
            id="hero-demo"
            posterSrc="/media/hero-demo/poster.webp"
            sources={heroDemoSources}
            eyebrow={tr('landing.demo.eyebrow')}
            caption={tr('landing.demo.caption')}
          />
        </section>
      ) : null}

      <div className="relative">
        <section className="relative z-10 mx-auto w-full max-w-[1100px] px-6 pt-24 pb-24 md:px-8 md:pt-28 md:pb-32">
          <div className="grid gap-x-12 gap-y-6 md:grid-cols-[minmax(0,1fr)_minmax(0,2.2fr)] md:items-start">
            <div className="text-sm font-semibold uppercase tracking-[0.18em] text-[color-mix(in_oklch,var(--thread-mauve)_72%,var(--paper))] md:pt-3">
              {tr('landing.spins.eyebrow')}
            </div>
            <p className='text-balance text-[clamp(1.45rem,2.6vw,2rem)] font-normal leading-[1.35] text-[var(--paper)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
              {tr('landing.spins.lead')}{' '}
              <span className="text-[var(--paper-muted)]">
                {tr('landing.spins.mid')}
              </span>{' '}
              <span className="text-[color-mix(in_oklch,var(--paper-muted)_70%,transparent)]">
                {tr('landing.spins.tail')}
              </span>
            </p>
          </div>
        </section>

        <section className="relative z-10 mx-auto w-full max-w-[1100px] border-t border-[color-mix(in_oklch,var(--border)_36%,transparent)] px-6 py-20 md:px-8 md:py-28">
          <p className='text-balance text-[clamp(1.4rem,2.5vw,1.95rem)] font-normal leading-[1.45] text-[var(--paper-muted)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
            {tr('landing.madeFor.prefix')}{' '}
            {spunFor.map((item, index) => (
              <span key={item.id}>
                <span style={{ color: item.tone }} className="font-medium">
                  {item.label}
                </span>
                {index < spunFor.length - 2
                  ? tr('landing.madeFor.sep')
                  : index === spunFor.length - 2
                    ? tr('landing.madeFor.lastSep')
                    : tr('landing.madeFor.end')}
              </span>
            ))}
            <span className="text-[color-mix(in_oklch,var(--paper-muted)_72%,transparent)]">
              {tr('landing.madeFor.tail')}
            </span>
          </p>
        </section>
      </div>

      <footer className="border-t border-[color-mix(in_oklch,var(--border)_36%,transparent)] px-6 py-10 md:px-8 md:py-12">
        <div className="mx-auto flex w-full max-w-[1100px] flex-col items-start justify-between gap-6 md:flex-row md:items-center">
          <Link
            to="/"
            className="inline-flex items-center gap-2.5 text-[15px] font-semibold tracking-tight text-[var(--paper)]"
          >
            <img src="/logo.png" alt="" className="size-7 object-contain" />
            snaelda
          </Link>
          <nav className="flex flex-wrap items-center gap-6 text-sm font-semibold text-[var(--paper-muted)]">
            <Link to="/terms" className="transition-colors hover:text-[var(--paper)]">
              {tr('landing.footer.terms')}
            </Link>
            <Link to="/privacy" className="transition-colors hover:text-[var(--paper)]">
              {tr('landing.footer.privacy')}
            </Link>
            {currentSession ? (
              <Link to="/app" className="transition-colors hover:text-[var(--thread-teal)]">
                {tr('landing.nav.openWorkspace')}
              </Link>
            ) : (
              <Link to="/login" className="transition-colors hover:text-[var(--thread-gold)]">
                {tr('landing.nav.login')}
              </Link>
            )}
          </nav>
        </div>
      </footer>
    </main>
  )
}

export function ReturningWorkspacePrompt({
  session,
  locale = DEFAULT_LOCALE,
}: {
  session: BuilderSession
  locale?: Locale
}) {
  const tr = translator(locale)
  const ownerLabel =
    session.user?.name ||
    session.user?.email ||
    (session.kind === 'trial'
      ? tr('landing.returning.fallbackTrial')
      : tr('landing.returning.fallbackWorkspace'))

  return (
    <div className="mt-8 flex w-full max-w-2xl flex-col items-center justify-between gap-4 rounded-[16px] border border-[color-mix(in_oklch,var(--thread-teal)_42%,var(--border))] bg-[color-mix(in_oklch,var(--thread-teal)_8%,var(--surface-1))] px-5 py-4 text-left sm:flex-row">
      <div className="min-w-0">
        <p className="text-xs font-bold uppercase tracking-[0.12em] text-[var(--thread-teal)]">
          {tr('landing.returning.eyebrow')}
        </p>
        <p className="mt-1 truncate text-sm text-[var(--paper-muted)]">
          {ownerLabel}
        </p>
      </div>
      <Link
        to="/app"
        className="inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-full bg-[var(--thread-teal)] px-5 text-sm font-bold text-[var(--ink)] transition-[background,transform] hover:-translate-y-px hover:bg-[color-mix(in_oklch,var(--thread-teal)_86%,white)]"
      >
        {tr('landing.returning.cta')}
        <ArrowRight aria-hidden className="size-4" />
      </Link>
    </div>
  )
}

function extractRecoveryKey(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return ''
  }
  try {
    const parsed = new URL(trimmed)
    return parsed.searchParams.get('restore') || parsed.searchParams.get('k') || trimmed
  } catch {
    return trimmed
  }
}
