import { createFileRoute } from '@tanstack/react-router'
import { NotFound } from '@/components/NotFound'
import { PublishedSitePage } from '@/components/PublishedSitePage'
import { getHostedPublicSiteContext } from '@/lib/public-site'

export const Route = createFileRoute('/$')({
  loader: async () => ({
    hostedPublic: await getHostedPublicSiteContext(),
  }),
  component: HostedPublicCatchAllRoute,
})

function HostedPublicCatchAllRoute() {
  const { hostedPublic } = Route.useLoaderData()

  if (!hostedPublic.isHostedPublic) {
    return <NotFound />
  }

  return (
    <PublishedSitePage
      hostname={hostedPublic.hostname}
      pagePath={hostedPublic.pagePath}
    />
  )
}
