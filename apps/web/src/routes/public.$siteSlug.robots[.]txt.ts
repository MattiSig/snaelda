import { createFileRoute } from '@tanstack/react-router'
import { getPublishedSite } from '@/lib/api'
import {
  buildPublishedRobotsTXT,
  buildTextErrorResponse,
} from '@/lib/published-site'

export const Route = createFileRoute('/public/$siteSlug/robots.txt')({
  server: {
    handlers: {
      GET: async ({ params }) => {
        try {
          const site = await getPublishedSite(params.siteSlug, '/')
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
