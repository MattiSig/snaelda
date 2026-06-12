import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { RepromptHistoryRecord, RepromptScope } from '@/lib/api'

type RepromptHistoryPanelProps = {
  reprompts: RepromptHistoryRecord[]
  activeScope: 'site' | 'page' | 'block'
  selectedPageId?: string
  selectedPageTitle?: string
  selectedBlockId?: string
  selectedBlockLabel?: string
  activeDiffId?: string
  activeRevertId?: string
  onActiveScopeChange: (scope: 'site' | 'page' | 'block') => void
  onShowDiff: (reprompt: RepromptHistoryRecord) => void
  onRevert: (reprompt: RepromptHistoryRecord) => void
}

export function RepromptHistoryPanel({
  reprompts,
  activeScope,
  selectedPageId,
  selectedPageTitle,
  selectedBlockId,
  selectedBlockLabel,
  activeDiffId,
  activeRevertId,
  onActiveScopeChange,
  onShowDiff,
  onRevert,
}: RepromptHistoryPanelProps) {
  const scopedReprompts = reprompts.filter((reprompt) => {
    if (activeScope === 'page') {
      return (
        reprompt.scope === 'page' &&
        Boolean(selectedPageId) &&
        reprompt.targetId === selectedPageId
      )
    }
    if (activeScope === 'block') {
      return (
        reprompt.scope === 'block' &&
        Boolean(selectedBlockId) &&
        reprompt.targetId === selectedBlockId
      )
    }
    return true
  })

  return (
    <div className="grid gap-4 rounded-[16px] border border-[color-mix(in_oklch,var(--border)_76%,transparent)] bg-[color-mix(in_oklch,var(--surface-1)_92%,var(--thread-mauve))] p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="grid gap-1">
          <p className="text-xs font-bold uppercase tracking-[0.1em] text-[var(--paper-muted)]">
            History
          </p>
          <p className="text-sm text-[var(--paper-muted)]">
            Review what changed before you keep pulling on the thread.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            size="sm"
            variant={activeScope === 'site' ? 'default' : 'outline'}
            onClick={() => onActiveScopeChange('site')}
          >
            Whole site
          </Button>
          <Button
            type="button"
            size="sm"
            variant={activeScope === 'page' ? 'default' : 'outline'}
            disabled={!selectedPageId}
            onClick={() => onActiveScopeChange('page')}
          >
            {selectedPageTitle ? `${selectedPageTitle} page` : 'Selected page'}
          </Button>
          <Button
            type="button"
            size="sm"
            variant={activeScope === 'block' ? 'default' : 'outline'}
            disabled={!selectedBlockId}
            onClick={() => onActiveScopeChange('block')}
          >
            {selectedBlockLabel ? selectedBlockLabel : 'Selected block'}
          </Button>
        </div>
      </div>

      {scopedReprompts.length ? (
        <div className="grid gap-3">
          {scopedReprompts.map((reprompt) => (
            <article
              key={reprompt.id}
              className="grid gap-3 rounded-[14px] border border-[color-mix(in_oklch,var(--border)_78%,transparent)] bg-[var(--surface-2)] p-4"
            >
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="grid gap-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-bold text-[var(--paper)]">
                      {reprompt.changeSummary || reprompt.prompt}
                    </p>
                    <span className="rounded-full bg-[color-mix(in_oklch,var(--thread-mauve)_18%,var(--surface-1))] px-2.5 py-1 text-[11px] font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                      {scopeLabel(reprompt.scope)}
                    </span>
                  </div>
                  <p className="text-xs uppercase tracking-[0.08em] text-[var(--paper-muted)]">
                    {formatRepromptTime(reprompt.createdAt)}
                  </p>
                </div>
              </div>
              <p className="text-sm text-[var(--paper-muted)]">
                “{reprompt.prompt}”
              </p>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p
                  className={cn(
                    'text-xs font-bold uppercase tracking-[0.08em]',
                    reprompt.undoneAt
                      ? 'text-[var(--thread-teal)]'
                      : 'text-[var(--paper-muted)]',
                  )}
                >
                  {reprompt.undoneAt ? 'Reverted' : 'Saved as a checkpoint'}
                </p>
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={activeDiffId === reprompt.id}
                    onClick={() => onShowDiff(reprompt)}
                  >
                    {activeDiffId === reprompt.id ? 'Loading diff...' : 'Show diff'}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={Boolean(reprompt.undoneAt) || activeRevertId === reprompt.id}
                    onClick={() => onRevert(reprompt)}
                  >
                    {reprompt.undoneAt
                      ? 'Restored'
                      : activeRevertId === reprompt.id
                        ? 'Restoring...'
                        : 'Revert'}
                  </Button>
                </div>
              </div>
            </article>
          ))}
        </div>
      ) : (
        <div className="rounded-[14px] border border-dashed border-[color-mix(in_oklch,var(--border)_68%,var(--thread-teal))] bg-[color-mix(in_oklch,var(--surface-1)_82%,var(--thread-wood))] p-4 text-sm text-[var(--paper-muted)]">
          {emptyStateLabel(activeScope)}
        </div>
      )}
    </div>
  )
}

function formatRepromptTime(value: string) {
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) {
    return 'Recently'
  }
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(timestamp)
}

function emptyStateLabel(scope: 'site' | 'page' | 'block') {
  if (scope === 'page') {
    return 'The selected page does not have any AI revisions yet.'
  }
  if (scope === 'block') {
    return 'The selected block does not have any AI revisions yet.'
  }
  return 'This site does not have any AI revisions yet.'
}

function scopeLabel(scope: RepromptScope) {
  switch (scope) {
    case 'block':
      return 'Block'
    case 'entry':
      return 'Entry'
    case 'theme':
      return 'Theme'
    case 'page':
      return 'Page'
    default:
      return 'Site'
  }
}
