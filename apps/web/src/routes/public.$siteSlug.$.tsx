import { createFileRoute } from '@tanstack/react-router'
import { PublishedSitePage } from '@/components/PublishedSitePage'

export const Route = createFileRoute('/public/$siteSlug/$')({
  component: PublicSitePageRoute,
})

function PublicSitePageRoute() {
  const { siteSlug, _splat } = Route.useParams()

  return <PublishedSitePage siteSlug={siteSlug} pagePath={`/${_splat}`} />
}
