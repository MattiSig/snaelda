import { Button } from '@/components/ui/button'
import type {
  DraftRevisionRecord,
  RepromptHistoryRecord,
} from '@/lib/api'
import { buildRepromptDiff } from '@/lib/reprompt-diff'
import { cn } from '@/lib/utils'

type RevisionDiffModalProps = {
  reprompt: RepromptHistoryRecord | null
  previousRevision: DraftRevisionRecord | null
  resultRevision: DraftRevisionRecord | null
  errorMessage?: string
  isLoading?: boolean
  onClose: () => void
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

  const pageDiffs =
    previousRevision && resultRevision
      ? buildRepromptDiff(reprompt, previousRevision, resultRevision)
      : []

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center bg-[color-mix(in_oklch,var(--background)_58%,transparent)] p-4 backdrop-blur-[10px]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="reprompt-diff-title"
    >
      <div className="grid max-h-[min(92vh,980px)] w-full max-w-[1200px] gap-4 overflow-hidden rounded-[22px] border border-[color-mix(in_oklch,var(--border)_82%,transparent)] bg-[var(--surface-1)] p-5 shadow-[0_28px_90px_color-mix(in_oklch,var(--background)_76%,transparent)]">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="grid gap-2">
            <p className="text-xs font-bold uppercase tracking-[0.12em] text-[var(--paper-muted)]">
              Diff view
            </p>
            <h2
              id="reprompt-diff-title"
              className="text-[1.4rem] font-black leading-[1.04] text-[var(--paper)]"
            >
              {reprompt.changeSummary || 'Revision changes'}
            </h2>
            <p className="max-w-[70ch] text-sm text-[var(--paper-muted)]">
              “{reprompt.prompt}”
            </p>
          </div>
          <Button type="button" size="sm" variant="outline" onClick={onClose}>
            Close
          </Button>
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
              pageDiffs.map((pageDiff) => (
                <section
                  key={pageDiff.key}
                  className="grid gap-3 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[var(--surface-2)] p-4"
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

                  <div className="grid gap-3">
                    {pageDiff.blocks.map((blockDiff) => (
                      <article
                        key={blockDiff.key}
                        className={cn(
                          'grid gap-4 rounded-[16px] border p-4',
                          statusSurface[blockDiff.status] || statusSurface.modified,
                        )}
                      >
                        <div className="flex flex-wrap items-center justify-between gap-3">
                          <p className="text-sm font-black uppercase tracking-[0.08em] text-[var(--paper)]">
                            {blockDiff.afterBlock?.type || blockDiff.beforeBlock?.type}
                          </p>
                          <p className="text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                            {statusLabel[blockDiff.status] || 'Changed'}
                          </p>
                        </div>

                        <div className="grid gap-3 lg:grid-cols-2">
                          <div className="grid gap-2 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_78%,transparent)] bg-[var(--surface-1)] p-3">
                            <p className="text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                              Before
                            </p>
                            {blockDiff.beforeBlock ? (
                              <div className="grid gap-2">
                                {blockDiff.fields.map((field) => (
                                  <div key={`${blockDiff.key}-before-${field.key}`} className="grid gap-1">
                                    <p className="text-[11px] font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                                      {field.key}
                                    </p>
                                    <pre className="overflow-x-auto whitespace-pre-wrap rounded-[12px] bg-[color-mix(in_oklch,var(--surface-2)_90%,var(--background))] p-3 text-xs leading-5 text-[var(--paper)]">
                                      {field.before || 'Removed'}
                                    </pre>
                                  </div>
                                ))}
                              </div>
                            ) : (
                              <p className="text-sm text-[var(--paper-muted)]">
                                This block did not exist yet.
                              </p>
                            )}
                          </div>

                          <div className="grid gap-2 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_78%,transparent)] bg-[var(--surface-1)] p-3">
                            <p className="text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                              After
                            </p>
                            {blockDiff.afterBlock ? (
                              <div className="grid gap-2">
                                {blockDiff.fields.map((field) => (
                                  <div key={`${blockDiff.key}-after-${field.key}`} className="grid gap-1">
                                    <p className="text-[11px] font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                                      {field.key}
                                    </p>
                                    <pre className="overflow-x-auto whitespace-pre-wrap rounded-[12px] bg-[color-mix(in_oklch,var(--surface-2)_90%,var(--background))] p-3 text-xs leading-5 text-[var(--paper)]">
                                      {field.after || 'Added'}
                                    </pre>
                                  </div>
                                ))}
                              </div>
                            ) : (
                              <p className="text-sm text-[var(--paper-muted)]">
                                This block was removed.
                              </p>
                            )}
                          </div>
                        </div>
                      </article>
                    ))}
                  </div>
                </section>
              ))
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
