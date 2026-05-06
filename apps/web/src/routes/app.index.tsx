import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  APIError,
  createSite,
  generateSite,
  listSites,
  type SiteSummary,
} from '@/lib/api'

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
    <div className="workspace-grid">
      <section className="builder-panel ribbon-panel">
        <div className="panel-heading">
          <p className="eyebrow">Create website</p>
          <h1>Spin up a generated draft, then tune it in the builder.</h1>
          <p>
            Add a name and brief to generate a structured site draft from the
            approved block set. Leave the brief empty if you just want the
            simpler starter scaffold.
          </p>
        </div>

        <form className="auth-panel" onSubmit={handleSubmit}>
          <label htmlFor="site-name">Business name</label>
          <input
            id="site-name"
            name="name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="Moss & Thread Studio"
            required
          />

          <label htmlFor="site-prompt">Brief</label>
          <textarea
            id="site-prompt"
            name="prompt"
            rows={6}
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            placeholder="A calm one-page site for a textile studio that runs workshops and custom commissions."
          />

          {errorMessage ? <p className="form-error">{errorMessage}</p> : null}

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

      <section className="builder-panel ribbon-panel">
        <div className="panel-heading">
          <p className="eyebrow">Workspace sites</p>
          <h2>Recent drafts</h2>
        </div>

        {isLoading ? (
          <p>Loading sites...</p>
        ) : sites.length === 0 ? (
          <div className="empty-state">
            <p>No sites yet. Create one to open the builder flow.</p>
          </div>
        ) : (
          <div className="site-list">
            {sites.map((site) => (
              <article key={site.id} className="site-card">
                <div>
                  <p className="eyebrow">{site.status}</p>
                  <h3>{site.name}</h3>
                  <p>{site.slug}.local</p>
                </div>
                <dl className="site-card__meta">
                  <div>
                    <dt>Pages</dt>
                    <dd>{site.pageCount}</dd>
                  </div>
                  <div>
                    <dt>Locale</dt>
                    <dd>{site.defaultLocale}</dd>
                  </div>
                </dl>
                <div className="site-card__actions">
                  <Link
                    to="/app/sites/$siteId"
                    params={{ siteId: site.id }}
                    className="site-inline-link"
                  >
                    Open builder
                  </Link>
                  <Link
                    to="/app/sites/$siteId/preview"
                    params={{ siteId: site.id }}
                    className="site-inline-link"
                  >
                    Preview
                  </Link>
                  {site.publishedVersionId ? (
                    <Link
                      to="/public/$siteSlug"
                      params={{ siteSlug: site.slug }}
                      className="site-inline-link"
                    >
                      Live site
                    </Link>
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
