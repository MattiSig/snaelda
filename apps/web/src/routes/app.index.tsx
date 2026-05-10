import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  APIError,
  createSite,
  generateSite,
  listSites,
  type SiteSummary,
} from '@/lib/api'
import { actions, emptyState, form, layout, ribbonPanel, siteCard, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/app/')({
  component: SitesIndex,
})

function SitesIndex() {
  const navigate = useNavigate()
  const [sites, setSites] = useState<SiteSummary[]>([])
  const [name, setName] = useState('')
  const [prompt, setPrompt] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

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

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSubmitting(true)
    setErrorMessage('')

    try {
      const isGenerated = prompt.trim() !== ''
      const response = isGenerated
        ? await generateSite({ name, prompt })
        : await createSite({ name, prompt })
      await navigate({
        to: isGenerated ? '/app/sites/$siteId/preview' : '/app/sites/$siteId',
        params: { siteId: response.draft.site.id },
      })
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not create site',
      )
      setIsSubmitting(false)
    }
  }

  return (
    <div className={layout.workspaceGrid}>
      <section className={ribbonPanel}>
        <div className="mb-5">
          <p className={text.eyebrow}>Create website</p>
          <h1 className={cn(text.h1, 'max-w-[10ch]')}>Start with the shape of the site.</h1>
          <p className={text.p}>
            Add the business name and a short brief. Snaelda will create a
            structured draft you can tune before publishing.
          </p>
        </div>

        <form className={form.grid} onSubmit={handleSubmit}>
          <label htmlFor="site-name" className={text.label}>Business name</label>
          <Input
            id="site-name"
            name="name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="Moss & Thread Studio"
            required
          />

          <label htmlFor="site-prompt" className={text.label}>Brief</label>
          <Textarea
            id="site-prompt"
            name="prompt"
            rows={6}
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            placeholder="A calm one-page site for a textile studio that runs workshops and custom commissions."
          />

          {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}

          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting
              ? prompt.trim() !== ''
                ? 'Generating draft...'
                : 'Creating draft...'
              : prompt.trim() !== ''
                ? 'Generate draft'
                : 'Create starter draft'}
          </Button>
        </form>
      </section>

      <section className={ribbonPanel}>
        <div className="mb-5 flex items-end justify-between gap-4 max-sm:block">
          <p className={text.eyebrow}>Workspace sites</p>
          <h2 className={text.h2}>Recent drafts</h2>
        </div>

        {isLoading ? (
          <p className={text.p}>Loading drafts...</p>
        ) : sites.length === 0 ? (
          <div className={emptyState}>
            <p className={text.p}>No drafts yet. Create one to open the builder.</p>
          </div>
        ) : (
          <div className={siteCard.list}>
            {sites.map((site) => (
              <article key={site.id} className={siteCard.card}>
                <div>
                  <p className={text.eyebrow}>{site.status}</p>
                  <h3 className={cn(text.h3, 'mb-2')}>{site.name}</h3>
                  <p className={text.p}>{site.slug}.local</p>
                </div>
                <dl className={siteCard.meta}>
                  <div className="rounded-[14px] border border-border bg-[var(--surface-1)] p-4">
                    <dt className={text.eyebrow}>Pages</dt>
                    <dd className="mt-1.5 text-[1.35rem] font-black text-[var(--paper)]">{site.pageCount}</dd>
                  </div>
                  <div className="rounded-[14px] border border-border bg-[var(--surface-1)] p-4">
                    <dt className={text.eyebrow}>Locale</dt>
                    <dd className="mt-1.5 text-[1.35rem] font-black text-[var(--paper)]">{site.defaultLocale}</dd>
                  </div>
                </dl>
                <div className={siteCard.actions}>
                  <Button asChild variant="plain" className={actions.inlineLink}>
                    <Link to="/app/sites/$siteId" params={{ siteId: site.id }}>
                      Open builder
                    </Link>
                  </Button>
                  <Button asChild variant="plain" className={actions.inlineLink}>
                    <Link
                      to="/app/sites/$siteId/preview"
                      params={{ siteId: site.id }}
                    >
                      Preview
                    </Link>
                  </Button>
                  {site.publishedVersionId ? (
                    <Button asChild variant="plain" className={actions.inlineLink}>
                      <Link
                        to="/public/$siteSlug"
                        params={{ siteSlug: site.slug }}
                      >
                        Live site
                      </Link>
                    </Button>
                  ) : null}
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
