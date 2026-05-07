import { createFileRoute } from '@tanstack/react-router'
import { PublishedSitePage } from '@/components/PublishedSitePage'

export const Route = createFileRoute('/public/$siteSlug/')({
  component: PublicSiteIndex,
})

function PublicSiteIndex() {
  const { siteSlug } = Route.useParams()

  return <PublishedSitePage siteSlug={siteSlug} pagePath="/" />
}
