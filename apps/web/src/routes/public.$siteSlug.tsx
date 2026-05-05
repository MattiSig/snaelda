import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/public/$siteSlug')({
  component: PublicExperiment,
})

function PublicExperiment() {
  const { siteSlug } = Route.useParams()

  return (
    <main className="public-shell">
      <section>
        <p className="eyebrow">Public render</p>
        <h1>{siteSlug}</h1>
        <p>Published pages will render from immutable snapshots.</p>
      </section>
    </main>
  )
}
