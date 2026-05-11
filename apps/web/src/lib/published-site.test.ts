import { describe, expect, it } from 'vitest'
import type { PublishedSiteResponse } from '@/lib/api'
import {
  buildAppRobotsTXT,
  buildAppSitemapXML,
  buildPublishedPageHead,
  buildPublishedPageURL,
  buildPublishedRobotsTXT,
  buildPublishedSitemapXML,
} from './published-site'

describe('published site helpers', () => {
  it('builds canonical and social metadata from page seo', () => {
    const result = buildPublishedPageHead(buildPathPublishedSite())

    expect(result.links).toEqual([
      {
        rel: 'canonical',
        href: 'http://loom-light.localhost:3000/contact',
      },
    ])
    expect(result.meta).toContainEqual({
      property: 'og:title',
      content: 'Contact | Loom & Light',
    })
    expect(result.meta).toContainEqual({
      name: 'twitter:description',
      content: 'Drop by the studio this weekend.',
    })
  })

  it('builds page urls for hosted domains without the local public prefix', () => {
    expect(buildPublishedPageURL(buildHostedPublishedSite(), '/contact')).toBe(
      'http://loom-light.localhost:3000/contact',
    )
  })

  it('builds sitemap and robots outputs for published sites', () => {
    const site = buildPathPublishedSite()
    const sitemap = buildPublishedSitemapXML(site)
    const robots = buildPublishedRobotsTXT(site)

    expect(sitemap).toContain(
      '<loc>http://loom-light.localhost:3000/</loc>',
    )
    expect(sitemap).toContain(
      '<loc>http://loom-light.localhost:3000/contact</loc>',
    )
    expect(robots).toContain(
      'Sitemap: http://loom-light.localhost:3000/sitemap.xml',
    )
  })

  it('builds app-level crawl files for the builder domain', () => {
    expect(buildAppSitemapXML('http://localhost:3000')).toContain(
      '<loc>http://localhost:3000/login</loc>',
    )
    expect(buildAppRobotsTXT('http://localhost:3000')).toContain(
      'Disallow: /app',
    )
  })
})

function buildPathPublishedSite(): PublishedSiteResponse {
  return {
    siteSlug: 'loom-light',
    hostname: 'loom-light.localhost',
    publicUrl: 'http://localhost:3000/public/loom-light/contact',
    pagePath: '/contact',
    version: {
      id: 'version-3',
      siteId: 'site-1',
      versionNumber: 3,
      createdAt: '2026-05-10T09:00:00Z',
      isCurrent: true,
    },
    page: {
      id: 'page-contact',
      title: 'Contact',
      slug: '/contact',
      seo: {
        title: 'Contact | Loom & Light',
        description: 'Drop by the studio this weekend.',
      },
      blocks: [],
    },
    snapshot: {
      schemaVersion: 'site_snapshot.v1',
      site: {
        id: 'site-1',
        name: 'Loom & Light',
        defaultLocale: 'en',
        seo: {
          title: 'Loom & Light',
          description: 'Warm sites spun from a prompt.',
        },
      },
      theme: {
        version: 'theme.v1',
        tokens: {
          colors: {
            background: '#151215',
            foreground: '#f3ead8',
          },
          typography: {},
          layout: {},
          shape: {},
        },
      },
      navigation: {
        primary: [],
      },
      pages: [
        {
          id: 'page-home',
          title: 'Home',
          slug: '/',
          blocks: [],
        },
        {
          id: 'page-contact',
          title: 'Contact',
          slug: '/contact',
          seo: {
            title: 'Contact | Loom & Light',
            description: 'Drop by the studio this weekend.',
          },
          blocks: [],
        },
      ],
    },
  }
}

function buildHostedPublishedSite(): PublishedSiteResponse {
  return {
    ...buildPathPublishedSite(),
    publicUrl: 'http://loom-light.localhost:3000/contact',
  }
}
