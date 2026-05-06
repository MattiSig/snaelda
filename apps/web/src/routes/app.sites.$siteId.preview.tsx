import { Link, createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { SiteDraftRenderer } from '@/components/SiteDraftRenderer'
import { APIError, getSiteDraft, type SiteDraft } from '@/lib/api'

export const Route = createFileRoute('/app/sites/$siteId/preview')({
  component: DraftPreview,
})

function DraftPreview() {
  const { siteId } = Route.useParams()
  const [draft, setDraft] = useState<SiteDraft | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let isMounted = true

    getSiteDraft(siteId)
      .then((response) => {
        if (isMounted) {
          setDraft(response.draft)
        }
      })
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setErrorMessage(
          error instanceof APIError ? error.message : 'Could not load preview',
        )
      })

    return () => {
      isMounted = false
    }
  }, [siteId])

  if (errorMessage) {
    return (
      <div className="builder-panel ribbon-panel">
        <p className="form-error">{errorMessage}</p>
      </div>
    )
  }

  if (!draft) {
    return (
      <div className="builder-panel ribbon-panel">
        <p>Loading preview...</p>
      </div>
    )
  }

  return (
    <div className="preview-shell">
      <div className="preview-toolbar">
        <div>
          <p className="eyebrow">Draft preview</p>
          <strong>{draft.site.name}</strong>
        </div>
        <Link
          to="/app/sites/$siteId"
          params={{ siteId }}
          className="site-inline-link"
        >
          Back to builder
        </Link>
      </div>
      <SiteDraftRenderer draft={draft} />
    </div>
  )
}
