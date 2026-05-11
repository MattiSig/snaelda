import { createFileRoute } from '@tanstack/react-router'
import { PublishedSitePage } from '@/components/PublishedSitePage'
import { getPublishedSite } from '@/lib/api'
import {
  buildPublishedPageHead,
  loadPublishedSitePageData,
} from '@/lib/published-site'

export const Route = createFileRoute('/public/$siteSlug/')({
  loader: async ({ params }) =>
    loadPublishedSitePageData(() => getPublishedSite(params.siteSlug, '/')),
  head: ({ loaderData }) => buildPublishedPageHead(loaderData?.site),
  component: PublicSiteIndex,
})

function PublicSiteIndex() {
  const published = Route.useLoaderData()

  return (
    <PublishedSitePage
      site={published.site}
      errorMessage={published.errorMessage}
    />
  )
}
