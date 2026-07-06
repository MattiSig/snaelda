import { APIError, type PublishedSiteResponse } from "@/lib/api";
import { getAppBaseURL, normalizePagePath } from "@/lib/public-site";

export type PublishedSitePageData = {
  site: PublishedSiteResponse | null;
  errorMessage: string;
};

export async function loadPublishedSitePageData(
  load: () => Promise<PublishedSiteResponse>,
  fallbackMessage = "Could not load published page",
): Promise<PublishedSitePageData> {
  try {
    return {
      site: await load(),
      errorMessage: "",
    };
  } catch (error) {
    return {
      site: null,
      errorMessage: getPublishedSiteErrorMessage(error, fallbackMessage),
    };
  }
}

export function buildPublishedPageHead(site?: PublishedSiteResponse | null) {
  if (!site) {
    return {};
  }

  const title = site.page.title;
  const description = site.page.description;
  const canonicalUrl = site.page.canonicalUrl || buildPublishedPageURL(site);
  const brandName = site.brand?.businessName?.trim() || site.siteSlug;
  const ogLocale = publishedOGLocale(site.defaultLocale);

  return {
    links: [{ rel: "canonical", href: canonicalUrl }],
    meta: [
      { title },
      { name: "description", content: description },
      { property: "og:type", content: "website" },
      { property: "og:site_name", content: brandName },
      { property: "og:title", content: title },
      { property: "og:description", content: description },
      { property: "og:url", content: canonicalUrl },
      { property: "og:locale", content: ogLocale },
      { name: "twitter:card", content: "summary" },
      { name: "twitter:title", content: title },
      { name: "twitter:description", content: description },
      ...(site.page.ogImageUrl
        ? [
            { property: "og:image", content: site.page.ogImageUrl },
            { name: "twitter:image", content: site.page.ogImageUrl },
          ]
        : []),
    ],
    scripts: site.page.localBusinessJsonLd
      ? [
          {
            type: "application/ld+json",
            children: JSON.stringify(site.page.localBusinessJsonLd),
          },
        ]
      : [],
  };
}

export function buildAppSitemapXML(appBaseURL = getAppBaseURL()) {
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    `  <url><loc>${escapeXML(new URL("/", appBaseURL).toString())}</loc></url>`,
    `  <url><loc>${escapeXML(new URL("/login", appBaseURL).toString())}</loc></url>`,
    "</urlset>",
    "",
  ].join("\n");
}

export function buildAppRobotsTXT(appBaseURL = getAppBaseURL()) {
  return [
    "User-agent: *",
    "Allow: /",
    "Disallow: /api",
    "Disallow: /app",
    `Sitemap: ${new URL("/sitemap.xml", appBaseURL).toString()}`,
    "",
  ].join("\n");
}

export function buildPublishedPageURL(
  site: PublishedSiteResponse,
  pagePath = site.pagePath,
) {
  const url = new URL(site.publicUrl, getAppBaseURL());
  if (site.hostname) {
    url.hostname = site.hostname;
  }
  url.pathname = resolvePublishedRoutePath(site, pagePath);
  url.search = "";
  url.hash = "";
  return url.toString();
}

export function buildPublishedAssetURL(
  site: PublishedSiteResponse,
  assetName: "robots.txt" | "sitemap.xml",
) {
  const url = new URL(site.publicUrl, getAppBaseURL());
  if (site.hostname) {
    url.hostname = site.hostname;
  }
  url.pathname = site.hostname
    ? `/${assetName}`
    : `/public/${site.siteSlug}/${assetName}`;
  url.search = "";
  url.hash = "";
  return url.toString();
}

export function buildTextErrorResponse(
  error: unknown,
  fallbackMessage: string,
  contentType: string,
) {
  return new Response(getPublishedSiteErrorMessage(error, fallbackMessage), {
    status: error instanceof APIError ? error.status : 500,
    headers: {
      "Content-Type": contentType,
      "Cache-Control": "no-store",
    },
  });
}

function resolvePublishedRoutePath(
  site: PublishedSiteResponse,
  pagePath: string,
) {
  const normalizedPath = normalizePagePath(pagePath);
  if (site.hostname) {
    return normalizedPath;
  }
  return normalizedPath === "/"
    ? `/public/${site.siteSlug}`
    : `/public/${site.siteSlug}${normalizedPath}`;
}

// publishedSiteHtmlLang reduces a published site's default locale to a
// supported `<html lang>` value ("is-IS" -> "is"), defaulting to "en". Published
// pages carry their own content locale, independent of the visitor's UI locale.
export function publishedSiteHtmlLang(defaultLocale?: string | null): string {
  const primary = (defaultLocale ?? "").trim().toLowerCase().split(/[-_]/)[0];
  return primary === "is" ? "is" : "en";
}

// publishedOGLocale maps the published locale to an Open Graph locale tag
// (`is_IS` / `en_US`).
export function publishedOGLocale(defaultLocale?: string | null): string {
  return publishedSiteHtmlLang(defaultLocale) === "is" ? "is_IS" : "en_US";
}

function getPublishedSiteErrorMessage(error: unknown, fallbackMessage: string) {
  return error instanceof APIError ? error.message : fallbackMessage;
}

function escapeXML(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}
