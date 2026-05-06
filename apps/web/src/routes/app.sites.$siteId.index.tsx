import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  APIError,
  deleteSite,
  getSiteDraft,
  type SiteDraft,
  updateSite,
} from '@/lib/api'

export const Route = createFileRoute('/app/sites/$siteId/')({
  component: SiteDetail,
})

function SiteDetail() {
  const { siteId } = Route.useParams()
  const navigate = useNavigate()
  const [draft, setDraft] = useState<SiteDraft | null>(null)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const [statusMessage, setStatusMessage] = useState('')

  useEffect(() => {
    let isMounted = true

    getSiteDraft(siteId)
      .then((response) => {
        if (!isMounted) {
          return
        }
        setDraft(response.draft)
        setName(response.draft.site.name)
        setSlug(response.draft.site.slug)
        setIsLoading(false)
      })
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setErrorMessage(
          error instanceof APIError ? error.message : 'Could not load site',
        )
        setIsLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [siteId])

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setIsSaving(true)
    setErrorMessage('')
    setStatusMessage('')

    try {
      const response = await updateSite(siteId, { name, slug })
      setDraft(response.draft)
      setName(response.draft.site.name)
      setSlug(response.draft.site.slug)
      setStatusMessage('Site details saved.')
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not save site',
      )
    } finally {
      setIsSaving(false)
    }
  }

  async function handleDelete() {
    const confirmed = window.confirm(
      'Delete this site draft? This removes the stored draft and its pages.',
    )
    if (!confirmed) {
      return
    }

    setIsDeleting(true)
    setErrorMessage('')

    try {
      await deleteSite(siteId)
      await navigate({ to: '/app' })
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : 'Could not delete site',
      )
      setIsDeleting(false)
    }
  }

  if (isLoading) {
    return (
      <div className="builder-panel ribbon-panel">
        <p>Loading site...</p>
      </div>
    )
  }

  if (!draft) {
    return (
      <div className="builder-panel ribbon-panel">
        <p className="form-error">{errorMessage || 'Site not found'}</p>
      </div>
    )
  }

  const blockCount = draft.pages.reduce((count, page) => count + page.blocks.length, 0)

  return (
    <div className="builder-grid">
      <section className="builder-panel ribbon-panel">
        <div className="panel-heading">
          <p className="eyebrow">Site</p>
          <h1>{draft.site.name}</h1>
          <p>
            Update the site identity here, then open preview to inspect the
            current draft rendered from the stored config.
          </p>
        </div>

        <dl className="metadata-list">
          <div>
            <dt>Status</dt>
            <dd>{draft.site.status}</dd>
          </div>
          <div>
            <dt>Pages</dt>
            <dd>{draft.pages.length}</dd>
          </div>
          <div>
            <dt>Blocks</dt>
            <dd>{blockCount}</dd>
          </div>
        </dl>

        <div className="site-detail-links">
          <Link
            to="/app/sites/$siteId/preview"
            params={{ siteId }}
            className="site-inline-link"
          >
            Open preview
          </Link>
        </div>

        <div className="page-outline">
          {draft.pages.map((page) => (
            <article key={page.id} className="page-outline__item">
              <div>
                <h3>{page.title}</h3>
                <p>{page.slug}</p>
              </div>
              <strong>{page.blocks.length} blocks</strong>
            </article>
          ))}
        </div>
      </section>

      <section className="editor-panel ribbon-panel">
        <div className="panel-heading">
          <p className="eyebrow">Site details</p>
          <h2>Rename and reslug the draft</h2>
        </div>

        <form className="auth-panel" onSubmit={handleSave}>
          <label htmlFor="site-name">Business name</label>
          <input
            id="site-name"
            name="name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            required
          />

          <label htmlFor="site-slug">Slug</label>
          <input
            id="site-slug"
            name="slug"
            value={slug}
            onChange={(event) => setSlug(event.target.value)}
            required
          />

          {errorMessage ? <p className="form-error">{errorMessage}</p> : null}
          {statusMessage ? <p className="form-success">{statusMessage}</p> : null}

          <Button type="submit" disabled={isSaving}>
            {isSaving ? 'Saving...' : 'Save site'}
          </Button>

          <Button
            type="button"
            variant="outline"
            disabled={isDeleting}
            onClick={handleDelete}
          >
            {isDeleting ? 'Deleting...' : 'Delete draft'}
          </Button>
        </form>
      </section>
    </div>
  )
}
