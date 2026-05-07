import { useEffect, useState } from 'react'
import { SiteDraftRenderer } from '@/components/SiteDraftRenderer'
import {
  APIError,
  getPublishedSite,
  type PublishedSiteResponse,
} from '@/lib/api'

export function PublishedSitePage({
  siteSlug,
  pagePath,
}: {
  siteSlug: string
  pagePath: string
}) {
  const requestKey = `${siteSlug}:${pagePath}`
  const [state, setState] = useState<{
    key: string
    site: PublishedSiteResponse | null
    errorMessage: string
  }>({
    key: requestKey,
    site: null,
    errorMessage: '',
  })

  useEffect(() => {
    let isMounted = true

    getPublishedSite(siteSlug, pagePath)
      .then((response) => {
        if (isMounted) {
          setState({
            key: requestKey,
            site: response,
            errorMessage: '',
          })
        }
      })
      .catch((error) => {
        if (!isMounted) {
          return
        }
        setState({
          key: requestKey,
          site: null,
          errorMessage:
            error instanceof APIError ? error.message : 'Could not load published page',
        })
      })

    return () => {
      isMounted = false
    }
  }, [pagePath, requestKey, siteSlug])

  const site = state.key === requestKey ? state.site : null
  const errorMessage = state.key === requestKey ? state.errorMessage : ''

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
          <p>Loading published page...</p>
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
          publish snapshot at <code>{site.pagePath}</code>.
        </p>
        <dl className="publish-meta">
          <div>
            <dt>Page</dt>
            <dd>{site.page.title}</dd>
          </div>
          <div>
            <dt>Version</dt>
            <dd>v{site.version.versionNumber}</dd>
          </div>
          <div>
            <dt>Hostname</dt>
            <dd>{site.hostname || 'local path'}</dd>
          </div>
        </dl>
      </section>
      <SiteDraftRenderer
        site={site.snapshot}
        eyebrow="Published page"
        showPageMeta={false}
        selectedPageId={site.page.id}
        linkMode="published"
        siteSlug={site.siteSlug}
      />
    </main>
  )
}
