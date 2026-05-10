import { useEffect, useState } from 'react'
import { SiteDraftRenderer } from '@/components/SiteDraftRenderer'
import {
  APIError,
  getPublishedSite,
  type PublishedSiteResponse,
} from '@/lib/api'
import { layout, ribbonPanel, statGrid, text } from '@/lib/styles'

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
      <main className={layout.publicShell}>
        <section className={ribbonPanel}>
          <p className={text.error}>{errorMessage}</p>
        </section>
      </main>
    )
  }

  if (!site) {
    return (
      <main className={layout.publicShell}>
        <section className={ribbonPanel}>
          <p className={text.p}>Loading published page...</p>
        </section>
      </main>
    )
  }

  return (
    <main className={layout.publicShell}>
      <section className={ribbonPanel}>
        <p className={text.eyebrow}>Published snapshot</p>
        <h1 className={text.h1}>{site.snapshot.site.name}</h1>
        <p className={text.p}>
          Serving version {site.version.versionNumber} from the immutable
          publish snapshot at <code>{site.pagePath}</code>.
        </p>
        <dl className={statGrid.list}>
          <div className={statGrid.item}>
            <dt className={text.eyebrow}>Page</dt>
            <dd className={statGrid.value}>{site.page.title}</dd>
          </div>
          <div className={statGrid.item}>
            <dt className={text.eyebrow}>Version</dt>
            <dd className={statGrid.value}>v{site.version.versionNumber}</dd>
          </div>
          <div className={statGrid.item}>
            <dt className={text.eyebrow}>Hostname</dt>
            <dd className={statGrid.value}>{site.hostname || 'local path'}</dd>
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
