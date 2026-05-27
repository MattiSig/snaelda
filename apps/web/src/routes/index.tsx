import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { CSSProperties, FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { Globe, Hourglass, PencilLine, Rocket, SlidersHorizontal, Sparkles } from 'lucide-react'
import { PublishedSitePage } from '@/components/PublishedSitePage'
import { APIError, getPublishedSiteByHostname, restoreWorkspace, startAnonymousSession } from '@/lib/api'
import {
  buildPublishedPageHead,
  loadPublishedSitePageData,
} from '@/lib/published-site'
import { getHostedPublicSiteContext } from '@/lib/public-site'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/')({
  validateSearch: (search: Record<string, unknown>) => ({
    restore: typeof search.restore === 'string' ? search.restore : '',
  }),
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

const noiseTexture =
  'url("data:image/svg+xml,%3Csvg viewBox=\'0 0 200 200\' xmlns=\'http://www.w3.org/2000/svg\'%3E%3Cfilter id=\'noiseFilter\'%3E%3CfeTurbulence type=\'fractalNoise\' baseFrequency=\'0.65\' numOctaves=\'3\' stitchTiles=\'stitch\'/%3E%3C/filter%3E%3Crect width=\'100%25\' height=\'100%25\' filter=\'url(%23noiseFilter)\' opacity=\'0.05\'/%3E%3C/svg%3E")'

const loomImage =
  'https://lh3.googleusercontent.com/aida-public/AB6AXuBQFen3SNnVewpIqowsHKFuQBPu7WB_x3FQP27Iggi1hrU0rQTzzCrs6Z6Vn9hVsnGS-_dNIdLMIEq2F43NZeMMd6WFzq9y5FZTNqT-VP9Cd67HvcFzNUWCxic6bymqP29p-td4DqMalWsuVMhVIy5SJVof1BvbjH7v3SwuXDQxIMICFuyQQGnvh-GrjH9xNAVzVzJ5fU84kUXv5-VBhCv8974nkmwb2tKsLcEhVGqQk6KFOCMQDKMmapT85J3h1Hz0rHhzEOqNbACF'

const landingTheme = {
  backgroundImage: noiseTexture,
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
  '--thread-violet': '#dabed6',
  '--thread-gold': '#ffb780',
  '--thread-teal': '#6fd8c8',
} as CSSProperties

const featureCards = [
  {
    title: 'Prompt your vision',
    body: 'Describe your dream space in plain English. The loom turns your words into a strong first structure.',
    icon: PencilLine,
    bar: 'bg-[var(--thread-teal)]',
    iconShell:
      'bg-[color-mix(in_oklch,var(--thread-teal)_24%,var(--surface-0))] text-[color-mix(in_oklch,var(--thread-teal)_72%,white)]',
  },
  {
    title: 'Refine with ease',
    body: 'Adjust layout, color, and copy as easily as swapping a fresh spool onto the spindle.',
    icon: SlidersHorizontal,
    bar: 'bg-[var(--thread-gold)]',
    iconShell:
      'bg-[color-mix(in_oklch,var(--thread-gold)_28%,var(--surface-0))] text-[color-mix(in_oklch,var(--thread-gold)_72%,white)]',
  },
  {
    title: 'Publish in a heartbeat',
    body: 'When the tapestry feels right, send it live with hosting, domains, and crawlable pages ready to go.',
    icon: Rocket,
    bar: 'bg-[var(--thread-violet)]',
    iconShell:
      'bg-[color-mix(in_oklch,var(--thread-violet)_24%,var(--surface-0))] text-[color-mix(in_oklch,var(--thread-violet)_72%,white)]',
  },
]

function Home() {
  const navigate = useNavigate()
  const { hostedPublic, published } = Route.useLoaderData()
  const search = Route.useSearch()
  const [prompt, setPrompt] = useState('')
  const [restoreMessage, setRestoreMessage] = useState('')
  const [isStartingWorkspace, setIsStartingWorkspace] = useState(false)

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
      await startAnonymousSession()
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
      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-x-0 top-0 h-[500px] bg-[radial-gradient(circle_at_50%_0%,color-mix(in_oklch,var(--thread-violet)_26%,var(--surface-0))_0%,transparent_70%)] opacity-70" />

        <section className="relative z-10 mx-auto flex w-full max-w-[1280px] flex-col items-center px-6 py-16 text-center md:px-8 md:py-24">
          <div className="mb-10 flex w-full max-w-6xl items-center justify-between gap-4 text-left">
            <div className="inline-flex items-center gap-3 rounded-full border border-[color-mix(in_oklch,var(--border)_78%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_82%,transparent)] px-4 py-2 text-sm font-semibold text-[var(--paper-muted)] backdrop-blur-sm">
              <img src="/logo.png" alt="" className="size-7 rounded-full object-contain" />
              Small-site workshop
            </div>
            <Link
              to="/login"
              className="text-sm font-semibold text-[var(--paper-muted)] underline-offset-4 transition-colors hover:text-[var(--thread-gold)] hover:underline"
            >
              Log in
            </Link>
          </div>

          <h1 className='max-w-4xl text-[clamp(3.2rem,8vw,4.9rem)] font-bold leading-[0.92] tracking-[-0.03em] text-[color-mix(in_oklch,var(--thread-violet)_72%,white)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
            Weave Your Digital Home
          </h1>
          <p className="mt-5 max-w-2xl text-[1.05rem] leading-8 text-[var(--paper-muted)] md:text-[1.125rem]">
            Snaelda turns your ideas into beautifully crafted, hand-woven websites.
            No sterile templates, just your story brought to life.
          </p>

          {restoreMessage ? (
            <p className="mt-6 max-w-2xl text-sm text-[var(--thread-gold)]" role="alert">
              {restoreMessage}
            </p>
          ) : null}

          <form
            className="group relative mt-10 flex w-full max-w-2xl flex-col items-stretch gap-4 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_60%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_92%,transparent)] p-4 shadow-[0_20px_60px_-15px_oklch(16%_0.05_336_/_0.55)] transition-colors duration-300 focus-within:border-[var(--thread-teal)] md:flex-row md:items-center"
            onSubmit={async (event: FormEvent) => {
              event.preventDefault()
              await handleGuestStart(prompt)
            }}
          >
            <Sparkles className="pointer-events-none absolute left-6 top-6 size-5 text-[color-mix(in_oklch,var(--thread-violet)_65%,var(--paper))] md:top-1/2 md:-translate-y-1/2" />
            <input
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="A cozy pottery studio in Portland..."
              className="min-h-14 w-full rounded-[14px] border border-transparent bg-[color-mix(in_oklch,var(--surface-1)_64%,var(--background))] py-4 pl-12 pr-4 text-base text-[var(--paper)] shadow-[inset_0_1px_0_oklch(100%_0_0_/_0.04)] outline-none placeholder:text-[color-mix(in_oklch,var(--paper-muted)_62%,transparent)] md:text-lg"
            />
            <Button
              type="submit"
              size="lg"
              variant="plain"
              disabled={isStartingWorkspace}
              className="min-h-14 shrink-0 rounded-[14px] bg-[var(--thread-gold)] px-7 text-[var(--ink)] shadow-[0_4px_14px_0_oklch(78%_0.11_78_/_0.35)] transition-transform duration-200 hover:-translate-y-px hover:bg-[color-mix(in_oklch,var(--thread-gold)_84%,white)]"
            >
              <Globe className="size-4.5" />
              {isStartingWorkspace ? 'Opening workspace...' : 'Weave My Site'}
            </Button>
          </form>

          <p className="mt-4 text-xs text-[color-mix(in_oklch,var(--paper-muted)_70%,transparent)]">
            By continuing, you agree to our{' '}
            <Link
              to="/terms"
              className="font-semibold text-[var(--paper-muted)] underline underline-offset-4 hover:text-[var(--paper)]"
            >
              Terms of Use
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

          <div className="relative mt-16 w-full max-w-5xl overflow-hidden rounded-[18px] border border-[color-mix(in_oklch,var(--border)_56%,transparent)] shadow-[0_30px_80px_-20px_oklch(16%_0.05_336_/_0.6)]">
            <img
              src={loomImage}
              alt="A glowing digital loom weaving a website in a warm dark workshop."
              className="h-[280px] w-full object-cover opacity-80 mix-blend-luminosity transition-all duration-700 hover:mix-blend-normal md:h-[400px]"
            />
            <div className="absolute inset-0 bg-[linear-gradient(180deg,transparent_0%,transparent_58%,color-mix(in_oklch,var(--background)_92%,black)_100%)]" />
            <div className="absolute inset-x-0 bottom-10 flex flex-col items-center gap-2">
              <Hourglass className="size-9 text-[var(--thread-gold)] animate-pulse" />
              <div className="h-12 w-px bg-[var(--thread-gold)] opacity-55" />
            </div>
          </div>
        </section>

        <section id="process" className="mx-auto w-full max-w-[1280px] px-6 pb-16 md:px-8 md:pb-20">
          <div className="grid gap-6 md:grid-cols-3">
            {featureCards.map((card) => {
              const Icon = card.icon

              return (
                <article
                  key={card.title}
                  className="group relative flex flex-col gap-4 overflow-hidden rounded-[18px] border border-[color-mix(in_oklch,var(--border)_56%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_84%,black)] p-8 shadow-[0_10px_30px_-10px_oklch(16%_0.05_336_/_0.36)]"
                >
                  <div className={cn('absolute left-0 top-0 h-1 w-full origin-left scale-x-0 transition-transform duration-500 group-hover:scale-x-100', card.bar)} />
                  <div className={cn('mb-2 flex size-12 items-center justify-center rounded-full', card.iconShell)}>
                    <Icon className="size-5" />
                  </div>
                  <h2 className='text-[1.55rem] font-semibold text-[color-mix(in_oklch,var(--thread-violet)_72%,white)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
                    {card.title}
                  </h2>
                  <p className="text-base leading-7 text-[var(--paper-muted)]">{card.body}</p>
                </article>
              )
            })}
          </div>
        </section>
      </div>

      <footer className="border-t border-[color-mix(in_oklch,var(--border)_56%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_88%,black)] px-6 py-12 md:px-8">
        <div className="mx-auto flex w-full max-w-[1280px] flex-col items-center justify-between gap-6 text-center md:flex-row md:text-left">
          <div className='text-[1.7rem] font-semibold tracking-[-0.02em] text-[color-mix(in_oklch,var(--thread-violet)_72%,white)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
            Snaelda
          </div>
          <nav className="flex flex-wrap items-center justify-center gap-6 text-sm font-semibold text-[var(--paper-muted)]">
            <a href="#process" className="transition-colors hover:text-[var(--paper)]">
              Manifesto
            </a>
            <Link to="/app" className="transition-colors hover:text-[var(--paper)]">
              Showcase
            </Link>
            <Link to="/login" className="transition-colors hover:text-[var(--paper)]">
              Support
            </Link>
            <Link to="/terms" className="transition-colors hover:text-[var(--paper)]">
              Terms
            </Link>
            <Link to="/privacy" className="transition-colors hover:text-[var(--paper)]">
              Privacy
            </Link>
          </nav>
          <Button
            asChild
            variant="outline"
            className="min-h-11 rounded-[14px] border-[1.5px] border-[var(--border)] bg-transparent px-6 text-[var(--paper)] hover:border-[var(--thread-gold)] hover:bg-transparent hover:text-[var(--thread-gold)]"
          >
            <Link to="/login">
              Log In
            </Link>
          </Button>
        </div>
      </footer>
    </main>
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
