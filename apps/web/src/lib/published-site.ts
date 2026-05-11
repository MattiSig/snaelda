import { APIError, type PublishedSiteResponse } from '@/lib/api'
import { getAppBaseURL, normalizePagePath } from '@/lib/public-site'

export type PublishedSitePageData = {
  site: PublishedSiteResponse | null
  errorMessage: string
}

export async function loadPublishedSitePageData(
  load: () => Promise<PublishedSiteResponse>,
  fallbackMessage = 'Could not load published page',
): Promise<PublishedSitePageData> {
  try {
    return {
      site: await load(),
      errorMessage: '',
    }
  } catch (error) {
    return {
      site: null,
      errorMessage: getPublishedSiteErrorMessage(error, fallbackMessage),
    }
  }
}

export function buildPublishedPageHead(site?: PublishedSiteResponse | null) {
  if (!site) {
    return {}
  }

  const title =
    site.page.seo?.title || site.snapshot.site.seo?.title || site.page.title
  const description =
    site.page.seo?.description ||
    site.snapshot.site.seo?.description ||
    `Visit ${site.snapshot.site.name}.`
  const canonicalUrl = buildPublishedPageURL(site, site.page.slug)

  return {
    links: [{ rel: 'canonical', href: canonicalUrl }],
    meta: [
      { title },
      { name: 'description', content: description },
      { property: 'og:type', content: 'website' },
      { property: 'og:site_name', content: site.snapshot.site.name },
      { property: 'og:title', content: title },
      { property: 'og:description', content: description },
      { property: 'og:url', content: canonicalUrl },
      { name: 'twitter:card', content: 'summary' },
      { name: 'twitter:title', content: title },
      { name: 'twitter:description', content: description },
    ],
  }
}

export function buildPublishedSitemapXML(site: PublishedSiteResponse) {
  const urls = site.snapshot.pages.map((page) =>
    buildPublishedPageURL(site, page.slug),
  )

  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="https://www.sitemaps.org/schemas/sitemap/0.9">',
    ...urls.map((url) => `  <url><loc>${escapeXML(url)}</loc></url>`),
    '</urlset>',
    '',
  ].join('\n')
}

export function buildPublishedRobotsTXT(site: PublishedSiteResponse) {
  return [
    'User-agent: *',
    'Allow: /',
    `Sitemap: ${buildPublishedAssetURL(site, 'sitemap.xml')}`,
    '',
  ].join('\n')
}

export function buildAppSitemapXML(appBaseURL = getAppBaseURL()) {
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="https://www.sitemaps.org/schemas/sitemap/0.9">',
    `  <url><loc>${escapeXML(new URL('/', appBaseURL).toString())}</loc></url>`,
    `  <url><loc>${escapeXML(new URL('/login', appBaseURL).toString())}</loc></url>`,
    '</urlset>',
    '',
  ].join('\n')
}

export function buildAppRobotsTXT(appBaseURL = getAppBaseURL()) {
  return [
    'User-agent: *',
    'Allow: /',
    'Disallow: /api',
    'Disallow: /app',
    `Sitemap: ${new URL('/sitemap.xml', appBaseURL).toString()}`,
    '',
  ].join('\n')
}

export function buildPublishedPageURL(
  site: PublishedSiteResponse,
  pagePath = '/',
) {
  const url = new URL(site.publicUrl, getAppBaseURL())
  if (site.hostname) {
    url.hostname = site.hostname
  }
  url.pathname = resolvePublishedRoutePath(site, pagePath)
  url.search = ''
  url.hash = ''
  return url.toString()
}

export function buildPublishedAssetURL(
  site: PublishedSiteResponse,
  assetName: 'robots.txt' | 'sitemap.xml',
) {
  const url = new URL(site.publicUrl, getAppBaseURL())
  if (site.hostname) {
    url.hostname = site.hostname
  }
  url.pathname = site.hostname
    ? `/${assetName}`
    : `/public/${site.siteSlug}/${assetName}`
  url.search = ''
  url.hash = ''
  return url.toString()
}

export function buildTextErrorResponse(
  error: unknown,
  fallbackMessage: string,
  contentType: string,
) {
  return new Response(getPublishedSiteErrorMessage(error, fallbackMessage), {
    status: error instanceof APIError ? error.status : 500,
    headers: {
      'Content-Type': contentType,
      'Cache-Control': 'no-store',
    },
  })
}

function resolvePublishedRoutePath(
  site: PublishedSiteResponse,
  pagePath: string,
) {
  const normalizedPath = normalizePagePath(pagePath)
  if (site.hostname) {
    return normalizedPath
  }
  return normalizedPath === '/'
    ? `/public/${site.siteSlug}`
    : `/public/${site.siteSlug}${normalizedPath}`
}

function getPublishedSiteErrorMessage(error: unknown, fallbackMessage: string) {
  return error instanceof APIError ? error.message : fallbackMessage
}

function escapeXML(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;')
}
