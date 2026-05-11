import { createFileRoute } from '@tanstack/react-router'
import { getPublishedSiteByHostname } from '@/lib/api'
import {
  buildAppRobotsTXT,
  buildPublishedRobotsTXT,
  buildTextErrorResponse,
} from '@/lib/published-site'
import { resolveHostedPublicSiteContext } from '@/lib/public-site'

export const Route = createFileRoute('/robots.txt')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        const url = new URL(request.url)
        const hostedPublic = resolveHostedPublicSiteContext({
          hostname:
            request.headers.get('x-forwarded-host') ??
            request.headers.get('host') ??
            url.host,
          pagePath: url.pathname,
        })

        if (!hostedPublic.isHostedPublic) {
          return new Response(buildAppRobotsTXT(url.origin), {
            headers: {
              'Content-Type': 'text/plain; charset=utf-8',
              'Cache-Control': 'no-store',
            },
          })
        }

        try {
          const site = await getPublishedSiteByHostname(hostedPublic.hostname, '/')
          return new Response(buildPublishedRobotsTXT(site), {
            headers: {
              'Content-Type': 'text/plain; charset=utf-8',
              'Cache-Control': 'no-store',
            },
          })
        } catch (error) {
          return buildTextErrorResponse(
            error,
            'Could not build robots.txt',
            'text/plain; charset=utf-8',
          )
        }
      },
    },
  },
})
