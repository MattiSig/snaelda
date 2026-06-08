import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  getAPIBaseURL,
  normalizePublishedSiteResponse,
  resolveRuntimeAPIBaseURL,
  startAnonymousSession,
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

describe('anonymous session', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('can request a fresh session when the current trial is blocked', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({
        session: {
          kind: 'trial',
          workspaceId: 'workspace-1',
          workspaceRole: 'owner',
        },
      }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    await startAnonymousSession({ freshIfBlocked: true })

    expect(fetchMock).toHaveBeenCalledOnce()
    expect(String(fetchMock.mock.calls[0][0])).toContain(
      '/api/sessions/anonymous?freshIfBlocked=true',
    )
  })
})

describe('generation stream recovery', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('polls the persisted job after the SSE connection drops', async () => {
    const encoder = new TextEncoder()
    let pullCount = 0
    const stream = new ReadableStream<Uint8Array>({
      pull(controller) {
        pullCount += 1
        if (pullCount === 1) {
          controller.enqueue(
            encoder.encode('event: job\ndata: {"jobId":"job-1"}\n\n'),
          )
          return
        }
        controller.error(new Error('connection terminated'))
      },
    })
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(stream, {
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
      }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        job: {
          id: 'job-1',
          kind: 'site',
          state: 'succeeded',
          siteId: 'site-1',
        },
      }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }))

    const result = await import('./api').then(({ streamGenerateSite }) =>
      streamGenerateSite({ prompt: 'A neighborhood bicycle repair shop' }),
    )

    expect(result).toEqual({
      jobId: 'job-1',
      siteId: 'site-1',
      draftId: 'site-1',
    })
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(String(fetchMock.mock.calls[1][0])).toContain(
      '/api/generation/jobs/job-1',
    )
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
