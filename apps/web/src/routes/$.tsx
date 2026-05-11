import { createFileRoute } from '@tanstack/react-router'
import { NotFound } from '@/components/NotFound'
import { PublishedSitePage } from '@/components/PublishedSitePage'
import { getPublishedSiteByHostname } from '@/lib/api'
import {
  buildPublishedPageHead,
  loadPublishedSitePageData,
} from '@/lib/published-site'
import { getHostedPublicSiteContext } from '@/lib/public-site'

export const Route = createFileRoute('/$')({
  loader: async () => {
    const hostedPublic = await getHostedPublicSiteContext()
    return {
      hostedPublic,
      published: hostedPublic.isHostedPublic
        ? await loadPublishedSitePageData(() =>
            getPublishedSiteByHostname(
              hostedPublic.hostname,
              hostedPublic.pagePath,
            ),
          )
        : { site: null, errorMessage: '' },
    }
  },
  head: ({ loaderData }) => buildPublishedPageHead(loaderData?.published.site),
  component: HostedPublicCatchAllRoute,
})

function HostedPublicCatchAllRoute() {
  const { hostedPublic, published } = Route.useLoaderData()

  if (!hostedPublic.isHostedPublic) {
    return <NotFound />
  }

  return (
    <PublishedSitePage
      site={published.site}
      errorMessage={published.errorMessage}
    />
  )
}
