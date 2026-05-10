import { useEffect, useState } from 'react'
import { SiteDraftRenderer } from '@/components/SiteDraftRenderer'
import {
  APIError,
  getPublishedSite,
  getPublishedSiteByHostname,
  type PublishedSiteResponse,
} from '@/lib/api'
import { layout, ribbonPanel, statGrid, text } from '@/lib/styles'

export function PublishedSitePage({
  siteSlug,
  hostname,
  pagePath,
}: {
  siteSlug?: string
  hostname?: string
  pagePath: string
}) {
  const source = siteSlug ? `slug:${siteSlug}` : `host:${hostname ?? ''}`
  const requestKey = `${source}:${pagePath}`
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
    const request = siteSlug
      ? getPublishedSite(siteSlug, pagePath)
      : getPublishedSiteByHostname(hostname ?? '', pagePath)

    request
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
  }, [hostname, pagePath, requestKey, siteSlug])

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
            <dd className={statGrid.value}>{site.hostname || hostname || 'local path'}</dd>
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
        publishedBasePath={hostname ? '' : undefined}
      />
    </main>
  )
}
