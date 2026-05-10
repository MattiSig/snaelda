import { Link, createFileRoute } from '@tanstack/react-router'
import { PublishedSitePage } from '@/components/PublishedSitePage'
import { getHostedPublicSiteContext } from '@/lib/public-site'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'
import { actions, form, layout, panel, text } from '@/lib/styles'

export const Route = createFileRoute('/')({
  loader: async () => ({
    hostedPublic: await getHostedPublicSiteContext(),
  }),
  component: Home,
})

function Home() {
  const { hostedPublic } = Route.useLoaderData()

  if (hostedPublic.isHostedPublic) {
    return (
      <PublishedSitePage
        hostname={hostedPublic.hostname}
        pagePath={hostedPublic.pagePath}
      />
    )
  }

  return (
    <main className={cn(layout.pageShell, 'pb-12 pt-10')}>
      <section className="grid min-h-[calc(100vh-150px)] items-center gap-8 lg:grid-cols-[minmax(0,1.02fr)_minmax(360px,0.98fr)]">
        <div className="grid gap-7">
          <div className="flex items-center gap-4">
            <img
              src="/logo.png"
              alt=""
              className="size-20 rounded-[22px] border border-border bg-[var(--surface-2)] object-contain p-2 shadow-[var(--shadow-tight)]"
            />
            <div>
              <p className={text.eyebrow}>Small-site workshop</p>
              <p className="mt-1 text-sm font-bold text-[var(--thread-coral)]">
                Prompt, tune, publish.
              </p>
            </div>
          </div>

          <div className="grid gap-5">
            <h1 className={cn(text.h1, 'max-w-[11ch]')}>
              Spin a real website from a rough idea.
            </h1>
            <p className={cn(text.p, 'text-lg leading-8')}>
              Snaelda turns a short business brief into a structured draft you
              can edit, preview, and publish without opening a heavyweight site
              builder.
            </p>
          </div>

          <div className={actions.rowLarge}>
            <Button asChild size="lg">
              <Link to="/login" search={{ redirect: '/app' }}>
                Start a draft
              </Link>
            </Button>
            <Button asChild variant="outline" size="lg">
              <Link to="/app">Open builder</Link>
            </Button>
          </div>
        </div>

        <div className={cn(panel, 'relative grid gap-5 p-5 max-sm:p-4')}>
          <div className="absolute -top-3 right-7 h-6 w-px bg-[var(--thread-teal)]" />
          <form className={cn(form.grid, 'rounded-[16px] border border-border bg-[var(--surface-2)] p-4')}>
            <label htmlFor="site-prompt" className={text.label}>Website brief</label>
            <Textarea
              id="site-prompt"
              name="prompt"
              rows={7}
              placeholder="A warm one-page site for a florist that hosts weekend bouquet classes."
            />
            <Button type="button" disabled>
              Generate draft
            </Button>
          </form>

          <div className="grid gap-3 rounded-[16px] border border-border bg-[var(--paper)] p-4 text-[var(--ink)]">
            <div className="flex items-center justify-between gap-3 border-b border-[oklch(78%_0.05_68)] pb-3">
              <div>
                <p className="text-xs font-black uppercase tracking-[0.12em] text-[oklch(42%_0.045_336)]">
                  Draft preview
                </p>
                <strong className="text-lg">Moss & Thread Studio</strong>
              </div>
              <span className="rounded-full bg-[oklch(86%_0.08_184)] px-3 py-1 text-xs font-black text-[var(--ink)]">
                Ready
              </span>
            </div>
            <div className="grid gap-2">
              <div className="h-16 rounded-[12px] bg-[oklch(28%_0.04_336)]" />
              <div className="grid grid-cols-3 gap-2">
                <span className="h-12 rounded-[10px] bg-[oklch(82%_0.095_184)]" />
                <span className="h-12 rounded-[10px] bg-[oklch(80%_0.12_16)]" />
                <span className="h-12 rounded-[10px] bg-[oklch(80%_0.105_78)]" />
              </div>
            </div>
            <div className="flex flex-wrap gap-2 pt-1 text-xs font-bold text-[oklch(42%_0.045_336)]">
              <span>Home</span>
              <span>Workshops</span>
              <span>Contact</span>
            </div>
          </div>
        </div>
      </section>
    </main>
  )
}
