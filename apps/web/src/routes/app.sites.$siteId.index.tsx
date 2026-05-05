import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/app/sites/$siteId/')({
  component: SiteDetail,
})

function SiteDetail() {
  const { siteId } = Route.useParams()

  return (
    <div className="builder-grid">
      <section className="builder-panel">
        <p className="eyebrow">Site</p>
        <h1>{siteId}</h1>
        <dl className="metadata-list">
          <div>
            <dt>Status</dt>
            <dd>Draft</dd>
          </div>
          <div>
            <dt>Pages</dt>
            <dd>0</dd>
          </div>
          <div>
            <dt>Blocks</dt>
            <dd>0</dd>
          </div>
        </dl>
      </section>

      <section className="editor-panel">
        <h2>Block fields</h2>
        <label htmlFor="headline">Headline</label>
        <input id="headline" name="headline" disabled />
        <label htmlFor="subheadline">Subheadline</label>
        <textarea id="subheadline" name="subheadline" rows={5} disabled />
        <button type="button" disabled>
          Save
        </button>
      </section>
    </div>
  )
}
