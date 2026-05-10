import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { SiteDraft } from '@/lib/api'
import { SiteDraftRenderer } from './SiteDraftRenderer'

describe('SiteDraftRenderer', () => {
  it('renders visible blocks and resolves draft-page links to anchors', () => {
    render(<SiteDraftRenderer site={buildDraft()} />)

    const navLinks = screen.getAllByRole('link')
    expect(navLinks[0]?.getAttribute('href')).toBe('#page-home')
    expect(navLinks[1]?.getAttribute('href')).toBe('#page-contact')
    expect(screen.getByRole('link', { name: 'Book a visit' }).getAttribute('href')).toBe(
      '#page-contact',
    )
    expect(screen.getByText('Friendly yarn for colder days.')).toBeTruthy()
    expect(screen.queryByText('This hidden block should never render.')).toBeNull()
  })

  it('resolves published links and narrows rendering to the selected page', () => {
    render(
      <SiteDraftRenderer
        site={buildDraft()}
        linkMode="published"
        selectedPageId="page-contact"
        showPageMeta={false}
        siteSlug="loom-light"
      />,
    )

    const navLinks = screen.getAllByRole('link')
    expect(navLinks[0]?.getAttribute('href')).toBe('/public/loom-light')
    expect(navLinks[1]?.getAttribute('href')).toBe('/public/loom-light/contact')
    expect(screen.queryByText('Home')).toBeNull()
    expect(screen.getByText('Contact')).toBeTruthy()
  })
})

function buildDraft(): SiteDraft {
  return {
    site: {
      id: 'site-1',
      name: 'Loom & Light',
      slug: 'loom-light',
      status: 'draft',
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
          pageId: 'page-home',
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
        blocks: [
          {
            id: 'block-hero',
            type: 'hero',
            version: 'hero.v1',
            props: {
              eyebrow: 'Prompt-built sites',
              headline: 'Publish a proper site before lunch.',
              subheadline: 'Friendly yarn for colder days.',
              primaryCta: {
                label: 'Book a visit',
                href: '/contact',
              },
            },
            settings: {},
          },
          {
            id: 'block-hidden',
            type: 'text_section',
            version: 'text_section.v1',
            props: {
              heading: 'Hidden note',
              body: 'This hidden block should never render.',
            },
            settings: {
              hidden: true,
            },
          },
        ],
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
  }
}
