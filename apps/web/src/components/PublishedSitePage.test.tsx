import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PublishedSiteResponse } from '@/lib/api'

const { getPublishedSiteMock } = vi.hoisted(() => ({
  getPublishedSiteMock: vi.fn(),
}))

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ...actual,
    getPublishedSite: getPublishedSiteMock,
  }
})

import { APIError } from '@/lib/api'
import { PublishedSitePage } from './PublishedSitePage'

describe('PublishedSitePage', () => {
  beforeEach(() => {
    getPublishedSiteMock.mockReset()
  })

  it('loads and renders a published snapshot page', async () => {
    getPublishedSiteMock.mockResolvedValueOnce(buildPublishedSiteResponse())

    render(<PublishedSitePage siteSlug="loom-light" pagePath="/contact" />)

    expect(screen.getByText('Loading published page...')).toBeTruthy()

    await waitFor(() => {
      expect(getPublishedSiteMock).toHaveBeenCalledWith('loom-light', '/contact')
      expect(screen.getByText('/contact')).toBeTruthy()
      expect(screen.getByText('loom-light.localhost')).toBeTruthy()
      expect(screen.getByText('Drop by the shop')).toBeTruthy()
    })
  })

  it('shows an API error message when the published page request fails', async () => {
    getPublishedSiteMock.mockRejectedValueOnce(
      new APIError(404, { message: 'Published page not found' }),
    )

    render(<PublishedSitePage siteSlug="missing-site" pagePath="/" />)

    await waitFor(() => {
      expect(screen.getByText('Published page not found')).toBeTruthy()
    })
  })
})

function buildPublishedSiteResponse(): PublishedSiteResponse {
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
      blocks: [
        {
          id: 'block-contact',
          type: 'text_section',
          version: 'text_section.v1',
          props: {
            heading: 'Drop by the shop',
            body: 'Open weekdays and most Saturdays.',
          },
          settings: {},
        },
      ],
    },
    snapshot: {
      schemaVersion: 'site_snapshot.v1',
      site: {
        id: 'site-1',
        name: 'Loom & Light',
        defaultLocale: 'en',
        seo: {
          description: 'Warm, small-business websites spun up from a prompt.',
        },
      },
      theme: {
        version: 'theme.v1',
        tokens: {
          colors: {
            background: '#151215',
            foreground: '#f3ead8',
            surface: '#231c24',
            surfaceMuted: '#302333',
            primary: '#8ee2d1',
            secondary: '#8fc6ff',
            accent: '#ff8cad',
            border: '#5a3e57',
            muted: '#caa778',
            ring: '#f3b547',
          },
          typography: {
            headingFont: 'Iowan Old Style',
            bodyFont: 'Avenir Next',
          },
          layout: {
            sectionSpacing: '96px',
            contentWidth: '720px',
          },
          shape: {
            radius: '28px',
          },
        },
      },
      navigation: {
        primary: [
          {
            label: 'Start',
            href: '/',
          },
          {
            label: 'Contact',
            href: '/contact',
          },
        ],
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
          blocks: [
            {
              id: 'block-contact',
              type: 'text_section',
              version: 'text_section.v1',
              props: {
                heading: 'Drop by the shop',
                body: 'Open weekdays and most Saturdays.',
              },
              settings: {},
            },
          ],
        },
      ],
    },
  }
}
