import { Check, Dot } from 'lucide-react'
import { paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export type GenerationProgressItem = {
  step: string
  label: string
}

export type GenerationShadowBlock = {
  type: string
  hasCopy: boolean
}

export type GenerationShadowPage = {
  title: string
  slug: string
  goal?: string
  blocks: GenerationShadowBlock[]
}

const shimmerBlocks = [
  'h-24 rounded-[18px]',
  'h-16 rounded-[16px]',
  'h-20 rounded-[16px]',
]

export function GenerationProgressCard({
  eyebrow,
  title,
  description,
  prompt,
  steps,
  activeStep,
  activeTotal,
  showSkeleton = false,
  previewTitle = 'Preview taking shape',
  idlePreviewText = "We'll sketch likely sections here once the structure is in place.",
  shadowPages,
}: {
  eyebrow: string
  title: string
  description: string
  prompt?: string
  steps: GenerationProgressItem[]
  activeStep?: string
  activeTotal?: number
  showSkeleton?: boolean
  previewTitle?: string
  idlePreviewText?: string
  shadowPages?: GenerationShadowPage[]
}) {
  const renderedSteps =
    activeTotal && activeTotal < steps.length
      ? steps.filter((item) => item.step !== 'assets.fetch')
      : steps
  const activeIndex = renderedSteps.findIndex((item) => item.step === activeStep)
  const hasShadow = Boolean(shadowPages && shadowPages.length > 0)

  return (
    <section className={cn(paddedPanel, 'rounded-[16px]')}>
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(260px,0.7fr)]">
        <div className="grid gap-4">
          <div className="grid gap-2">
            <p className={text.eyebrow}>{eyebrow}</p>
            <h1 className={text.sectionTitle}>{title}</h1>
            <p className={text.p}>{description}</p>
            {prompt ? (
              <p className="rounded-[14px] border border-border bg-[var(--surface-2)] px-4 py-3 text-sm italic text-[var(--paper-muted)]">
                "{prompt}"
              </p>
            ) : null}
          </div>

          <ol
            aria-live="polite"
            className="grid gap-2.5"
          >
            {renderedSteps.map((item, index) => {
              const isActive = activeStep === item.step
              const isComplete = activeIndex > index
              return (
                <li
                  key={item.step}
                  className={cn(
                    'grid grid-cols-[auto_minmax(0,1fr)] items-center gap-3 rounded-[14px] border px-4 py-3 transition-[border-color,background,transform]',
                    isActive
                      ? 'border-[color-mix(in_oklch,var(--thread-gold)_70%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_10%,var(--surface-2))] shadow-[0_0_0_1px_color-mix(in_oklch,var(--thread-gold)_22%,transparent)]'
                      : isComplete
                        ? 'border-[color-mix(in_oklch,var(--thread-teal)_58%,var(--border))] bg-[color-mix(in_oklch,var(--thread-teal)_10%,var(--surface-2))]'
                        : 'border-border bg-[var(--surface-1)]',
                  )}
                >
                  <span
                    className={cn(
                      'flex size-8 items-center justify-center rounded-full border text-[var(--paper)]',
                      isActive
                        ? 'border-[var(--thread-gold)] bg-[color-mix(in_oklch,var(--thread-gold)_18%,var(--surface-2))] motion-safe:animate-pulse'
                        : isComplete
                          ? 'border-[var(--thread-teal)] bg-[color-mix(in_oklch,var(--thread-teal)_16%,var(--surface-2))]'
                          : 'border-border bg-[var(--surface-2)] text-[var(--paper-muted)]',
                    )}
                  >
                    {isComplete ? <Check className="size-4" /> : <Dot className="size-5" />}
                  </span>
                  <div className="min-w-0">
                    <p className="text-sm font-semibold text-[var(--paper)]">
                      {item.label}
                    </p>
                    <p className="mt-1 text-xs uppercase tracking-[0.12em] text-[var(--paper-muted)]">
                      Step {index + 1}
                    </p>
                  </div>
                </li>
              )
            })}
          </ol>
        </div>

        <div className="grid content-start gap-3 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_82%,var(--thread-mauve))] bg-[linear-gradient(180deg,color-mix(in_oklch,var(--surface-2)_92%,transparent),color-mix(in_oklch,var(--surface-1)_96%,transparent))] p-4">
          <p className={text.label}>{previewTitle}</p>
          {hasShadow ? (
            <ShadowDraft pages={shadowPages!} />
          ) : showSkeleton ? (
            <div className="grid gap-3">
              {shimmerBlocks.map((className, index) => (
                <div
                  key={index}
                  className={cn(
                    className,
                    'motion-safe:animate-pulse border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[linear-gradient(115deg,var(--surface-2),color-mix(in_oklch,var(--thread-mauve)_16%,var(--surface-3)),var(--surface-2))]',
                  )}
                />
              ))}
            </div>
          ) : (
            <p className="text-sm text-[var(--paper-muted)]">
              {idlePreviewText}
            </p>
          )}
        </div>
      </div>
    </section>
  )
}

function ShadowDraft({ pages }: { pages: GenerationShadowPage[] }) {
  return (
    <ol className="grid gap-3">
      {pages.map((page) => (
        <li
          key={page.slug || page.title}
          className="grid gap-2 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[var(--surface-2)] px-3 py-3"
        >
          <div className="grid gap-1">
            <p className="text-[0.65rem] uppercase tracking-[0.12em] text-[var(--paper-muted)]">
              {page.slug || '/'}
            </p>
            <p className="text-sm font-bold text-[var(--paper)]">{page.title}</p>
            {page.goal ? (
              <p className="line-clamp-2 text-xs text-[var(--paper-muted)]">
                {page.goal}
              </p>
            ) : null}
          </div>
          {page.blocks.length > 0 ? (
            <ul className="grid gap-1.5">
              {page.blocks.map((block, blockIndex) => (
                <li
                  key={blockIndex}
                  className={cn(
                    'flex items-center justify-between gap-2 rounded-[10px] border px-2.5 py-1.5 text-xs',
                    block.hasCopy
                      ? 'border-[color-mix(in_oklch,var(--thread-teal)_55%,var(--border))] bg-[color-mix(in_oklch,var(--thread-teal)_10%,var(--surface-1))] text-[var(--paper)]'
                      : 'border-dashed border-border bg-[color-mix(in_oklch,var(--surface-1)_92%,transparent)] text-[var(--paper-muted)] motion-safe:animate-pulse',
                  )}
                >
                  <span className="truncate font-semibold">{prettifyBlockType(block.type)}</span>
                  {block.hasCopy ? (
                    <Check className="size-3" />
                  ) : (
                    <Dot className="size-3" />
                  )}
                </li>
              ))}
            </ul>
          ) : (
            <div className="h-6 rounded-[10px] border border-dashed border-border bg-[var(--surface-1)] motion-safe:animate-pulse" />
          )}
        </li>
      ))}
    </ol>
  )
}

function prettifyBlockType(type: string): string {
  if (!type) {
    return 'Block'
  }
  return type
    .split(/[._-]/)
    .map((part) => (part ? part[0].toUpperCase() + part.slice(1) : part))
    .join(' ')
}
