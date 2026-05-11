import { createFileRoute } from '@tanstack/react-router'
import { getPublishedSite } from '@/lib/api'
import {
  buildPublishedSitemapXML,
  buildTextErrorResponse,
} from '@/lib/published-site'

export const Route = createFileRoute('/public/$siteSlug/sitemap.xml')({
  server: {
    handlers: {
      GET: async ({ params }) => {
        try {
          const site = await getPublishedSite(params.siteSlug, '/')
          return new Response(buildPublishedSitemapXML(site), {
            headers: {
              'Content-Type': 'application/xml; charset=utf-8',
              'Cache-Control': 'no-store',
            },
          })
        } catch (error) {
          return buildTextErrorResponse(
            error,
            'Could not build sitemap.xml',
            'text/plain; charset=utf-8',
          )
        }
      },
    },
  },
})
