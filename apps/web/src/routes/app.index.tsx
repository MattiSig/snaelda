import { Link, createFileRoute, useNavigate, useRouterState } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useEffect, useRef, useState } from 'react'
import {
  GenerationProgressCard,
  type GenerationProgressItem,
  type GenerationShadowPage,
} from '@/components/GenerationProgressCard'
import { GenerationIntakeForm } from '@/components/GenerationIntakeForm'
import { Ellipsis, PencilLine, Settings, Sparkles, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  APIError,
  createSite,
  fetchClarifyingQuestions,
  listSites,
  streamGenerateSite,
  type ClarifyingAnswer,
  type ClarifyingQuestion,
  type GenerationPartialEvent,
  type SiteSummary,
} from '@/lib/api'
import { actions, emptyState, form, paddedPanel, ribbonPanel, siteCard, text } from '@/lib/styles'
import { cn } from '@/lib/utils'
import { useLocale } from '@/lib/locale'

export const Route = createFileRoute('/app/')({
  component: SitesIndex,
})

const generationSteps: GenerationProgressItem[] = [
  { step: 'prompt.normalize', label: 'Reading your prompt' },
  { step: 'plan.pages', label: 'Planning pages and structure' },
  { step: 'plan.theme', label: 'Picking colors and typography' },
  { step: 'plan.blocks', label: 'Choosing blocks for each page' },
  { step: 'assets.fetch', label: 'Finding starter imagery' },
  { step: 'copy.write', label: 'Writing copy' },
  { step: 'validate.repair', label: 'Checking and repairing' },
  { step: 'persist', label: 'Saving your draft' },
]

function reduceShadowPages(
  current: GenerationShadowPage[],
  partial: GenerationPartialEvent,
): GenerationShadowPage[] {
  if (partial.kind === 'outline') {
    const outline = partial.payload as {
      pages?: Array<{ title?: string; slug?: string; goal?: string }>
    }
    if (!outline.pages) {
      return current
    }
    return outline.pages.map((page) => ({
      title: page.title ?? 'Page',
      slug: page.slug ?? '',
      goal: page.goal,
      blocks: [],
    }))
  }
  if (partial.kind === 'page-content') {
    const slug = partial.pageSlug ?? ''
    const page = partial.payload as {
      blocks?: Array<{ type: string }>
    }
    const blocks = (page.blocks ?? []).map((block) => ({
      type: block.type,
      hasCopy: true,
    }))
    return current.map((p) => (p.slug === slug ? { ...p, blocks } : p))
  }
  return current
}

function SitesIndex() {
  const navigate = useNavigate()
  const locale = useLocale()
  const locationSearch = useRouterState({ select: (state) => state.location.search })
  const routeSearch = locationSearch as Record<string, unknown>
  const promptFromUrl =
    typeof routeSearch.prompt === 'string' ? routeSearch.prompt : ''
  const [sites, setSites] = useState<SiteSummary[]>([])
  const [name, setName] = useState('')
  const [prompt, setPrompt] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [openActionsSiteId, setOpenActionsSiteId] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [isAutoGenerating, setIsAutoGenerating] = useState(Boolean(promptFromUrl))
  const [generationPrompt, setGenerationPrompt] = useState('')
  const [generationStep, setGenerationStep] = useState('')
  const [generationStepTotal, setGenerationStepTotal] = useState(0)
  const [intakeQuestions, setIntakeQuestions] = useState<ClarifyingQuestion[] | null>(null)
  const [isPreparingIntake, setIsPreparingIntake] = useState(false)
  const [shadowPages, setShadowPages] = useState<GenerationShadowPage[]>([])
  const intakePromptRef = useRef('')
  const intakeNameRef = useRef('')
  const hasAutoSubmitted = useRef(false)
  const actionsMenuRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    let isMounted = true

    listSites()
      .then((response) => {
        if (isMounted) {
          setSites(response.sites)
          setIsLoading(false)
        }
      })
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setErrorMessage(
          error instanceof APIError ? error.message : 'Could not load sites',
        )
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [])

  useEffect(() => {
    if (!promptFromUrl || hasAutoSubmitted.current || isLoading) {
      return
    }
    hasAutoSubmitted.current = true
    setPrompt(promptFromUrl)
    setName('')
    const timer = setTimeout(() => {
      void startIntake({ promptValue: promptFromUrl, nameValue: '' })
    }, 300)
    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [promptFromUrl, isLoading])

  useEffect(() => {
    if (!isCreateOpen) {
      return
    }

    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape' && !isSubmitting) {
        setIsCreateOpen(false)
      }
    }

    window.addEventListener('keydown', handleEscape)
    return () => window.removeEventListener('keydown', handleEscape)
  }, [isCreateOpen, isSubmitting])

  useEffect(() => {
    if (!openActionsSiteId) {
      return
    }

    function handlePointerDown(event: PointerEvent) {
      if (!actionsMenuRef.current?.contains(event.target as Node)) {
        setOpenActionsSiteId('')
      }
    }

    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpenActionsSiteId('')
      }
    }

    window.addEventListener('pointerdown', handlePointerDown)
    window.addEventListener('keydown', handleEscape)

    return () => {
      window.removeEventListener('pointerdown', handlePointerDown)
      window.removeEventListener('keydown', handleEscape)
    }
  }, [openActionsSiteId])

  async function startIntake({
    promptValue,
    nameValue,
  }: {
    promptValue: string
    nameValue: string
  }) {
    const trimmedPrompt = promptValue.trim()
    if (!trimmedPrompt) {
      return
    }
    setErrorMessage('')
    intakePromptRef.current = trimmedPrompt
    intakeNameRef.current = nameValue
    setGenerationPrompt(trimmedPrompt)
    setIsPreparingIntake(true)
    setIntakeQuestions(null)
    try {
      const response = await fetchClarifyingQuestions({
        name: nameValue.trim() || undefined,
        prompt: trimmedPrompt,
        preferredLanguage: locale,
      })
      if (!response.questions || response.questions.length === 0) {
        await runGeneration({
          promptValue: trimmedPrompt,
          nameValue,
          interviewAnswers: [],
        })
        return
      }
      setIntakeQuestions(response.questions)
      setIsPreparingIntake(false)
      setIsCreateOpen(false)
    } catch (error) {
      setIsPreparingIntake(false)
      setIsAutoGenerating(false)
      setGenerationPrompt('')
      if (!(error instanceof APIError) || error.status >= 500) {
        // Intake is optional. Transient failures should not prevent generation.
        await runGeneration({
          promptValue: trimmedPrompt,
          nameValue,
          interviewAnswers: [],
        })
        return
      }
      setErrorMessage(error.message)
    }
  }

  async function runGeneration({
    promptValue,
    nameValue,
    interviewAnswers,
  }: {
    promptValue: string
    nameValue: string
    interviewAnswers: ClarifyingAnswer[]
  }) {
    const trimmedPrompt = promptValue.trim()
    if (!trimmedPrompt) {
      return
    }
    setIntakeQuestions(null)
    setIsPreparingIntake(false)
    setIsSubmitting(true)
    setGenerationPrompt(trimmedPrompt)
    setGenerationStep('')
    setGenerationStepTotal(0)
    setShadowPages([])
    setErrorMessage('')
    try {
      const response = await streamGenerateSite(
        {
          name: nameValue,
          prompt: trimmedPrompt,
          preferredLanguage: locale,
          interviewAnswers: interviewAnswers.length > 0 ? interviewAnswers : undefined,
        },
        {
          onProgress: (step) => {
            setGenerationStep(step.step)
            setGenerationStepTotal(step.total)
          },
          onPartial: (partial) => {
            setShadowPages((current) => reduceShadowPages(current, partial))
          },
        },
      )
      setIsCreateOpen(false)
      await navigate({
        to: '/app/sites/$siteId/preview',
        params: { siteId: response.siteId },
      })
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not generate site',
      )
      setIsSubmitting(false)
      setIsAutoGenerating(false)
      setGenerationPrompt('')
      setGenerationStep('')
      setGenerationStepTotal(0)
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setErrorMessage('')

    const isGenerated = prompt.trim() !== ''
    if (isGenerated) {
      await startIntake({ promptValue: prompt, nameValue: name })
      return
    }
    setIsSubmitting(true)
    try {
      const response = await createSite({ name, prompt })
      setIsCreateOpen(false)
      await navigate({
        to: '/app/sites/$siteId',
        params: { siteId: response.draft.site.id },
        search: { panel: undefined },
      })
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not create site',
      )
      setIsSubmitting(false)
    }
  }

  if (intakeQuestions && intakeQuestions.length > 0 && !errorMessage) {
    return (
      <GenerationIntakeForm
        prompt={generationPrompt || intakePromptRef.current}
        questions={intakeQuestions}
        isSubmitting={isSubmitting}
        onSubmit={(answers) =>
          runGeneration({
            promptValue: intakePromptRef.current,
            nameValue: intakeNameRef.current,
            interviewAnswers: answers,
          })
        }
        onSkipAll={() =>
          runGeneration({
            promptValue: intakePromptRef.current,
            nameValue: intakeNameRef.current,
            interviewAnswers: [],
          })
        }
      />
    )
  }

  if (
    (isPreparingIntake || isAutoGenerating || (isSubmitting && generationPrompt)) &&
    !errorMessage
  ) {
    const isPreparing = isPreparingIntake && !isSubmitting
    return (
      <GenerationProgressCard
        eyebrow="New site"
        title={isPreparing ? 'Reading your prompt...' : 'Weaving your draft...'}
        description={
          isPreparing
            ? 'Snaelda is figuring out whether a quick question or two would sharpen the result.'
            : 'Snaelda is generating your first draft. This usually takes about a minute.'
        }
        prompt={generationPrompt || promptFromUrl}
        steps={generationSteps}
        activeStep={isPreparing ? 'prompt.normalize' : generationStep}
        activeTotal={generationStepTotal}
        showSkeleton={
          !isPreparing &&
          (generationStep === 'plan.blocks' ||
            generationStep === 'assets.fetch' ||
            generationStep === 'copy.write' ||
            generationStep === 'validate.repair' ||
            generationStep === 'persist')
        }
        shadowPages={isPreparing ? undefined : shadowPages}
      />
    )
  }

  return (
    <>
      <section className={cn(paddedPanel, 'rounded-[12px]')}>
        <div className="mb-5 flex items-start justify-between gap-4 max-sm:grid">
          <div className="grid gap-2">
            <p className={text.eyebrow}>Sites</p>
            <h1 className={text.sectionTitle}>Your sites</h1>
            <p className={text.p}>
              Open a draft, preview the public version, or start a new site from a short prompt.
            </p>
          </div>
          <Button
            type="button"
            className="min-w-[168px] justify-center"
            onClick={() => {
              setErrorMessage('')
              setIsCreateOpen(true)
            }}
          >
            <Sparkles className="size-4" />
            New site
          </Button>
        </div>

        {errorMessage && !isCreateOpen ? (
          <div className="mb-5 rounded-[12px] border border-[color-mix(in_oklch,var(--thread-gold)_60%,var(--border))] bg-[color-mix(in_oklch,var(--thread-gold)_10%,var(--surface-1))] p-4">
            <p className="m-0 text-sm font-bold text-[var(--paper)]">Could not start a new site</p>
            <p className="mt-2 m-0 text-sm text-[var(--paper-muted)]">{errorMessage}</p>
            <div className="mt-3 flex flex-wrap gap-3">
              <Button asChild type="button" size="sm" variant="outline">
                <Link to="/app/billing">Manage plan</Link>
              </Button>
              <Button
                type="button"
                size="sm"
                variant="plain"
                className={actions.inlineLink}
                onClick={() => setErrorMessage('')}
              >
                Dismiss
              </Button>
            </div>
          </div>
        ) : null}

        {isLoading ? (
          <p className={text.p}>Loading drafts...</p>
        ) : sites.length === 0 ? (
          <div className={emptyState}>
            <p className={text.p}>No sites yet. Start one from the builder prompt.</p>
          </div>
        ) : (
          <div className="overflow-hidden rounded-[10px] border border-border bg-[var(--surface-2)]">
            <div className={siteCard.list}>
            {sites.map((site) => (
              <article
                key={site.id}
                className={cn(
                  siteCard.card,
                  'grid-cols-[minmax(0,1.2fr)_minmax(220px,0.7fr)_auto] items-center gap-4 border-b border-border px-5 py-4 last:border-b-0 max-lg:grid max-lg:gap-3',
                )}
              >
                <div className="min-w-0">
                  <h3 className="truncate text-[1.02rem] font-bold text-[var(--paper)]">
                    {site.name}
                  </h3>
                  <p className="mt-1 truncate text-sm text-[var(--paper-muted)]">
                    {site.slug}.local
                  </p>
                </div>

                <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-[var(--paper-muted)]">
                  <span className="inline-flex min-h-7 items-center rounded-[999px] border border-[color-mix(in_oklch,var(--thread-teal)_52%,var(--border))] bg-[color-mix(in_oklch,var(--thread-teal)_12%,var(--surface-1))] px-2.5 py-1 text-[0.68rem] font-bold uppercase tracking-[0.08em] text-[var(--paper)]">
                    {site.status}
                  </span>
                  <span className="flex items-center gap-1.5">
                    <span className={text.eyebrow}>Pages</span>
                    <span className="font-semibold text-[var(--paper)] tabular-nums">
                      {site.pageCount}
                    </span>
                  </span>
                  <Button asChild variant="plain" className="text-sm font-semibold text-[var(--thread-mauve)] hover:text-[var(--paper)]">
                    <Link
                      to="/app/sites/$siteId"
                      params={{ siteId: site.id }}
                      search={{ panel: 'theme' }}
                    >
                      Theme
                    </Link>
                  </Button>
                </div>

                <div className="flex flex-wrap items-center justify-end gap-2 max-lg:justify-start">
                  <Button asChild variant="outline" size="icon" className="size-10 rounded-[10px]" aria-label={`Open site settings for ${site.name}`}>
                    <Link
                      to="/app/sites/$siteId"
                      params={{ siteId: site.id }}
                      search={{ panel: 'settings' }}
                    >
                      <Settings className="size-4" />
                    </Link>
                  </Button>
                  <Button asChild variant="plain" className={actions.inlineLink}>
                    <Link
                      to="/app/sites/$siteId"
                      params={{ siteId: site.id }}
                      search={{ panel: undefined }}
                    >
                      <PencilLine className="size-4" />
                      Edit
                    </Link>
                  </Button>
                  <div ref={openActionsSiteId === site.id ? actionsMenuRef : null} className="relative">
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      className="size-10 rounded-[10px]"
                      aria-label={`More actions for ${site.name}`}
                      aria-haspopup="menu"
                      aria-expanded={openActionsSiteId === site.id}
                      onClick={() =>
                        setOpenActionsSiteId((current) =>
                          current === site.id ? '' : site.id,
                        )
                      }
                    >
                      <Ellipsis className="size-4" />
                    </Button>
                    {openActionsSiteId === site.id ? (
                      <div
                        role="menu"
                        className="absolute right-0 top-[calc(100%+10px)] z-20 grid min-w-[180px] gap-1 rounded-[10px] border border-border bg-[var(--surface-1)] p-2 shadow-[var(--shadow-soft)]"
                      >
                        <Button asChild variant="plain" className="rounded-[8px] px-3 py-2 text-sm font-bold text-[var(--paper-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]">
                          <Link
                            to="/app/sites/$siteId/preview"
                            params={{ siteId: site.id }}
                            onClick={() => setOpenActionsSiteId('')}
                          >
                            Preview
                          </Link>
                        </Button>
                        {site.publishedVersionId ? (
                          <Button asChild variant="plain" className="rounded-[8px] px-3 py-2 text-sm font-bold text-[var(--paper-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]">
                            <Link
                              to="/public/$siteSlug"
                              params={{ siteSlug: site.slug }}
                              onClick={() => setOpenActionsSiteId('')}
                            >
                              Open live site
                            </Link>
                          </Button>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                </div>
              </article>
            ))}
            </div>
          </div>
        )}
      </section>
      {isCreateOpen ? (
        <div
          className="fixed inset-0 z-50 grid place-items-center bg-[color-mix(in_oklch,var(--background)_58%,transparent)] p-4 backdrop-blur-[10px]"
          onClick={() => {
            if (!isSubmitting) {
              setIsCreateOpen(false)
            }
          }}
        >
          <section
            className={cn(ribbonPanel, 'w-full max-w-[720px] rounded-[12px] shadow-[0_28px_100px_oklch(7%_0.03_340_/_0.5)]')}
            onClick={(event) => event.stopPropagation()}
          >
            <div className="mb-6 flex items-start justify-between gap-4">
              <div className="grid gap-3">
                <p className={text.eyebrow}>New site</p>
                <h2 className={text.appTitle}>Create a site, then shape the first draft.</h2>
                <p className={text.sectionLead}>
                  Add a business name. Include a short prompt if you want Snaelda to generate the first draft for you.
                </p>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label="Close new site dialog"
                className="shrink-0"
                onClick={() => setIsCreateOpen(false)}
                disabled={isSubmitting}
              >
                <X className="size-4" />
              </Button>
            </div>

            <form className="grid gap-4" onSubmit={handleSubmit}>
              <label htmlFor="site-name" className={text.label}>Business name</label>
              <Input
                id="site-name"
                name="name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Moss & Thread Studio"
                required
              />

              <div className="grid gap-2">
                <label htmlFor="site-prompt" className={text.label}>Prompt</label>
                <p className={form.hint}>
                  Leave this blank to start from an empty draft.
                </p>
              </div>
              <Textarea
                id="site-prompt"
                name="prompt"
                rows={6}
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder="A calm one-page site for a textile studio that runs workshops and custom commissions."
              />

              {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}

              <div className="flex flex-wrap items-center justify-between gap-3 pt-2">
                <p className="text-sm text-[var(--paper-muted)]">
                  Draft first, builder next, publish when it looks right.
                </p>
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setIsCreateOpen(false)}
                    disabled={isSubmitting}
                  >
                    Cancel
                  </Button>
                  <Button type="submit" disabled={isSubmitting}>
                    {isSubmitting
                      ? prompt.trim() !== ''
                        ? 'Generating site draft...'
                        : 'Creating blank draft...'
                      : prompt.trim() !== ''
                        ? 'Generate site draft'
                        : 'Create blank draft'}
                  </Button>
                </div>
              </div>
            </form>
          </section>
        </div>
      ) : null}
    </>
  )
}
