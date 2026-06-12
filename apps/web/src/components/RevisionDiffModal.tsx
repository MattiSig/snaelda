import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { SiteDraftRenderer } from '@/components/SiteDraftRenderer'
import { Button } from '@/components/ui/button'
import type {
  DraftRevisionRecord,
  RepromptHistoryRecord,
} from '@/lib/api'
import {
  buildRepromptDiff,
  type BlockDiff,
  type DiffTextPart,
} from '@/lib/reprompt-diff'
import { cn } from '@/lib/utils'

type RevisionDiffModalProps = {
  reprompt: RepromptHistoryRecord | null
  previousRevision: DraftRevisionRecord | null
  resultRevision: DraftRevisionRecord | null
  errorMessage?: string
  isLoading?: boolean
  onClose: () => void
}

type DiffViewMode = 'before' | 'after' | 'both'

type FlattenedChange = {
  pageKey: string
  pageTitle: string
  pageSlug: string
  block: BlockDiff
}

type ChangeHandle = {
  before?: HTMLElement | null
  after?: HTMLElement | null
}

const statusLabel: Record<string, string> = {
  added: 'Added',
  removed: 'Removed',
  modified: 'Changed',
}

const statusSurface: Record<string, string> = {
  added:
    'border-[color-mix(in_oklch,var(--thread-teal)_72%,var(--border))] bg-[color-mix(in_oklch,var(--thread-teal)_18%,var(--surface-2))]',
  removed:
    'border-[color-mix(in_oklch,var(--thread-coral)_72%,var(--border))] bg-[color-mix(in_oklch,var(--thread-coral)_14%,var(--surface-2))]',
  modified:
    'border-[color-mix(in_oklch,var(--thread-gold)_72%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_13%,var(--surface-2))]',
}

const viewModes: Array<{ id: DiffViewMode; label: string }> = [
  { id: 'both', label: 'Both' },
  { id: 'before', label: 'Before' },
  { id: 'after', label: 'After' },
]

const focusableSelector = [
  'button:not([disabled])',
  '[href]',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ')

export function RevisionDiffModal({
  reprompt,
  previousRevision,
  resultRevision,
  errorMessage,
  isLoading,
  onClose,
}: RevisionDiffModalProps) {
  if (!reprompt) {
    return null
  }

  return (
    <RevisionDiffDialog
      key={reprompt.id}
      reprompt={reprompt}
      previousRevision={previousRevision}
      resultRevision={resultRevision}
      errorMessage={errorMessage}
      isLoading={isLoading}
      onClose={onClose}
    />
  )
}

function RevisionDiffDialog({
  reprompt,
  previousRevision,
  resultRevision,
  errorMessage,
  isLoading,
  onClose,
}: {
  reprompt: RepromptHistoryRecord
  previousRevision: DraftRevisionRecord | null
  resultRevision: DraftRevisionRecord | null
  errorMessage?: string
  isLoading?: boolean
  onClose: () => void
}) {
  const dialogRef = useRef<HTMLDivElement | null>(null)
  const closeButtonRef = useRef<HTMLButtonElement | null>(null)
  const changeRefs = useRef(new Map<string, ChangeHandle>())
  const titleId = useId()
  const descriptionId = useId()
  const [viewMode, setViewMode] = useState<DiffViewMode>('both')
  const [activeChangeIndex, setActiveChangeIndex] = useState(0)

  const pageDiffs = useMemo(
    () =>
      reprompt && previousRevision && resultRevision
        ? buildRepromptDiff(reprompt, previousRevision, resultRevision)
        : [],
    [previousRevision, reprompt, resultRevision],
  )

  const changes = useMemo<FlattenedChange[]>(
    () =>
      pageDiffs.flatMap((pageDiff) =>
        pageDiff.blocks.map((block) => ({
          pageKey: pageDiff.key,
          pageTitle: pageDiff.title,
          pageSlug: pageDiff.afterPage?.slug || pageDiff.beforePage?.slug || '',
          block,
        })),
      ),
    [pageDiffs],
  )

  const changeIndexByKey = useMemo(() => {
    const pairs = changes.map((change, index) => [change.block.key, index] as const)
    return new Map<string, number>(pairs)
  }, [changes])

  const boundedChangeIndex = changes.length
    ? Math.min(activeChangeIndex, changes.length - 1)
    : 0
  const activeChange = changes[boundedChangeIndex] ?? null

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      closeButtonRef.current?.focus()
    })
    return () => {
      window.cancelAnimationFrame(frame)
    }
  }, [])

  useEffect(() => {
    if (!activeChange) {
      return
    }
    const handles = changeRefs.current.get(activeChange.block.key)
    const target =
      (viewMode === 'before' ? handles?.before : undefined) ||
      (viewMode === 'after' ? handles?.after : undefined) ||
      handles?.after ||
      handles?.before
    if (!target || typeof target.scrollIntoView !== 'function') {
      return
    }

    const frame = window.requestAnimationFrame(() => {
      target.scrollIntoView({
        block: 'center',
        inline: 'nearest',
        behavior: 'smooth',
      })
    })
    return () => window.cancelAnimationFrame(frame)
  }, [activeChange, viewMode])

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
        return
      }

      if (event.key === 'Tab') {
        trapFocus(event, dialogRef.current)
        return
      }

      if (!changes.length) {
        return
      }

      if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
        event.preventDefault()
        setActiveChangeIndex((current) => {
          const safeCurrent = Math.min(current, changes.length - 1)
          return (safeCurrent + 1) % changes.length
        })
        return
      }

      if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
        event.preventDefault()
        setActiveChangeIndex((current) =>
          Math.min(current, changes.length - 1) === 0
            ? changes.length - 1
            : Math.min(current, changes.length - 1) - 1,
        )
        return
      }

      if (event.key === 'Enter' && !isInteractiveTarget(event.target)) {
        event.preventDefault()
        setViewMode((current) => nextViewMode(current))
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [changes.length, onClose])

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center bg-[color-mix(in_oklch,var(--background)_58%,transparent)] p-4 backdrop-blur-[10px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <div
        ref={dialogRef}
        className="grid max-h-[min(94vh,1120px)] w-full max-w-[1380px] gap-4 overflow-hidden rounded-[22px] border border-[color-mix(in_oklch,var(--border)_82%,transparent)] bg-[var(--surface-1)] p-5 shadow-[0_28px_90px_color-mix(in_oklch,var(--background)_76%,transparent)]"
      >
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="grid gap-2">
            <p className="text-xs font-bold uppercase tracking-[0.12em] text-[var(--paper-muted)]">
              Revision diff
            </p>
            <h2
              id={titleId}
              className="text-[1.4rem] font-black leading-[1.04] text-[var(--paper)]"
            >
              {reprompt.changeSummary || 'Revision changes'}
            </h2>
            <p
              id={descriptionId}
              className="max-w-[72ch] text-sm text-[var(--paper-muted)]"
            >
              “{reprompt.prompt}”
            </p>
          </div>
          <Button
            ref={closeButtonRef}
            type="button"
            size="sm"
            variant="outline"
            onClick={onClose}
          >
            Close
          </Button>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_92%,var(--thread-mauve))] px-4 py-3">
          <div className="grid gap-1">
            <p className="text-sm font-semibold text-[var(--paper)]">
              {changes.length
                ? `Reviewing ${String(changes.length)} change${changes.length === 1 ? '' : 's'}`
                : 'No visible changes detected'}
            </p>
            <p className="text-xs text-[var(--paper-muted)]">
              Arrow keys move between changed blocks, Enter cycles before, after,
              and both, Esc closes.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <div className="flex flex-wrap gap-2">
              {viewModes.map((mode) => (
                <Button
                  key={mode.id}
                  type="button"
                  size="sm"
                  variant={viewMode === mode.id ? 'default' : 'outline'}
                  onClick={() => setViewMode(mode.id)}
                >
                  {mode.label}
                </Button>
              ))}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!changes.length}
                onClick={() =>
                  setActiveChangeIndex((current) =>
                    Math.min(current, changes.length - 1) === 0
                      ? changes.length - 1
                      : Math.min(current, changes.length - 1) - 1,
                  )
                }
              >
                Previous
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!changes.length}
                onClick={() =>
                  setActiveChangeIndex((current) =>
                    (Math.min(current, changes.length - 1) + 1) % changes.length,
                  )
                }
              >
                Next
              </Button>
            </div>
          </div>
        </div>

        {isLoading ? (
          <div className="rounded-[16px] border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[var(--surface-2)] p-5 text-sm text-[var(--paper-muted)]">
            Loading both checkpoints...
          </div>
        ) : null}

        {errorMessage ? (
          <div className="rounded-[16px] border border-[color-mix(in_oklch,var(--thread-coral)_72%,var(--border))] bg-[color-mix(in_oklch,var(--thread-coral)_14%,var(--surface-2))] p-5 text-sm font-semibold text-[var(--paper)]">
            {errorMessage}
          </div>
        ) : null}

        {!isLoading && !errorMessage ? (
          <div className="grid gap-4 overflow-y-auto pr-1">
            {pageDiffs.length ? (
              pageDiffs.map((pageDiff) => {
                const pageActiveChange =
                  pageDiff.blocks.find(
                    (blockDiff) => blockDiff.key === activeChange?.block.key,
                  ) || pageDiff.blocks[0]

                return (
                  <section
                    key={pageDiff.key}
                    className="grid gap-4 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[var(--surface-2)] p-4"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div className="grid gap-1">
                        <p className="text-[1.05rem] font-black text-[var(--paper)]">
                          {pageDiff.title}
                        </p>
                        <p className="text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                          {statusLabel[pageDiff.status] || 'Changed'} page
                        </p>
                      </div>
                      <p className="text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                        {pageDiff.afterPage?.slug || pageDiff.beforePage?.slug}
                      </p>
                    </div>

                    <div
                      className={cn(
                        'grid gap-4',
                        viewMode === 'both' ? 'xl:grid-cols-2' : 'grid-cols-1',
                      )}
                    >
                      {viewMode !== 'after' ? (
                        <RevisionPreviewPanel
                          label="Before"
                          revision={previousRevision}
                          page={pageDiff.beforePage}
                          changes={pageDiff.blocks}
                          activeChangeKey={activeChange?.block.key}
                          side="before"
                          onChangeFocus={(changeKey) => {
                            const nextIndex = changeIndexByKey.get(changeKey)
                            if (nextIndex !== undefined) {
                              setActiveChangeIndex(nextIndex)
                            }
                          }}
                          onChangeRef={(changeKey, element) => {
                            setChangeRef(changeRefs.current, changeKey, 'before', element)
                          }}
                        />
                      ) : null}

                      {viewMode !== 'before' ? (
                        <RevisionPreviewPanel
                          label="After"
                          revision={resultRevision}
                          page={pageDiff.afterPage}
                          changes={pageDiff.blocks}
                          activeChangeKey={activeChange?.block.key}
                          side="after"
                          onChangeFocus={(changeKey) => {
                            const nextIndex = changeIndexByKey.get(changeKey)
                            if (nextIndex !== undefined) {
                              setActiveChangeIndex(nextIndex)
                            }
                          }}
                          onChangeRef={(changeKey, element) => {
                            setChangeRef(changeRefs.current, changeKey, 'after', element)
                          }}
                        />
                      ) : null}
                    </div>

                    <div className="grid gap-3 rounded-[16px] border border-[color-mix(in_oklch,var(--border)_72%,transparent)] bg-[var(--surface-1)] p-4">
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <p className="text-xs font-bold uppercase tracking-[0.1em] text-[var(--paper-muted)]">
                          Changed blocks
                        </p>
                        {pageActiveChange ? (
                          <p className="text-sm font-semibold text-[var(--paper)]">
                            {pageActiveChange.summary}
                          </p>
                        ) : null}
                      </div>

                      <div className="flex flex-wrap gap-2">
                        {pageDiff.blocks.map((blockDiff) => (
                          <button
                            key={blockDiff.key}
                            type="button"
                            className={cn(
                              'rounded-full border px-3 py-1.5 text-xs font-bold uppercase tracking-[0.08em] transition-colors',
                              activeChange?.block.key === blockDiff.key
                                ? 'border-[var(--thread-teal)] bg-[color-mix(in_oklch,var(--thread-teal)_16%,var(--surface-2))] text-[var(--paper)]'
                                : 'border-[color-mix(in_oklch,var(--border)_72%,transparent)] bg-[var(--surface-2)] text-[var(--paper-muted)] hover:text-[var(--paper)]',
                            )}
                            onClick={() => {
                              const nextIndex = changeIndexByKey.get(blockDiff.key)
                              if (nextIndex !== undefined) {
                                setActiveChangeIndex(nextIndex)
                              }
                            }}
                          >
                            {statusLabel[blockDiff.status]} · {blockDiff.summary}
                          </button>
                        ))}
                      </div>

                      {pageActiveChange ? (
                        <ChangeFieldDetails blockDiff={pageActiveChange} />
                      ) : null}
                    </div>
                  </section>
                )
              })
            ) : (
              <div className="rounded-[16px] border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[var(--surface-2)] p-5 text-sm text-[var(--paper-muted)]">
                No visible draft changes were detected between these checkpoints.
              </div>
            )}
          </div>
        ) : null}
      </div>
    </div>
  )
}

function RevisionPreviewPanel({
  label,
  revision,
  page,
  changes,
  activeChangeKey,
  side,
  onChangeFocus,
  onChangeRef,
}: {
  label: string
  revision: DraftRevisionRecord | null
  page?: DraftRevisionRecord['draft']['pages'][number]
  changes: BlockDiff[]
  activeChangeKey?: string
  side: 'before' | 'after'
  onChangeFocus: (changeKey: string) => void
  onChangeRef: (changeKey: string, element: HTMLElement | null) => void
}) {
  if (!revision || !page) {
    return (
      <div className="grid min-h-[240px] place-items-center rounded-[18px] border border-dashed border-[color-mix(in_oklch,var(--border)_72%,transparent)] bg-[var(--surface-1)] p-6 text-center">
        <div className="grid gap-2">
          <p className="text-xs font-bold uppercase tracking-[0.1em] text-[var(--paper-muted)]">
            {label}
          </p>
          <p className="text-sm text-[var(--paper-muted)]">
            This page did not exist in this checkpoint.
          </p>
        </div>
      </div>
    )
  }

  const changeMap = new Map(
    changes
      .map((change) => {
        const block = side === 'before' ? change.beforeBlock : change.afterBlock
        if (!block) {
          return null
        }
        return [block.id, change] as const
      })
      .filter(Boolean) as Array<[string, BlockDiff]>,
  )

  return (
    <div
      data-testid={`revision-preview-${label.toLowerCase()}`}
      className="grid gap-3 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[var(--surface-1)] p-3"
    >
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-bold uppercase tracking-[0.1em] text-[var(--paper-muted)]">
          {label}
        </p>
        <p className="text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)]">
          {page.slug}
        </p>
      </div>

      <div className="max-h-[52vh] overflow-y-auto rounded-[16px] border border-[color-mix(in_oklch,var(--border)_70%,transparent)] bg-[color-mix(in_oklch,var(--surface-2)_90%,var(--background))] p-3">
        <SiteDraftRenderer
          site={revision.draft}
          selectedPageId={page.id}
          showPageMeta={false}
          renderBlock={({ block, children }) => {
            const change = changeMap.get(block.id)
            if (!change) {
              return children
            }

            const isActive = activeChangeKey === change.key
            return (
              <div
                key={block.id}
                ref={(element) => onChangeRef(change.key, element)}
                tabIndex={-1}
                className={cn(
                  'relative scroll-mt-8 rounded-[24px] border-2 p-2 transition-shadow',
                  statusSurface[change.status] || statusSurface.modified,
                  isActive &&
                    'ring-2 ring-[color-mix(in_oklch,var(--thread-teal)_76%,transparent)] shadow-[0_18px_44px_color-mix(in_oklch,var(--background)_42%,transparent)]',
                )}
              >
                <button
                  type="button"
                  className="absolute right-3 top-3 z-10 rounded-full bg-[color-mix(in_oklch,var(--surface-1)_92%,var(--background))] px-2.5 py-1 text-[11px] font-bold uppercase tracking-[0.08em] text-[var(--paper)] shadow-[0_10px_24px_color-mix(in_oklch,var(--background)_22%,transparent)]"
                  onClick={() => onChangeFocus(change.key)}
                >
                  {statusLabel[change.status]}
                </button>
                {children}
              </div>
            )
          }}
        />
      </div>
    </div>
  )
}

function ChangeFieldDetails({ blockDiff }: { blockDiff: BlockDiff }) {
  return (
    <div
      className={cn(
        'grid gap-4 rounded-[16px] border p-4',
        statusSurface[blockDiff.status] || statusSurface.modified,
      )}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="grid gap-1">
          <p className="text-sm font-black uppercase tracking-[0.08em] text-[var(--paper)]">
            {blockDiff.summary}
          </p>
          <p className="text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)]">
            {statusLabel[blockDiff.status]} block
          </p>
        </div>
      </div>

      {blockDiff.fields.length ? (
        <div className="grid gap-3 lg:grid-cols-2">
          <FieldColumn label="Before" fields={blockDiff.fields} side="before" />
          <FieldColumn label="After" fields={blockDiff.fields} side="after" />
        </div>
      ) : (
        <p className="text-sm text-[var(--paper-muted)]">
          The visual structure changed even though no simple text fields were
          captured separately.
        </p>
      )}
    </div>
  )
}

function FieldColumn({
  label,
  fields,
  side,
}: {
  label: string
  fields: BlockDiff['fields']
  side: 'before' | 'after'
}) {
  return (
    <div className="grid gap-2 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_78%,transparent)] bg-[var(--surface-1)] p-3">
      <p className="text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
        {label}
      </p>
      <div className="grid gap-2">
        {fields.map((field) => {
          const value = side === 'before' ? field.before : field.after
          const parts = side === 'before' ? field.beforeParts : field.afterParts

          return (
            <div key={`${side}-${field.key}`} className="grid gap-1">
              <p className="text-[11px] font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                {field.key}
              </p>
              <div className="rounded-[12px] bg-[color-mix(in_oklch,var(--surface-2)_90%,var(--background))] p-3 text-sm leading-6 text-[var(--paper)]">
                {value ? (
                  <HighlightedText parts={parts} />
                ) : (
                  <span className="text-[var(--paper-muted)]">
                    {side === 'before' ? 'Not present' : 'Added in this revision'}
                  </span>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function HighlightedText({ parts }: { parts: DiffTextPart[] }) {
  return (
    <span className="whitespace-pre-wrap break-words">
      {parts.map((part, index) =>
        part.changed ? (
          <mark
            key={`${part.value}-${String(index)}`}
            className="rounded-[6px] bg-[color-mix(in_oklch,var(--thread-gold)_28%,var(--surface-1))] px-1 py-0.5 text-[var(--paper)]"
          >
            {part.value}
          </mark>
        ) : (
          <span key={`${part.value}-${String(index)}`}>{part.value}</span>
        ),
      )}
    </span>
  )
}

function setChangeRef(
  changeRefs: Map<string, ChangeHandle>,
  changeKey: string,
  side: 'before' | 'after',
  element: HTMLElement | null,
) {
  const current = changeRefs.get(changeKey) ?? {}
  current[side] = element
  changeRefs.set(changeKey, current)
}

function nextViewMode(current: DiffViewMode): DiffViewMode {
  if (current === 'both') {
    return 'before'
  }
  if (current === 'before') {
    return 'after'
  }
  return 'both'
}

function isInteractiveTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false
  }
  const tag = target.tagName
  if (tag === 'BUTTON' || tag === 'A' || tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
    return true
  }
  return target.isContentEditable
}

function trapFocus(event: KeyboardEvent, container: HTMLElement | null) {
  if (!container) {
    return
  }
  const focusable = [
    ...container.querySelectorAll<HTMLElement>(focusableSelector),
  ].filter((element) => !element.hasAttribute('disabled'))

  if (focusable.length === 0) {
    return
  }

  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const active = document.activeElement

  if (event.shiftKey) {
    if (active === first || !container.contains(active)) {
      event.preventDefault()
      last.focus()
    }
    return
  }

  if (active === last) {
    event.preventDefault()
    first.focus()
  }
}
