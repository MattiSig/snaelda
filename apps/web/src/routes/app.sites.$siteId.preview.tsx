import { Link, createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { SiteDraftRenderer } from '@/components/SiteDraftRenderer'
import { Button } from '@/components/ui/button'
import { APIError, getSiteDraft, type SiteDraft } from '@/lib/api'
import { actions, layout, preview, ribbonPanel, text } from '@/lib/styles'

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
      <div className={ribbonPanel}>
        <p className={text.error}>{errorMessage}</p>
      </div>
    )
  }

  if (!draft) {
    return (
      <div className={ribbonPanel}>
        <p className={text.p}>Loading preview...</p>
      </div>
    )
  }

  return (
    <div className={layout.previewShell}>
      <div className={preview.toolbar}>
        <div>
          <p className={text.eyebrow}>Draft preview</p>
          <strong>{draft.site.name}</strong>
        </div>
        <Button asChild variant="plain" className={actions.inlineLink}>
          <Link to="/app/sites/$siteId" params={{ siteId }}>
            Back to builder
          </Link>
        </Button>
      </div>
      <SiteDraftRenderer site={draft} eyebrow="Draft preview" />
    </div>
  )
}
