import { Link } from '@tanstack/react-router'
import type { CSSProperties, ReactNode } from 'react'
import { cn } from '@/lib/utils'

const noiseTexture =
  'url("data:image/svg+xml,%3Csvg viewBox=\'0 0 200 200\' xmlns=\'http://www.w3.org/2000/svg\'%3E%3Cfilter id=\'noiseFilter\'%3E%3CfeTurbulence type=\'fractalNoise\' baseFrequency=\'0.65\' numOctaves=\'3\' stitchTiles=\'stitch\'/%3E%3C/filter%3E%3Crect width=\'100%25\' height=\'100%25\' filter=\'url(%23noiseFilter)\' opacity=\'0.05\'/%3E%3C/svg%3E")'

const legalTheme = {
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

export type LegalPageProps = {
  title: string
  lastUpdated: string
  intro?: ReactNode
  children: ReactNode
}

export function LegalPage({ title, lastUpdated, intro, children }: LegalPageProps) {
  return (
    <main
      className={cn(
        'min-h-screen bg-[var(--surface-0)] text-[var(--paper)] antialiased',
        '[font-family:"Be_Vietnam_Pro","Avenir_Next","Segoe_UI",sans-serif]',
      )}
      style={legalTheme}
    >
      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-x-0 top-0 h-[420px] bg-[radial-gradient(circle_at_50%_0%,color-mix(in_oklch,var(--thread-violet)_22%,var(--surface-0))_0%,transparent_70%)] opacity-70" />

        <header className="relative z-10 mx-auto flex w-full max-w-[920px] items-center justify-between gap-4 px-6 pt-10 md:px-8 md:pt-14">
          <Link
            to="/"
            className="inline-flex items-center gap-3 rounded-full border border-[color-mix(in_oklch,var(--border)_78%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_82%,transparent)] px-4 py-2 text-sm font-semibold text-[var(--paper-muted)] backdrop-blur-sm transition-colors hover:text-[var(--paper)]"
          >
            <img src="/logo.png" alt="" className="size-7 rounded-full object-contain" />
            Snaelda
          </Link>
          <nav className="flex flex-wrap items-center gap-5 text-sm font-semibold text-[var(--paper-muted)]">
            <Link to="/terms" className="hover:text-[var(--paper)]">
              Terms
            </Link>
            <Link to="/privacy" className="hover:text-[var(--paper)]">
              Privacy
            </Link>
            <Link to="/login" className="hover:text-[var(--thread-gold)]">
              Log in
            </Link>
          </nav>
        </header>

        <section className="relative z-10 mx-auto w-full max-w-[920px] px-6 pb-24 pt-12 md:px-8">
          <p className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--thread-gold)]">
            Legal
          </p>
          <h1 className='mt-3 text-[clamp(2.4rem,5vw,3.6rem)] font-bold leading-[0.98] tracking-[-0.02em] text-[color-mix(in_oklch,var(--thread-violet)_72%,white)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
            {title}
          </h1>
          <p className="mt-3 text-sm text-[color-mix(in_oklch,var(--paper-muted)_70%,transparent)]">
            Last updated: {lastUpdated}
          </p>

          {intro ? (
            <div className="mt-8 rounded-[16px] border border-[color-mix(in_oklch,var(--border)_60%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_88%,transparent)] p-6 text-[15px] leading-7 text-[var(--paper-muted)]">
              {intro}
            </div>
          ) : null}

          <article className="legal-prose mt-10 text-[15.5px] leading-[1.75] text-[var(--paper-muted)]">
            {children}
          </article>
        </section>

        <footer className="border-t border-[color-mix(in_oklch,var(--border)_56%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_88%,black)] px-6 py-10 md:px-8">
          <div className="mx-auto flex w-full max-w-[920px] flex-col items-center justify-between gap-4 text-center text-sm text-[var(--paper-muted)] md:flex-row md:text-left">
            <div className='text-[1.2rem] font-semibold tracking-[-0.01em] text-[color-mix(in_oklch,var(--thread-violet)_72%,white)] [font-family:"Literata","Iowan_Old_Style","Palatino_Linotype",serif]'>
              Snaelda
            </div>
            <nav className="flex flex-wrap items-center justify-center gap-5">
              <Link to="/terms" className="hover:text-[var(--paper)]">
                Terms
              </Link>
              <Link to="/privacy" className="hover:text-[var(--paper)]">
                Privacy
              </Link>
              <Link to="/" className="hover:text-[var(--paper)]">
                Home
              </Link>
            </nav>
          </div>
        </footer>
      </div>

      <style>{`
        .legal-prose h2 {
          margin-top: 2.6rem;
          margin-bottom: 0.9rem;
          font-family: "Literata", "Iowan Old Style", "Palatino Linotype", serif;
          font-size: 1.55rem;
          font-weight: 600;
          line-height: 1.2;
          color: color-mix(in oklch, var(--thread-violet) 72%, white);
          letter-spacing: -0.01em;
        }
        .legal-prose h3 {
          margin-top: 1.8rem;
          margin-bottom: 0.5rem;
          font-size: 1.05rem;
          font-weight: 700;
          color: var(--paper);
        }
        .legal-prose p { margin: 0 0 1rem 0; }
        .legal-prose ul, .legal-prose ol {
          margin: 0 0 1.1rem 1.25rem;
          padding: 0;
        }
        .legal-prose li { margin-bottom: 0.35rem; }
        .legal-prose a {
          color: var(--thread-gold);
          text-decoration: underline;
          text-underline-offset: 3px;
        }
        .legal-prose a:hover { color: color-mix(in oklch, var(--thread-gold) 80%, white); }
        .legal-prose strong { color: var(--paper); font-weight: 600; }
        .legal-prose hr {
          border: none;
          border-top: 1px solid color-mix(in oklch, var(--border) 60%, transparent);
          margin: 2.4rem 0;
        }
      `}</style>
    </main>
  )
}
