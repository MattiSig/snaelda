import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { SiteDraftRenderer } from '@/components/SiteDraftRenderer'
import {
  APIError,
  getPublishedSite,
  type PublishedSiteResponse,
} from '@/lib/api'

export const Route = createFileRoute('/public/$siteSlug')({
  component: PublicSite,
})

function PublicSite() {
  const { siteSlug } = Route.useParams()
  const [site, setSite] = useState<PublishedSiteResponse | null>(null)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let isMounted = true

    getPublishedSite(siteSlug)
      .then((response) => {
        if (isMounted) {
          setSite(response)
        }
      })
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setErrorMessage(
          error instanceof APIError ? error.message : 'Could not load published site',
        )
      })

    return () => {
      isMounted = false
    }
  }, [siteSlug])

  if (errorMessage) {
    return (
      <main className="public-shell">
        <section className="ribbon-panel">
          <p className="form-error">{errorMessage}</p>
        </section>
      </main>
    )
  }

  if (!site) {
    return (
      <main className="public-shell">
        <section className="ribbon-panel">
          <p>Loading published site...</p>
        </section>
      </main>
    )
  }

  return (
    <main className="public-shell">
      <section className="ribbon-panel">
        <p className="eyebrow">Published snapshot</p>
        <h1>{site.snapshot.site.name}</h1>
        <p>
          Serving version {site.version.versionNumber} from the immutable
          publish snapshot instead of the mutable draft tables.
        </p>
        <dl className="publish-meta">
          <div>
            <dt>Version</dt>
            <dd>v{site.version.versionNumber}</dd>
          </div>
          <div>
            <dt>Slug</dt>
            <dd>{site.siteSlug}</dd>
          </div>
          <div>
            <dt>Hostname</dt>
            <dd>{site.hostname || 'local path'}</dd>
          </div>
        </dl>
      </section>
      <SiteDraftRenderer site={site.snapshot} eyebrow="Published site" showPageMeta={false} />
    </main>
  )
}
