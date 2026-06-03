import { useMemo, useState } from 'react'
import { Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { ClarifyingAnswer, ClarifyingQuestion } from '@/lib/api'
import { actions, paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

type IntakeFormProps = {
  prompt: string
  questions: ClarifyingQuestion[]
  isSubmitting?: boolean
  onSubmit: (answers: ClarifyingAnswer[]) => void
  onSkipAll: () => void
}

type DraftAnswer = {
  selectedOptions: string[]
  text: string
  skipped: boolean
}

function emptyDraft(): DraftAnswer {
  return { selectedOptions: [], text: '', skipped: false }
}

export function GenerationIntakeForm({
  prompt,
  questions,
  isSubmitting = false,
  onSubmit,
  onSkipAll,
}: IntakeFormProps) {
  const initial = useMemo(() => {
    const seed: Record<string, DraftAnswer> = {}
    for (const question of questions) {
      seed[question.id] = emptyDraft()
    }
    return seed
  }, [questions])
  const [drafts, setDrafts] = useState<Record<string, DraftAnswer>>(initial)

  function update(id: string, next: Partial<DraftAnswer>) {
    setDrafts((current) => ({
      ...current,
      [id]: { ...emptyDraft(), ...current[id], ...next, skipped: false },
    }))
  }

  function skip(id: string) {
    setDrafts((current) => ({
      ...current,
      [id]: { ...emptyDraft(), skipped: true },
    }))
  }

  function handleSubmit() {
    const answers: ClarifyingAnswer[] = questions.map((question) => {
      const draft = drafts[question.id] ?? emptyDraft()
      if (draft.skipped) {
        return { questionId: question.id, prompt: question.prompt, skipped: true }
      }
      if (question.kind === 'text') {
        const text = draft.text.trim()
        if (!text) {
          return { questionId: question.id, prompt: question.prompt, skipped: true }
        }
        return { questionId: question.id, prompt: question.prompt, text }
      }
      const selected = draft.selectedOptions.filter(Boolean)
      if (selected.length === 0) {
        return { questionId: question.id, prompt: question.prompt, skipped: true }
      }
      return {
        questionId: question.id,
        prompt: question.prompt,
        selectedOptions: selected,
      }
    })
    onSubmit(answers)
  }

  return (
    <section className={cn(paddedPanel, 'rounded-[16px]')}>
      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(260px,0.7fr)]">
        <div className="grid gap-5">
          <div className="grid gap-2">
            <p className={text.eyebrow}>Quick check</p>
            <h1 className={text.sectionTitle}>A couple of questions before we draft</h1>
            <p className={text.p}>
              Your answers shape the structure and copy. Skip anything that does not apply — we will make a reasonable call.
            </p>
            {prompt ? (
              <p className="rounded-[14px] border border-border bg-[var(--surface-2)] px-4 py-3 text-sm italic text-[var(--paper-muted)]">
                "{prompt}"
              </p>
            ) : null}
          </div>

          <ol className="grid gap-4">
            {questions.map((question, index) => {
              const draft = drafts[question.id] ?? emptyDraft()
              const isSkipped = draft.skipped
              return (
                <li
                  key={question.id}
                  className={cn(
                    'grid gap-3 rounded-[14px] border px-4 py-4 transition-[border-color,background]',
                    isSkipped
                      ? 'border-dashed border-border bg-[color-mix(in_oklch,var(--surface-1)_94%,transparent)]'
                      : 'border-border bg-[var(--surface-1)]',
                  )}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="grid gap-1">
                      <p className="text-xs uppercase tracking-[0.12em] text-[var(--paper-muted)]">
                        Question {index + 1}
                      </p>
                      <p className="text-sm font-semibold text-[var(--paper)]">
                        {question.prompt}
                      </p>
                      {question.helper ? (
                        <p className="text-xs text-[var(--paper-muted)]">
                          {question.helper}
                        </p>
                      ) : null}
                    </div>
                    <button
                      type="button"
                      className="shrink-0 text-xs font-semibold uppercase tracking-[0.1em] text-[var(--paper-muted)] underline-offset-4 hover:text-[var(--paper)] hover:underline"
                      onClick={() => skip(question.id)}
                      disabled={isSubmitting || isSkipped}
                    >
                      Skip
                    </button>
                  </div>

                  {question.kind === 'text' ? (
                    <Input
                      value={draft.text}
                      onChange={(event) =>
                        update(question.id, { text: event.target.value })
                      }
                      placeholder="Type a short answer"
                      disabled={isSubmitting}
                    />
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      {(question.options ?? []).map((option) => {
                        const isSelected = draft.selectedOptions.includes(option)
                        return (
                          <button
                            key={option}
                            type="button"
                            disabled={isSubmitting}
                            onClick={() => {
                              if (question.kind === 'multi') {
                                const next = isSelected
                                  ? draft.selectedOptions.filter((item) => item !== option)
                                  : [...draft.selectedOptions, option]
                                update(question.id, { selectedOptions: next })
                                return
                              }
                              update(question.id, {
                                selectedOptions: isSelected ? [] : [option],
                              })
                            }}
                            className={cn(
                              'rounded-full border px-3.5 py-1.5 text-sm font-semibold transition-[background,border-color,transform] hover:-translate-y-px',
                              isSelected
                                ? 'border-[var(--thread-gold)] bg-[color-mix(in_oklch,var(--thread-gold)_14%,var(--surface-2))] text-[var(--paper)]'
                                : 'border-border bg-[var(--surface-2)] text-[var(--paper-muted)] hover:border-[var(--thread-teal)] hover:text-[var(--paper)]',
                              isSubmitting && 'opacity-60',
                            )}
                          >
                            {option}
                          </button>
                        )
                      })}
                    </div>
                  )}
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
              onClick={onSkipAll}
              disabled={isSubmitting}
            >
              Skip all and generate
            </button>
          </div>
        </div>

        <aside className="grid content-start gap-3 rounded-[18px] border border-[color-mix(in_oklch,var(--border)_82%,var(--thread-mauve))] bg-[linear-gradient(180deg,color-mix(in_oklch,var(--surface-2)_92%,transparent),color-mix(in_oklch,var(--surface-1)_96%,transparent))] p-4">
          <p className={text.label}>Why we ask</p>
          <p className="text-sm text-[var(--paper-muted)]">
            A few well-chosen answers help Snaelda pick the right pages, blocks, and tone for your draft instead of guessing.
          </p>
          <p className="text-xs text-[var(--paper-muted)]">
            Once the draft lands, you can keep refining it with AI or edit any block individually.
          </p>
        </aside>
      </div>
    </section>
  )
}
