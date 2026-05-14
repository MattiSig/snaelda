import { renderToStaticMarkup } from "react-dom/server";
import type { PublishedSnapshot, SiteVersion } from "@/lib/api";
import { SiteDraftRenderer } from "@/components/SiteDraftRenderer";
import { normalizePagePath } from "@/lib/public-site";
import { buildSiteThemeStyle } from "@/lib/site-theme";

export type PublishedArtifactRenderInput = {
  appBaseURL: string;
  siteSlug: string;
  hostname?: string;
  version: SiteVersion;
  snapshot: PublishedSnapshot;
};

export type PublishedArtifactFile = {
  path: string;
  contentType: string;
  body: string;
};

export type PublishedArtifactBundle = {
  schemaVersion: "published_artifacts.v1";
  files: PublishedArtifactFile[];
};

type PublishedArtifactManifest = {
  schemaVersion: "published_artifacts.v1";
  siteSlug: string;
  hostname?: string;
  version: SiteVersion;
  pages: Array<{
    pagePath: string;
    filePath: string;
    title: string;
    description: string;
    canonicalUrl: string;
  }>;
};

export function buildPublishedArtifactBundle(
  input: PublishedArtifactRenderInput,
): PublishedArtifactBundle {
  const pages = input.snapshot.pages.map((page) => {
    const filePath = buildPageArtifactPath(page.slug);
    const title =
      page.seo?.title || input.snapshot.site.seo?.title || page.title;
    const description =
      page.seo?.description ||
      input.snapshot.site.seo?.description ||
      `Visit ${input.snapshot.site.name}.`;
    const canonicalUrl = buildCanonicalURL(input, page.slug);

    return {
      pagePath: normalizePagePath(page.slug),
      filePath,
      title,
      description,
      canonicalUrl,
      html: renderToStaticMarkup(
        <SiteDraftRenderer
          site={input.snapshot}
          eyebrow=""
          showPageMeta={false}
          selectedPageId={page.id}
          linkMode="published"
          siteSlug={input.siteSlug}
          publishedBasePath=""
        />,
      ),
    };
  });

  const manifest: PublishedArtifactManifest = {
    schemaVersion: "published_artifacts.v1",
    siteSlug: input.siteSlug,
    hostname: input.hostname || undefined,
    version: input.version,
    pages: pages.map(
      ({ pagePath, filePath, title, description, canonicalUrl }) => ({
        pagePath,
        filePath,
        title,
        description,
        canonicalUrl,
      }),
    ),
  };

  const files: PublishedArtifactFile[] = pages.map(({ filePath, html }) => ({
    path: filePath,
    contentType: "text/html; charset=utf-8",
    body: html,
  }));

  files.push({
    path: "assets/theme.css",
    contentType: "text/css; charset=utf-8",
    body: buildPublishedThemeCSS(input.snapshot),
  });
  files.push({
    path: "sitemap.xml",
    contentType: "application/xml; charset=utf-8",
    body: buildSitemapXML(manifest),
  });
  files.push({
    path: "robots.txt",
    contentType: "text/plain; charset=utf-8",
    body: buildRobotsTXT(manifest),
  });
  files.push({
    path: "manifest.json",
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(manifest, null, 2) + "\n",
  });

  return {
    schemaVersion: "published_artifacts.v1",
    files,
  };
}

function buildPageArtifactPath(pagePath: string) {
  const normalizedPath = normalizePagePath(pagePath);
  if (normalizedPath === "/") {
    return "pages/index.html";
  }

  return `pages${normalizedPath}/index.html`;
}

function buildPublishedThemeCSS(snapshot: PublishedSnapshot) {
  const style = buildSiteThemeStyle(snapshot.theme);
  const declarations = Object.entries(style)
    .filter(
      ([, value]) => value !== undefined && value !== null && value !== "",
    )
    .map(([name, value]) => `  ${name}: ${String(value)};`);

  return [":root {", ...declarations, "}", ""].join("\n");
}

function buildSitemapXML(manifest: PublishedArtifactManifest) {
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="https://www.sitemaps.org/schemas/sitemap/0.9">',
    ...manifest.pages.map(
      (page) => `  <url><loc>${escapeXML(page.canonicalUrl)}</loc></url>`,
    ),
    "</urlset>",
    "",
  ].join("\n");
}

function buildRobotsTXT(manifest: PublishedArtifactManifest) {
  const sitemapPath = manifest.hostname
    ? "/sitemap.xml"
    : `/public/${manifest.siteSlug}/sitemap.xml`;
  const sitemapURL = new URL(
    sitemapPath,
    manifest.pages[0]?.canonicalUrl ?? "http://localhost:3000",
  ).toString();

  return ["User-agent: *", "Allow: /", `Sitemap: ${sitemapURL}`, ""].join("\n");
}

function buildCanonicalURL(
  input: PublishedArtifactRenderInput,
  pagePath: string,
) {
  const url = new URL(input.appBaseURL);
  if (input.hostname) {
    url.hostname = input.hostname;
  }
  url.pathname = normalizePagePath(pagePath);
  url.search = "";
  url.hash = "";
  return url.toString();
}

function escapeXML(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}
