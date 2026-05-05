import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/preview/$token')({
  component: TokenPreview,
})

function TokenPreview() {
  const { token } = Route.useParams()

  return (
    <main className="page-shell">
      <section className="preview-surface">
        <div className="preview-toolbar">
          <p className="eyebrow">Preview token</p>
          <strong>{token}</strong>
        </div>
        <article className="render-frame">
          <h1>Preview route</h1>
        </article>
      </section>
    </main>
  )
}
