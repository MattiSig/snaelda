import { createFileRoute } from "@tanstack/react-router";
import { getPublishedArtifact } from "@/lib/api";
import {
  buildAppSitemapXML,
  buildTextErrorResponse,
} from "@/lib/published-site";
import { resolveHostedPublicSiteContext } from "@/lib/public-site";

export const Route = createFileRoute("/sitemap.xml")({
  server: {
    handlers: {
      GET: async ({ request }) => {
        const url = new URL(request.url);
        const hostedPublic = resolveHostedPublicSiteContext({
          hostname:
            request.headers.get("x-forwarded-host") ??
            request.headers.get("host") ??
            url.host,
          pagePath: url.pathname,
        });

        if (!hostedPublic.isHostedPublic) {
          return new Response(buildAppSitemapXML(url.origin), {
            headers: {
              "Content-Type": "application/xml; charset=utf-8",
              "Cache-Control": "no-store",
            },
          });
        }

        try {
          const body = await getPublishedArtifact({
            hostname: hostedPublic.hostname,
            path: "sitemap.xml",
          });
          return new Response(body, {
            headers: {
              "Content-Type": "application/xml; charset=utf-8",
              "Cache-Control": "no-store",
            },
          });
        } catch (error) {
          return buildTextErrorResponse(
            error,
            "Could not build sitemap.xml",
            "text/plain; charset=utf-8",
          );
        }
      },
    },
  },
});
