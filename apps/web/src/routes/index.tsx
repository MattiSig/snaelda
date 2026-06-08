import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { CSSProperties, FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { ArrowRight, Globe, Sparkles } from 'lucide-react'
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
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/')({
  validateSearch: (search: Record<string, unknown>) => {
    const restore =
      typeof search.restore === 'string' && search.restore.length > 0
        ? search.restore
        : undefined
    return restore ? { restore } : {}
  },
  loader: async () => {
    const hostedPublic = await getHostedPublicSiteContext()
    return {
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
  head: ({ loaderData }) => buildPublishedPageHead(loaderData?.published.site),
  component: Home,
})

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

const spunFor: Array<{ label: string; tone: string }> = [
  { label: 'shops', tone: 'var(--thread-mauve)' },
  { label: 'studios', tone: 'var(--thread-gold)' },
  { label: 'services', tone: 'var(--thread-teal)' },
  { label: 'contractors', tone: 'var(--thread-coral)' },
  { label: 'side projects', tone: 'var(--thread-violet)' },
  { label: 'everything in between', tone: 'var(--thread-mauve)' },
]

const landingPromptChips = [
  'Make it warm and local',
  'Include booking',
  'Keep it simple',
  'Make it feel premium',
]

function Home() {
  const navigate = useNavigate()
  const { hostedPublic, published } = Route.useLoaderData()
  const search = Route.useSearch()
  const [prompt, setPrompt] = useState('')
  const [restoreMessage, setRestoreMessage] = useState('')
  const [isStartingWorkspace, setIsStartingWorkspace] = useState(false)
  const [currentSession, setCurrentSession] = useState<BuilderSession | null>(null)

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
          error instanceof APIError ? error.message : 'Could not restore that workspace',
        )
      })
  }, [navigate, search.restore])

  async function handleGuestStart(promptOverride?: string) {
    setIsStartingWorkspace(true)
    setRestoreMessage('')
    try {
      await startAnonymousSession({ freshIfBlocked: true })
      await navigate({
        to: '/app',
        search: promptOverride?.trim() ? { prompt: promptOverride.trim() } : {},
      })
    } catch (error) {
      setRestoreMessage(
        error instanceof APIError ? error.message : 'Could not start a workspace',
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
              Open workspace
              <ArrowRight aria-hidden className="size-4" />
            </Link>
          ) : (
            <Link
              to="/login"
              className="text-sm font-semibold text-[var(--paper-muted)] underline-offset-4 transition-colors hover:text-[var(--thread-gold)] hover:underline"
            >
              Log in
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
            Spin up a website.{' '}
            <span className="italic font-normal text-[var(--paper-muted)]">
              A real one.
            </span>
          </h1>
          <p className="mt-6 max-w-xl text-[1.05rem] leading-8 text-[var(--paper-muted)] md:text-[1.125rem]">
            Describe what you do. Snaelda lays down a real first draft, ready
            to refine, tweak, and publish.
          </p>

          {restoreMessage ? (
            <p className="mt-6 max-w-2xl text-sm text-[var(--thread-gold)]" role="alert">
              {restoreMessage}
            </p>
          ) : null}

          {currentSession ? (
            <ReturningWorkspacePrompt session={currentSession} />
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
              placeholder="A cozy pottery studio in Portland..."
              aria-label="Describe your business"
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
                ? 'Opening workspace...'
                : currentSession
                  ? 'Start another site'
                  : 'Spin my site'}
            </Button>
          </form>

          <div className="mt-4 flex w-full max-w-2xl flex-wrap items-center justify-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-[0.14em] text-[color-mix(in_oklch,var(--paper-muted)_72%,transparent)]">
              Add direction
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
            The first draft is not the final word. Once it lands, keep shaping
            the site from the preview or the AI refine panel.
          </p>

          <p className="mt-5 text-xs text-[color-mix(in_oklch,var(--paper-muted)_70%,transparent)]">
            By continuing, you agree to our{' '}
            <Link
              to="/terms"
              className="font-semibold text-[var(--paper-muted)] underline underline-offset-4 hover:text-[var(--paper)]"
            >
              Terms
            </Link>{' '}
            and{' '}
            <Link
              to="/privacy"
              className="font-semibold text-[var(--paper-muted)] underline underline-offset-4 hover:text-[var(--paper)]"
            >
              Privacy Policy
            </Link>
            .
          </p>
        </div>
      </section>

      <div className="relative">
        <section className="relative z-10 mx-auto w-full max-w-[1100px] px-6 pt-24 pb-24 md:px-8 md:pt-28 md:pb-32">
          <div className="grid gap-x-12 gap-y-6 md:grid-cols-[minmax(0,1fr)_minmax(0,2.2fr)] md:items-start">
            <div className="text-sm font-semibold uppercase tracking-[0.18em] text-[color-mix(in_oklch,var(--thread-mauve)_72%,var(--paper))] md:pt-3">
              How it spins
            </div>
            <p className='text-balance text-[clamp(1.45rem,2.6vw,2rem)] font-normal leading-[1.35] text-[var(--paper)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
              Prompt your idea. Snaelda lays down a real first draft: pages,
              structure, copy, the works.{' '}
              <span className="text-[var(--paper-muted)]">
                Tweak whatever feels off in a lightweight editor.
              </span>{' '}
              <span className="text-[color-mix(in_oklch,var(--paper-muted)_70%,transparent)]">
                Publish when it feels right. Point your domain when it feels like home.
              </span>
            </p>
          </div>
        </section>

        <section className="relative z-10 mx-auto w-full max-w-[1100px] border-t border-[color-mix(in_oklch,var(--border)_36%,transparent)] px-6 py-20 md:px-8 md:py-28">
          <p className='text-balance text-[clamp(1.4rem,2.5vw,1.95rem)] font-normal leading-[1.45] text-[var(--paper-muted)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
            Made for{' '}
            {spunFor.map((item, index) => (
              <span key={item.label}>
                <span style={{ color: item.tone }} className="font-medium">
                  {item.label}
                </span>
                {index < spunFor.length - 2
                  ? ', '
                  : index === spunFor.length - 2
                    ? ', and '
                    : '. '}
              </span>
            ))}
            <span className="text-[color-mix(in_oklch,var(--paper-muted)_72%,transparent)]">
              Small operations that need a website, not a website project.
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
              Terms
            </Link>
            <Link to="/privacy" className="transition-colors hover:text-[var(--paper)]">
              Privacy
            </Link>
            {currentSession ? (
              <Link to="/app" className="transition-colors hover:text-[var(--thread-teal)]">
                Open workspace
              </Link>
            ) : (
              <Link to="/login" className="transition-colors hover:text-[var(--thread-gold)]">
                Log in
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
}: {
  session: BuilderSession
}) {
  const ownerLabel =
    session.user?.name ||
    session.user?.email ||
    (session.kind === 'trial' ? 'Your trial workspace' : 'Your workspace')

  return (
    <div className="mt-8 flex w-full max-w-2xl flex-col items-center justify-between gap-4 rounded-[16px] border border-[color-mix(in_oklch,var(--thread-teal)_42%,var(--border))] bg-[color-mix(in_oklch,var(--thread-teal)_8%,var(--surface-1))] px-5 py-4 text-left sm:flex-row">
      <div className="min-w-0">
        <p className="text-xs font-bold uppercase tracking-[0.12em] text-[var(--thread-teal)]">
          Your workspace is waiting
        </p>
        <p className="mt-1 truncate text-sm text-[var(--paper-muted)]">
          {ownerLabel}
        </p>
      </div>
      <Link
        to="/app"
        className="inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-full bg-[var(--thread-teal)] px-5 text-sm font-bold text-[var(--ink)] transition-[background,transform] hover:-translate-y-px hover:bg-[color-mix(in_oklch,var(--thread-teal)_86%,white)]"
      >
        Continue editing
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
