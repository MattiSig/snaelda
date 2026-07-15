import { cn } from "@/lib/utils";

export const text = {
  eyebrow:
    "text-xs font-bold uppercase tracking-[0.12em] text-[var(--paper-muted)]",
  label:
    "text-xs font-bold uppercase tracking-[0.1em] text-[var(--paper-muted)]",
  h1: "max-w-[12ch] text-[clamp(3rem,7vw,6.4rem)] font-black leading-[0.9] text-[var(--paper)] text-balance",
  h2: "text-[clamp(1.75rem,3.4vw,3rem)] font-black leading-[0.96] text-[var(--paper)] text-balance",
  h3: "text-[1.2rem] font-extrabold leading-[1.05] text-[var(--paper)] text-balance",
  appTitle:
    "max-w-[13ch] text-[2.25rem] font-black leading-[1.02] text-[var(--paper)] sm:text-[2.7rem]",
  sectionTitle:
    "text-[1.35rem] font-black leading-[1.08] text-[var(--paper)] sm:text-[1.55rem]",
  sectionLead: "max-w-[62ch] text-[1rem] leading-7 text-[var(--paper-muted)]",
  p: "max-w-[68ch] text-[var(--paper-muted)]",
  muted: "text-[color-mix(in_oklch,var(--paper-muted)_72%,var(--background))]",
  error: "m-0 font-bold text-destructive",
  success: "m-0 font-bold text-[var(--success)]",
};

export const ribbon =
  "relative overflow-hidden before:pointer-events-none before:absolute before:inset-x-5 before:top-0 before:h-px before:bg-[color-mix(in_oklch,var(--thread-coral)_60%,var(--thread-gold))] before:opacity-40 after:pointer-events-none after:absolute after:right-7 after:top-0 after:h-5 after:w-px after:bg-[var(--thread-teal)] after:opacity-30";

export const panel = cn(
  "rounded-[14px] border border-[color-mix(in_oklch,var(--border)_80%,transparent)]",
  "bg-[var(--surface-1)]",
);

export const paddedPanel = cn(panel, "p-6 max-sm:p-4");
export const ribbonPanel = cn(paddedPanel, ribbon);

export const layout = {
  pageShell: "mx-auto w-full max-w-[1440px] px-5 pb-8 pt-8 max-sm:px-3.5",
  narrowShell: "max-w-[640px]",
  publicShell:
    "mx-auto grid min-h-[calc(100vh-76px)] w-full max-w-[1440px] items-start gap-5 p-5 max-sm:px-3.5",
  homeGrid:
    "grid items-start gap-5 lg:grid-cols-[minmax(0,1.08fr)_minmax(340px,0.92fr)]",
  workspaceGrid:
    "grid items-start gap-5 xl:grid-cols-[minmax(0,0.95fr)_minmax(360px,1.05fr)]",
  siteHomeGrid:
    "grid items-start gap-5 xl:grid-cols-[minmax(0,1.18fr)_minmax(320px,0.82fr)]",
  builderGrid:
    "grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(380px,520px)]",
  builderSidebar: "grid gap-5",
  appShell:
    "mx-auto grid min-h-[calc(100vh-72px)] w-full max-w-[1760px] gap-4 px-5 pb-5 pt-4 max-lg:min-h-0 max-sm:px-3.5",
  appContent: "pb-5",
  previewShell: "grid gap-4",
};

export const topbar = {
  shell:
    "mx-auto mt-3 flex min-h-[64px] w-[min(100%-24px,1440px)] items-center justify-between gap-5 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_70%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_92%,transparent)] py-2 pl-4 pr-3 backdrop-blur-[12px] max-sm:grid max-sm:w-[min(100%-18px,1440px)] max-sm:p-3.5",
  brand:
    "flex items-center gap-2 text-[1.15rem] font-black tracking-[0.02em] text-[var(--paper)]",
  controls: "flex items-center gap-2 max-sm:grid max-sm:grid-cols-2",
  nav: "flex flex-wrap gap-1.5 max-sm:grid max-sm:grid-cols-2",
  link: "rounded-full px-3.5 py-2.5 text-sm font-bold text-[var(--paper-muted)] transition-[background,color,transform] hover:-translate-y-px hover:bg-[var(--surface-2)] hover:text-[var(--paper)]",
};

export const form = {
  grid: "grid gap-3",
  field: "grid gap-2.5",
  toggle: cn(text.label, "flex items-center gap-3 tracking-[0.06em]"),
  hint: "-mt-1 mb-0 text-sm text-[var(--paper-muted)]",
};

export const actions = {
  row: "flex flex-wrap gap-2",
  rowLarge: "flex flex-wrap gap-3",
  panelFooter: "flex flex-wrap gap-2 px-6 pb-6",
  inlineLink:
    "rounded-full border border-border bg-[var(--surface-2)] px-3.5 py-2.5 text-sm font-bold text-[var(--paper)] transition-[background,border-color,transform] hover:-translate-y-px hover:border-[var(--thread-teal)] hover:bg-[var(--surface-3)] disabled:cursor-not-allowed disabled:opacity-45 disabled:hover:translate-y-0",
};

export const emptyState =
  "rounded-[16px] border border-dashed border-[color-mix(in_oklch,var(--border)_68%,var(--thread-teal))] bg-[color-mix(in_oklch,var(--surface-1)_82%,var(--thread-wood))] p-5";

export const statGrid = {
  list: "grid grid-cols-3 gap-3 py-5 max-lg:grid-cols-1",
  item: "rounded-[14px] border border-border bg-[var(--surface-2)] p-4",
  value: "mt-1.5 text-[1.45rem] font-black text-[var(--paper)]",
};

export const siteCard = {
  list: "grid gap-1.5",
  card: cn("grid gap-3 rounded-[14px] px-1 py-4 first:pt-1.5"),
  row: "flex items-start justify-between gap-4 max-sm:grid",
  meta: "flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-[var(--paper-muted)]",
  actions: "flex flex-wrap gap-2",
};

export const preview = {
  toolbar:
    "flex items-center justify-between gap-4 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_70%,transparent)] bg-[var(--surface-1)] px-5 py-3.5 max-sm:flex-col max-sm:items-start max-sm:p-4",
  shell:
    "w-full bg-[var(--color-background)] text-[var(--color-text)] [font-family:var(--font-body)] [font-weight:var(--font-bodyWeight,400)]",
  frame: "w-full",
  header:
    "w-full border-b border-[color-mix(in_oklch,var(--color-border)_55%,transparent)]",
  headerInner:
    "mx-auto flex w-full max-w-[1180px] flex-wrap items-center justify-between gap-x-10 gap-y-4 px-[max(1.25rem,4vw)] py-6",
  headerBrand:
    "text-[1.35rem] leading-none tracking-tight text-[var(--color-text)] [font-family:var(--font-heading)] [font-weight:var(--font-headingWeight,700)]",
  nav: "flex flex-wrap items-baseline gap-x-6 gap-y-2",
  navLink:
    "text-sm font-medium text-[color-mix(in_oklch,var(--color-text)_72%,var(--color-background))] transition-colors hover:text-[var(--color-text)]",
  page: "w-full",
  pageMeta:
    "mx-auto mb-2 flex w-full max-w-[1180px] items-center justify-between gap-4 px-[max(1.25rem,4vw)] pt-6 text-xs uppercase tracking-[0.12em] text-[color-mix(in_oklch,var(--color-text)_55%,var(--color-background))]",
  pageStack: "grid",
  panel:
    "w-full px-[max(var(--space-sectionPaddingX,24px),4vw)] py-[clamp(56px,calc(var(--space-sectionPaddingY,96px)*0.75),120px)]",
  panelInner: "mx-auto w-full max-w-[1180px]",
  panelNarrow:
    "mx-auto w-full max-w-[min(var(--size-contentWidth,720px),68ch)]",
  hero: "py-[clamp(80px,14vw,180px)]",
  ctaSurface:
    "bg-[var(--color-text)] text-[var(--color-background)] [--color-buttonBackground:var(--color-background)] [--color-buttonForeground:var(--color-text)] [--color-buttonBorder:var(--color-background)] [--color-buttonGhostForeground:var(--color-background)] [--color-buttonGhostBorder:var(--color-background)]",
  actionRow: "flex flex-wrap items-center gap-3",
  button:
    "inline-flex items-center justify-center rounded-[var(--radius-inner)] border border-[var(--color-buttonBorder)] bg-[var(--color-buttonBackground)] px-5 py-3 text-sm font-semibold text-[var(--color-buttonForeground)] shadow-[var(--shadow-button)] transition-transform hover:-translate-y-px",
  ghostButton:
    "border-[var(--color-buttonGhostBorder)] bg-transparent text-[var(--color-buttonGhostForeground)] shadow-none hover:bg-[color-mix(in_oklch,var(--color-text)_6%,transparent)]",
  sectionHeading: "mb-12 grid max-w-[60ch] gap-3",
  features: "grid grid-cols-1 gap-x-10 gap-y-12 md:grid-cols-2 xl:grid-cols-3",
  feature: "grid gap-2",
  cardGrid: "grid gap-x-10 gap-y-12 md:grid-cols-2 xl:grid-cols-3",
  split:
    "grid grid-cols-1 gap-10 lg:grid-cols-[minmax(0,1.1fr)_minmax(280px,0.9fr)] lg:items-center",
  imagePlaceholder:
    "grid min-h-[260px] place-items-center rounded-[var(--radius-inner)] bg-[color-mix(in_oklch,var(--color-surface-muted)_92%,var(--color-text))] p-5 text-sm text-[color-mix(in_oklch,var(--color-text)_55%,var(--color-background))]",
  imagePlaceholderTall:
    "grid min-h-[320px] place-items-end rounded-[var(--radius-inner)] p-5 text-left [background-image:var(--image-tallBackground)]",
  quoteCard:
    "grid gap-5 border-t border-[color-mix(in_oklch,var(--color-border)_55%,transparent)] pt-8",
  pricingGrid: "grid gap-5 md:grid-cols-2 xl:grid-cols-3",
  pricingCard:
    "grid gap-5 rounded-[var(--radius-inner)] border border-[var(--color-border)] bg-[var(--color-surface)] p-7",
  chipList:
    "flex flex-col gap-1.5 text-[color-mix(in_oklch,var(--color-text)_82%,var(--color-background))]",
  chip: "flex items-baseline gap-2 text-sm",
  faqList:
    "divide-y divide-[color-mix(in_oklch,var(--color-border)_55%,transparent)]",
  faqItem: "grid gap-3 py-7 first:pt-0 last:pb-0",
  footerShell:
    "w-full border-t border-[color-mix(in_oklch,var(--color-border)_55%,transparent)] px-[max(1.25rem,4vw)] py-14",
  footerInner:
    "mx-auto grid w-full max-w-[1180px] gap-10 md:grid-cols-[minmax(0,1.3fr)_minmax(220px,0.7fr)]",
  footerLinks: "flex flex-wrap gap-x-6 gap-y-2",
  footerLink:
    "text-sm text-[color-mix(in_oklch,var(--color-text)_78%,var(--color-background))] underline-offset-4 hover:underline",
};
