import { cn } from '@/lib/utils'

export const text = {
  eyebrow:
    'text-xs font-bold uppercase tracking-[0.12em] text-[var(--paper-muted)]',
  label: 'text-xs font-bold uppercase tracking-[0.1em] text-[var(--paper-muted)]',
  h1: 'max-w-[12ch] text-[clamp(3rem,7vw,6.4rem)] font-black leading-[0.9] text-[var(--paper)] text-balance',
  h2: 'text-[clamp(1.75rem,3.4vw,3rem)] font-black leading-[0.96] text-[var(--paper)] text-balance',
  h3: 'text-[1.2rem] font-extrabold leading-[1.05] text-[var(--paper)] text-balance',
  p: 'max-w-[68ch] text-[var(--paper-muted)]',
  muted: 'text-[color-mix(in_oklch,var(--paper-muted)_72%,var(--background))]',
  error: 'm-0 font-bold text-destructive',
  success: 'm-0 font-bold text-[var(--success)]',
}

export const ribbon =
  'relative overflow-hidden before:pointer-events-none before:absolute before:inset-x-4 before:top-0 before:h-px before:bg-[color-mix(in_oklch,var(--thread-coral)_72%,var(--thread-gold))] before:opacity-70 after:pointer-events-none after:absolute after:right-6 after:top-0 after:h-8 after:w-px after:bg-[var(--thread-teal)] after:opacity-55'

export const panel = cn(
  'rounded-[18px] border border-border',
  'bg-[var(--surface-1)]',
  'shadow-[var(--shadow-soft)]',
)

export const paddedPanel = cn(panel, 'p-6 max-sm:p-4')
export const ribbonPanel = cn(paddedPanel, ribbon)

export const layout = {
  pageShell: 'mx-auto w-full max-w-[1440px] px-5 pb-8 pt-8 max-sm:px-3.5',
  narrowShell: 'max-w-[640px]',
  publicShell:
    'mx-auto grid min-h-[calc(100vh-76px)] w-full max-w-[1440px] items-start gap-5 p-5 max-sm:px-3.5',
  homeGrid:
    'grid items-start gap-5 lg:grid-cols-[minmax(0,1.08fr)_minmax(340px,0.92fr)]',
  workspaceGrid:
    'grid items-start gap-5 xl:grid-cols-[minmax(0,0.95fr)_minmax(360px,1.05fr)]',
  builderGrid:
    'grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(340px,430px)]',
  builderSidebar: 'grid gap-5',
  appShell:
    'mx-auto grid min-h-[calc(100vh-104px)] w-full max-w-[1440px] gap-5 p-5 lg:grid-cols-[244px_minmax(0,1fr)] max-lg:min-h-0 max-sm:px-3.5',
  appContent: 'pb-5 pt-1 max-lg:pt-0',
  previewShell: 'grid gap-4',
}

export const topbar = {
  shell:
    'mx-auto mt-3 flex min-h-[72px] w-[min(100%-24px,1440px)] items-center justify-between gap-5 rounded-[18px] border border-border bg-[color-mix(in_oklch,var(--surface-1)_86%,transparent)] py-2.5 pl-4 pr-3 shadow-[var(--shadow-tight)] backdrop-blur-[18px] max-sm:grid max-sm:w-[min(100%-18px,1440px)] max-sm:p-3.5',
  brand:
    'flex items-center gap-2 text-[1.15rem] font-black tracking-[0.02em] text-[var(--paper)]',
  controls:
    'flex items-center gap-2 max-sm:grid max-sm:grid-cols-2',
  nav: 'flex flex-wrap gap-1.5 max-sm:grid max-sm:grid-cols-2',
  link:
    'rounded-full px-3.5 py-2.5 text-sm font-bold text-[var(--paper-muted)] transition-[background,color,transform] hover:-translate-y-px hover:bg-[var(--surface-2)] hover:text-[var(--paper)]',
}

export const form = {
  grid: 'grid gap-3',
  field: 'grid gap-2.5',
  toggle: cn(text.label, 'flex items-center gap-3 tracking-[0.06em]'),
  hint: '-mt-1 mb-0 text-sm text-[var(--paper-muted)]',
}

export const actions = {
  row: 'flex flex-wrap gap-2',
  rowLarge: 'flex flex-wrap gap-3',
  panelFooter: 'flex flex-wrap gap-2 px-6 pb-6',
  inlineLink:
    'rounded-full border border-border bg-[var(--surface-2)] px-3.5 py-2.5 text-sm font-bold text-[var(--paper)] transition-[background,border-color,transform] hover:-translate-y-px hover:border-[var(--thread-teal)] hover:bg-[var(--surface-3)] disabled:cursor-not-allowed disabled:opacity-45 disabled:hover:translate-y-0',
}

export const emptyState =
  'rounded-[16px] border border-dashed border-[color-mix(in_oklch,var(--border)_68%,var(--thread-teal))] bg-[color-mix(in_oklch,var(--surface-1)_82%,var(--thread-wood))] p-5'

export const statGrid = {
  list: 'grid grid-cols-3 gap-3 py-5 max-lg:grid-cols-1',
  item:
    'rounded-[14px] border border-border bg-[var(--surface-2)] p-4',
  value: 'mt-1.5 text-[1.45rem] font-black text-[var(--paper)]',
}

export const siteCard = {
  list: 'grid gap-4',
  card: cn(
    ribbon,
    'rounded-[16px] border border-border bg-[var(--surface-2)] p-5',
  ),
  meta: 'mt-4 grid grid-cols-2 gap-3 max-lg:grid-cols-1',
  actions: 'mt-4 flex flex-wrap gap-2',
}

export const preview = {
  toolbar:
    'flex items-center justify-between gap-4 rounded-[18px] border border-border bg-[var(--surface-1)] px-5 py-4 shadow-[var(--shadow-tight)] max-sm:flex-col max-sm:items-start max-sm:p-4',
  shell:
    'grid gap-5 rounded-[18px] border border-border bg-[var(--site-background)] p-5 text-[var(--site-foreground)] shadow-[var(--shadow-soft)] [font-family:var(--site-font-body)] max-sm:p-4',
  frame:
    'rounded-[var(--site-radius-panel)] border border-[var(--site-border)] bg-[var(--site-surface)]',
  header: 'grid gap-4 p-6 max-sm:p-4',
  nav: 'flex flex-wrap gap-2',
  navLink:
    'rounded-full bg-[var(--site-surface-muted)] px-[15px] py-2.5 text-[var(--site-foreground)]',
  page:
    'rounded-[var(--site-radius-panel)] border border-[var(--site-border)] bg-[var(--site-surface)] p-[calc(var(--site-section-spacing,96px)*0.24)]',
  pageMeta: 'mb-4 flex items-center justify-between gap-4',
  pageStack: 'grid gap-[calc(var(--site-section-spacing,96px)*0.2)]',
  panel: cn(
    ribbon,
    'rounded-[var(--site-radius-panel)] border border-[var(--site-border)] bg-[var(--site-surface-muted)] p-[calc(var(--site-section-spacing,96px)*0.25)]',
  ),
  hero:
    'p-[calc(var(--site-section-spacing,96px)*0.35)_calc(var(--site-section-spacing,96px)*0.28)]',
  actionRow: 'flex items-center justify-between gap-[18px] max-sm:flex-col max-sm:items-start',
  button:
    'inline-flex items-center justify-center rounded-full border border-[var(--site-border)] bg-[var(--site-primary)] px-[15px] py-2.5 font-bold text-[var(--site-background)]',
  ghostButton:
    'bg-[var(--site-surface-muted)] text-[var(--site-foreground)]',
  sectionHeading: 'mb-[18px]',
  features: 'grid grid-cols-3 gap-3.5 max-lg:grid-cols-1',
  feature:
    'rounded-[var(--site-radius-inner)] border border-[var(--site-border)] bg-[var(--site-surface-muted)] p-[18px]',
  split:
    'grid grid-cols-[minmax(0,1.2fr)_minmax(220px,0.8fr)] gap-[18px] max-lg:grid-cols-1',
  imagePlaceholder:
    'grid min-h-[220px] place-items-center rounded-[var(--site-radius-inner)] border border-[var(--site-border)] bg-[var(--site-surface)] p-[18px] text-[color-mix(in_srgb,var(--site-foreground)_74%,transparent)]',
}
