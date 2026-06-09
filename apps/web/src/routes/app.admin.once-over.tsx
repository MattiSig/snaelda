import { createFileRoute } from '@tanstack/react-router'
import { ArrowUpRight, CheckCircle2, Loader2, Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  APIError,
  deliverOnceOver,
  getCurrentSession,
  listPendingOnceOvers,
  type BuilderSession,
  type PendingOnceOverRequest,
} from '@/lib/api'
import { actions, emptyState, form, paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/app/admin/once-over')({
  component: OnceOverQueuePage,
})

type DraftDelivery = {
  videoUrl: string
  deliveryNextSteps: string[]
}

const defaultNextSteps = ['', '', '']

function OnceOverQueuePage() {
  const [session, setSession] = useState<BuilderSession | null>(null)
  const [requests, setRequests] = useState<PendingOnceOverRequest[]>([])
  const [drafts, setDrafts] = useState<Record<string, DraftDelivery>>({})
  const [isLoading, setIsLoading] = useState(true)
  const [savingRequestId, setSavingRequestId] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [statusMessage, setStatusMessage] = useState('')

  useEffect(() => {
    let isMounted = true

    getCurrentSession()
      .then((nextSession) => {
        if (!isMounted) {
          return
        }
        setSession(nextSession)
        if (!nextSession.isOperator) {
          setErrorMessage('Operator access is required to view the Once-over queue.')
          setIsLoading(false)
          return
        }
        return listPendingOnceOvers()
          .then((response) => {
            if (!isMounted) {
              return
            }
            setRequests(response.requests)
            setDrafts(buildDrafts(response.requests))
            setIsLoading(false)
          })
          .catch((error) => {
            if (!isMounted) {
              return
            }
            setErrorMessage(
              error instanceof APIError
                ? error.message
                : 'Could not load the Once-over queue.',
            )
            setIsLoading(false)
          })
      })
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setErrorMessage(
          error instanceof APIError ? error.message : 'Could not load your session.',
        )
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [])

  function updateDraft(requestId: string, nextDraft: DraftDelivery) {
    setDrafts((current) => ({
      ...current,
      [requestId]: nextDraft,
    }))
  }

  function handleStepChange(requestId: string, index: number, value: string) {
    const currentDraft = drafts[requestId] ?? {
      videoUrl: '',
      deliveryNextSteps: [...defaultNextSteps],
    }
    const nextSteps = [...currentDraft.deliveryNextSteps]
    nextSteps[index] = value
    updateDraft(requestId, {
      ...currentDraft,
      deliveryNextSteps: nextSteps,
    })
  }

  function handleAddStep(requestId: string) {
    const currentDraft = drafts[requestId] ?? {
      videoUrl: '',
      deliveryNextSteps: [...defaultNextSteps],
    }
    if (currentDraft.deliveryNextSteps.length >= 5) {
      return
    }
    updateDraft(requestId, {
      ...currentDraft,
      deliveryNextSteps: [...currentDraft.deliveryNextSteps, ''],
    })
  }

  function handleRemoveStep(requestId: string, index: number) {
    const currentDraft = drafts[requestId]
    if (!currentDraft || currentDraft.deliveryNextSteps.length <= 1) {
      return
    }
    updateDraft(requestId, {
      ...currentDraft,
      deliveryNextSteps: currentDraft.deliveryNextSteps.filter((_, itemIndex) => itemIndex !== index),
    })
  }

  async function handleDeliver(request: PendingOnceOverRequest) {
    const draft = drafts[request.id] ?? {
      videoUrl: '',
      deliveryNextSteps: [...defaultNextSteps],
    }
    setSavingRequestId(request.id)
    setErrorMessage('')
    setStatusMessage('')

    try {
      await deliverOnceOver(request.id, {
        videoUrl: draft.videoUrl,
        deliveryNextSteps: draft.deliveryNextSteps.filter((step) => step.trim().length > 0),
      })
      setRequests((current) => current.filter((item) => item.id !== request.id))
      setDrafts((current) => {
        const next = { ...current }
        delete next[request.id]
        return next
      })
      setStatusMessage(`Delivered the Once-over for ${request.workspaceName}.`)
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not deliver this Once-over.',
      )
    } finally {
      setSavingRequestId('')
    }
  }

  if (isLoading) {
    return (
      <section className={cn(paddedPanel, 'rounded-[14px]')}>
        <p className={text.p}>Loading the Once-over queue…</p>
      </section>
    )
  }

  if (!session?.isOperator) {
    return (
      <section className={cn(paddedPanel, 'rounded-[14px]')}>
        <p className={text.error}>{errorMessage || 'Operator access is required.'}</p>
      </section>
    )
  }

  return (
    <div className="grid gap-4">
      <section className={cn(paddedPanel, 'rounded-[14px]')}>
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="grid gap-2">
            <p className={text.eyebrow}>Operator queue</p>
            <h1 className={text.sectionTitle}>Once-over requests ready for a human pass</h1>
            <p className={text.p}>
              Work from oldest ready request to newest. Each card keeps the intake,
              delivery link, and next-step handoff in one place.
            </p>
          </div>
          <div className="rounded-full border border-border bg-[var(--surface-2)] px-4 py-2 text-sm font-bold text-[var(--paper)]">
            {requests.length} pending
          </div>
        </div>

        {errorMessage ? <p className={cn(text.error, 'mt-4')}>{errorMessage}</p> : null}
        {statusMessage ? <p className={cn(text.success, 'mt-4')}>{statusMessage}</p> : null}
      </section>

      {requests.length === 0 ? (
        <section className={emptyState}>
          <p className={text.sectionTitle}>No pending Once-overs</p>
          <p className={cn(text.p, 'mt-2')}>
            Everything marked ready for review has already been delivered.
          </p>
        </section>
      ) : (
        <div className="grid gap-4">
          {requests.map((request) => {
            const draft = drafts[request.id] ?? {
              videoUrl: '',
              deliveryNextSteps: [...defaultNextSteps],
            }
            const isSaving = savingRequestId === request.id
            const ownerLabel =
              request.ownerName && request.ownerEmail
                ? `${request.ownerName} · ${request.ownerEmail}`
                : request.ownerName || request.ownerEmail || 'No billing contact on file'
            return (
              <article
                key={request.id}
                className="grid gap-5 rounded-[16px] border border-border bg-[var(--surface-1)] p-5 lg:grid-cols-[minmax(0,0.92fr)_minmax(320px,1.08fr)]"
              >
                <div className="grid gap-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="grid gap-1">
                      <h2 className="text-[1.2rem] font-black text-[var(--paper)]">
                        {request.workspaceName}
                      </h2>
                      <p className="text-sm text-[var(--paper-muted)]">
                        {ownerLabel}
                      </p>
                    </div>
                    <div className="rounded-full border border-border bg-[var(--surface-2)] px-3 py-1 text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper)]">
                      Ready {formatDate(request.intakeSubmittedAt)}
                    </div>
                  </div>

                  <div className="grid gap-3 sm:grid-cols-2">
                    <MetaCard label="Purchased" value={formatDate(request.paidAt)} />
                    <MetaCard label="Outcome" value={request.intakeOutcome} />
                  </div>

                  <IntakeCard label="Business" value={request.intakeBusiness} />
                  <IntakeCard label="Visitor" value={request.intakeVisitor} />
                  {request.intakeStuckOn ? (
                    <IntakeCard label="What still feels stuck" value={request.intakeStuckOn} />
                  ) : null}
                </div>

                <div className="grid gap-3 rounded-[16px] border border-border bg-[var(--surface-2)] p-4">
                  <div>
                    <p className={text.label}>Delivery</p>
                    <p className="mt-2 text-sm text-[var(--paper-muted)]">
                      Add the walkthrough link and the concrete next steps the customer owns.
                    </p>
                  </div>

                  <div className={form.field}>
                    <label htmlFor={`video-url-${request.id}`} className={text.label}>
                      Walkthrough URL
                    </label>
                    <Input
                      id={`video-url-${request.id}`}
                      type="url"
                      value={draft.videoUrl}
                      onChange={(event) =>
                        updateDraft(request.id, {
                          ...draft,
                          videoUrl: event.target.value,
                        })
                      }
                      placeholder="https://loom.com/share/..."
                      disabled={isSaving}
                    />
                  </div>

                  <div className="grid gap-3">
                    <div className="flex items-center justify-between gap-3">
                      <p className={text.label}>Next steps</p>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => handleAddStep(request.id)}
                        disabled={isSaving || draft.deliveryNextSteps.length >= 5}
                      >
                        <Plus className="size-4" />
                        Add step
                      </Button>
                    </div>
                    {draft.deliveryNextSteps.map((step, index) => (
                      <div key={`${request.id}-step-${index}`} className="grid gap-2">
                        <div className="flex items-center justify-between gap-3">
                          <label htmlFor={`${request.id}-step-${index}`} className={text.label}>
                            Step {index + 1}
                          </label>
                          {draft.deliveryNextSteps.length > 1 ? (
                            <button
                              type="button"
                              className="inline-flex items-center gap-1 text-xs font-bold uppercase tracking-[0.08em] text-[var(--paper-muted)] transition-colors hover:text-[var(--paper)]"
                              onClick={() => handleRemoveStep(request.id, index)}
                              disabled={isSaving}
                            >
                              <Trash2 className="size-3.5" />
                              Remove
                            </button>
                          ) : null}
                        </div>
                        <Textarea
                          id={`${request.id}-step-${index}`}
                          rows={2}
                          value={step}
                          onChange={(event) => handleStepChange(request.id, index, event.target.value)}
                          placeholder="Replace the last placeholder photo with your own product shot."
                          disabled={isSaving}
                        />
                      </div>
                    ))}
                  </div>

                  <div className={actions.rowLarge}>
                    <Button
                      type="button"
                      onClick={() => handleDeliver(request)}
                      disabled={isSaving}
                    >
                      {isSaving ? (
                        <Loader2 className="size-4 animate-spin" />
                      ) : (
                        <CheckCircle2 className="size-4" />
                      )}
                      {isSaving ? 'Sending delivery…' : 'Deliver Once-over'}
                    </Button>
                    {draft.videoUrl ? (
                      <a
                        className="inline-flex min-h-11 items-center gap-2 rounded-full border border-border bg-[var(--surface-1)] px-4 py-2 text-sm font-bold text-[var(--paper)] transition-[background,border-color,transform] hover:-translate-y-px hover:border-[var(--thread-teal)] hover:bg-[var(--surface-3)]"
                        href={draft.videoUrl}
                        target="_blank"
                        rel="noreferrer"
                      >
                        Open walkthrough
                        <ArrowUpRight className="size-4" />
                      </a>
                    ) : null}
                  </div>
                </div>
              </article>
            )
          })}
        </div>
      )}
    </div>
  )
}

function buildDrafts(requests: PendingOnceOverRequest[]) {
  return requests.reduce<Record<string, DraftDelivery>>((acc, request) => {
    acc[request.id] = {
      videoUrl: '',
      deliveryNextSteps: [...defaultNextSteps],
    }
    return acc
  }, {})
}

function MetaCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[12px] border border-border bg-[var(--surface-2)] p-4">
      <p className={text.label}>{label}</p>
      <p className="mt-2 text-sm font-semibold text-[var(--paper)]">{value}</p>
    </div>
  )
}

function IntakeCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[14px] border border-border bg-[var(--surface-2)] p-4">
      <p className={text.label}>{label}</p>
      <p className="mt-2 text-sm leading-6 text-[var(--paper)]">{value}</p>
    </div>
  )
}

function formatDate(value: string) {
  return new Date(value).toLocaleDateString()
}
