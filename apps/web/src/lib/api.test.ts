import { afterEach, describe, expect, it } from 'vitest'
import {
  getAPIBaseURL,
  normalizePublishedSiteResponse,
  resolveRuntimeAPIBaseURL,
} from './api'

describe('api base url', () => {
  const originalViteAPIBaseURL = process.env.VITE_API_BASE_URL
  const originalAPIBaseURL = process.env.API_BASE_URL

  afterEach(() => {
    if (originalViteAPIBaseURL === undefined) {
      delete process.env.VITE_API_BASE_URL
    } else {
      process.env.VITE_API_BASE_URL = originalViteAPIBaseURL
    }
    if (originalAPIBaseURL === undefined) {
      delete process.env.API_BASE_URL
    } else {
      process.env.API_BASE_URL = originalAPIBaseURL
    }
  })

  it('uses the production API from the production browser origin', () => {
    expect(
      resolveRuntimeAPIBaseURL({ hostname: 'snaelda.io', protocol: 'https:' }),
    ).toBe('https://api.snaelda.io')
  })

  it('uses the production API from the www production browser origin', () => {
    expect(
      resolveRuntimeAPIBaseURL({ hostname: 'www.snaelda.io', protocol: 'https:' }),
    ).toBe('https://api.snaelda.io')
  })

  it('keeps the local API fallback for localhost', () => {
    delete process.env.VITE_API_BASE_URL
    delete process.env.API_BASE_URL
    window.history.replaceState(null, '', 'http://localhost:3000/')

    expect(getAPIBaseURL()).toBe('http://localhost:8080')
  })

  it('uses API_BASE_URL for Node render workers', () => {
    delete process.env.VITE_API_BASE_URL
    process.env.API_BASE_URL = 'https://api.snaelda.io'

    expect(getAPIBaseURL()).toBe('https://api.snaelda.io')
  })
})

describe('published site response normalization', () => {
  it('rewrites legacy localhost API asset URLs in published content', () => {
    const site = normalizePublishedSiteResponse({
      siteSlug: 'slottskogen-coffee-wagon',
      hostname: 'slottskogen-coffee-wagon.snaelda.io',
      publicUrl: 'https://slottskogen-coffee-wagon.snaelda.io',
      version: {
        id: 'version-id',
        siteId: 'site-id',
        versionNumber: 1,
        createdAt: '2026-05-27T00:00:00Z',
        isCurrent: true,
        publishNote: '',
      },
      pagePath: '/',
      page: {
        pagePath: '/',
        title: 'Coffee wagon',
        description: 'Coffee in Slottskogen',
        canonicalUrl: 'https://slottskogen-coffee-wagon.snaelda.io/',
        ogImageUrl:
          'http://localhost:8080/api/public/sites/slottskogen-coffee-wagon/assets/hero.jpg',
        localBusinessJsonLd: {
          image: [
            'http://localhost:8080/api/public/sites/slottskogen-coffee-wagon/assets/hero.jpg',
          ],
        },
        html:
          '<img src="http://localhost:8080/api/public/sites/slottskogen-coffee-wagon/assets/hero.jpg">',
      },
    })

    expect(site.page.html).toContain('https://api.snaelda.io/api/public/sites/')
    expect(site.page.ogImageUrl).toBe(
      'https://api.snaelda.io/api/public/sites/slottskogen-coffee-wagon/assets/hero.jpg',
    )
    expect(site.page.localBusinessJsonLd).toEqual({
      image: [
        'https://api.snaelda.io/api/public/sites/slottskogen-coffee-wagon/assets/hero.jpg',
      ],
    })
  })
})
