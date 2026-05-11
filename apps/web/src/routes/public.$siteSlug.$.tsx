import { createFileRoute } from '@tanstack/react-router'
import { PublishedSitePage } from '@/components/PublishedSitePage'
import { getPublishedSite } from '@/lib/api'
import {
  buildPublishedPageHead,
  loadPublishedSitePageData,
} from '@/lib/published-site'

export const Route = createFileRoute('/public/$siteSlug/$')({
  loader: async ({ params }) =>
    loadPublishedSitePageData(() =>
      getPublishedSite(params.siteSlug, `/${params._splat}`),
    ),
  head: ({ loaderData }) => buildPublishedPageHead(loaderData?.site),
  component: PublicSitePageRoute,
})

function PublicSitePageRoute() {
  const published = Route.useLoaderData()

  return (
    <PublishedSitePage
      site={published.site}
      errorMessage={published.errorMessage}
    />
  )
}
