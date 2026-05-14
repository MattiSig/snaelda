import { createFileRoute } from "@tanstack/react-router";
import { getPublishedArtifact } from "@/lib/api";
import { buildTextErrorResponse } from "@/lib/published-site";

export const Route = createFileRoute("/public/$siteSlug/robots.txt")({
  server: {
    handlers: {
      GET: async ({ params }) => {
        try {
          const body = await getPublishedArtifact({
            siteSlug: params.siteSlug,
            path: "robots.txt",
          });
          return new Response(body, {
            headers: {
              "Content-Type": "text/plain; charset=utf-8",
              "Cache-Control": "no-store",
            },
          });
        } catch (error) {
          return buildTextErrorResponse(
            error,
            "Could not build robots.txt",
            "text/plain; charset=utf-8",
          );
        }
      },
    },
  },
});
