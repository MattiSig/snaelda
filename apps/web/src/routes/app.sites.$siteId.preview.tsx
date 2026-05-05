import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/app/sites/$siteId/preview')({
  component: DraftPreview,
})

function DraftPreview() {
  const { siteId } = Route.useParams()

  return (
    <div className="preview-surface">
      <div className="preview-toolbar">
        <p className="eyebrow">Draft preview</p>
        <strong>{siteId}</strong>
      </div>
      <article className="render-frame">
        <h1>Preview will render the current draft.</h1>
      </article>
    </div>
  )
}
