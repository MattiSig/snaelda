import { useState } from 'react'
import { Layers, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import type { SeedCollectionInput, SeedCollectionSuggestion } from '@/lib/api'
import { actions, paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

type CollectionIntakeFormProps = {
  prompt: string
  suggestions: SeedCollectionSuggestion[]
  isSubmitting?: boolean
  onSubmit: (collections: SeedCollectionInput[]) => void
  onSkip: () => void
}

type DraftCollection = {
  enabled: boolean
  itemsText: string
}

export function CollectionIntakeForm({
  prompt,
  suggestions,
  isSubmitting = false,
  onSubmit,
  onSkip,
}: CollectionIntakeFormProps) {
  const [drafts, setDrafts] = useState<Record<string, DraftCollection>>(() => {
    const seed: Record<string, DraftCollection> = {}
    for (const suggestion of suggestions) {
      seed[suggestion.id] = { enabled: true, itemsText: '' }
    }
    return seed
  })

  function update(id: string, next: Partial<DraftCollection>) {
    setDrafts((current) => {
      const base = current[id] ?? { enabled: true, itemsText: '' }
      return { ...current, [id]: { ...base, ...next } }
    })
  }

  function handleSubmit() {
    const collections: SeedCollectionInput[] = []
    for (const suggestion of suggestions) {
      const draft = drafts[suggestion.id]
      const itemsText = draft?.itemsText.trim() ?? ''
      if (!draft?.enabled || !itemsText) continue
      collections.push({
        suggestionId: suggestion.id,
        singularLabel: suggestion.singularLabel,
        pluralLabel: suggestion.pluralLabel,
        itemsText,
      })
    }
    onSubmit(collections)
  }

  return (
    <section className={cn(paddedPanel, 'rounded-[16px]')}>
      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(260px,0.7fr)]">
        <div className="grid gap-5">
          <div className="grid gap-2">
            <p className={text.eyebrow}>One more thing</p>
            <h1 className={text.sectionTitle}>Anything worth listing?</h1>
            <p className={text.p}>
              These become editable collections — each item gets its own entry and page. List what you have now, or skip and add them later.
            </p>
            {prompt ? (
              <p className="rounded-[14px] border border-border bg-[var(--surface-2)] px-4 py-3 text-sm italic text-[var(--paper-muted)]">
                "{prompt}"
              </p>
            ) : null}
          </div>

          <ol className="grid gap-4">
            {suggestions.map((suggestion) => {
              const draft = drafts[suggestion.id] ?? { enabled: true, itemsText: '' }
              const isEnabled = draft.enabled
              return (
                <li
                  key={suggestion.id}
                  className={cn(
                    'grid gap-3 rounded-[14px] border px-4 py-4 transition-[border-color,background]',
                    isEnabled
                      ? 'border-border bg-[var(--surface-1)]'
                      : 'border-dashed border-border bg-[color-mix(in_oklch,var(--surface-1)_94%,transparent)]',
                  )}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="grid gap-1">
                      <p className="flex items-center gap-1.5 text-xs uppercase tracking-[0.12em] text-[var(--paper-muted)]">
                        <Layers className="size-3.5" />
                        Collection
                      </p>
                      <p className="text-sm font-semibold text-[var(--paper)]">
                        {suggestion.pluralLabel}
                      </p>
                      {suggestion.helper ? (
                        <p className="text-xs text-[var(--paper-muted)]">
                          {suggestion.helper}
                        </p>
                      ) : null}
                    </div>
                    <button
                      type="button"
                      className="shrink-0 text-xs font-semibold uppercase tracking-[0.1em] text-[var(--paper-muted)] underline-offset-4 hover:text-[var(--paper)] hover:underline"
                      onClick={() => update(suggestion.id, { enabled: !isEnabled })}
                      disabled={isSubmitting}
                    >
                      {isEnabled ? 'Skip' : 'Include'}
                    </button>
                  </div>

                  {isEnabled ? (
                    <div className="grid gap-1.5">
                      {suggestion.itemHint ? (
                        <p className="text-xs text-[var(--paper-muted)]">{suggestion.itemHint}</p>
                      ) : null}
                      <Textarea
                        value={draft.itemsText}
                        onChange={(event) =>
                          update(suggestion.id, { itemsText: event.target.value })
                        }
                        placeholder={suggestion.example ? `e.g. ${suggestion.example}` : 'One item per line'}
                        rows={4}
                        disabled={isSubmitting}
                      />
                    </div>
                  ) : null}
                </li>
              )
            })}
          </ol>

          <div className={cn(actions.row, 'items-center')}>
            <Button onClick={handleSubmit} disabled={isSubmitting}>
              <Sparkles className="size-4" />
              {isSubmitting ? 'Generating...' : 'Generate site'}
            </Button>
            <button
              type="button"
              className={actions.inlineLink}
              onClick={onSkip}
              disabled={isSubmitting}
            >
              Skip and generate
            </button>
          </div>
        </div>

        <aside className="grid content-start gap-3 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_82%,var(--thread-mauve))] bg-[linear-gradient(180deg,color-mix(in_oklch,var(--surface-2)_92%,transparent),color-mix(in_oklch,var(--surface-1)_96%,transparent))] p-4">
          <p className={text.label}>Why collections</p>
          <p className="text-sm text-[var(--paper-muted)]">
            Rough lines are enough — a name and a price, or just a name. Snaelda structures them into entries you can edit one by one.
          </p>
          <p className="text-xs text-[var(--paper-muted)]">
            Every entry gets its own page on your site, and you can add, remove, or reorder entries any time.
          </p>
        </aside>
      </div>
    </section>
  )
}
