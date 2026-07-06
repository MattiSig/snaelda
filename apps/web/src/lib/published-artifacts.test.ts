import { describe, expect, it } from 'vitest'
import type { PublishedSnapshot, SiteVersion } from '@/lib/api'
import {
  buildPublishedArtifactBundle,
  type PublishedArtifactRenderInput,
} from './published-artifacts'

const baseVersion: SiteVersion = {
  id: '00000000-0000-4000-8000-000000000701',
  siteId: '00000000-0000-4000-8000-000000000201',
  versionNumber: 1,
  createdAt: '2026-05-21T00:00:00Z',
  isCurrent: true,
}

function buildBaseInput(
  snapshot: PublishedSnapshot,
): PublishedArtifactRenderInput {
  return {
    publicBaseURL: 'https://example.com/',
    siteSlug: 'demo',
    hostname: 'demo.example.com',
    version: baseVersion,
    snapshot,
  }
}

function buildSnapshot(): PublishedSnapshot {
  return {
    schemaVersion: 'site-config.v1',
    site: {
      id: 'site_demo',
      name: 'Demo Studio',
      defaultLocale: 'en',
      seo: {
        title: 'Demo Studio',
        description: 'Demo studio for spec-19 testing.',
      },
    },
    brand: {
      businessName: 'Demo Studio',
      primaryColor: '#8ee2d1',
    },
    theme: {
      version: 'theme.v1',
      tokens: {
        colors: {
          background: '#151215',
          foreground: '#f3ead8',
          primary: '#8ee2d1',
        },
        typography: { headingFont: 'Iowan Old Style', bodyFont: 'Avenir Next' },
        layout: { sectionPaddingY: '96px' },
        shape: { radius: '14px' },
      },
    },
    navigation: {
      primary: [
        { label: 'Home', pageId: 'page_home' },
        { label: 'Draft page', pageId: 'page_draft' },
      ],
    },
    collections: [
      {
        id: 'col_services',
        slug: 'services',
        singularLabel: 'Service',
        pluralLabel: 'Services',
        sortOrder: 0,
        schema: [
          { key: 'title', label: 'Title', type: 'text', required: true },
          { key: 'summary', label: 'Summary', type: 'long_text' },
          { key: 'details', label: 'Details', type: 'rich_text' },
        ],
        entries: [
          {
            id: 'entry_scaffolding',
            slug: 'scaffolding',
            fields: {
              title: 'Scaffolding rentals',
              summary: 'Heavy-duty scaffolding for facade projects.',
              details:
                'We build, inspect, and dismantle scaffolding for builds across the region.',
            },
            seo: {
              title: 'Scaffolding rentals',
              description: 'Heavy-duty scaffolding for facade projects.',
            },
            status: 'published',
            sortOrder: 0,
          },
          {
            id: 'entry_painting',
            slug: 'painting',
            fields: {
              title: 'Facade painting',
              summary: 'Touch up wood and stucco facades.',
              details: 'Crew with deep experience repainting exterior trim.',
            },
            seo: {
              title: 'Facade painting',
              description: 'Touch up wood and stucco facades.',
            },
            status: 'published',
            sortOrder: 1,
          },
          {
            id: 'entry_draft',
            slug: 'draft-only',
            fields: { title: 'Draft only', summary: 'Should not publish.' },
            status: 'draft',
            sortOrder: 2,
          },
        ],
      },
    ],
    pages: [
      {
        id: 'page_home',
        title: 'Home',
        slug: '/',
        type: 'static',
        seo: { title: 'Home', description: 'Welcome to Demo Studio.' },
        blocks: [
          {
            id: 'block_home_list',
            type: 'collection_list',
            version: '1.0.0',
            props: {
              heading: 'Featured services',
              collection: 'col_services',
              limit: 6,
              layout: 'grid',
            },
            settings: {},
          },
        ],
      },
      {
        id: 'page_service_detail',
        title: 'Service detail',
        slug: '/service-template',
        type: 'collection_detail',
        collectionId: 'col_services',
        seo: { title: 'Service detail', description: 'Detail template.' },
        blocks: [
          {
            id: 'block_detail_hero',
            type: 'hero',
            version: '1.0.0',
            props: {
              headline: 'Default headline',
              subheadline: 'Default subheadline',
              layout: 'centered',
            },
            bindings: {
              headline: { source: 'entry', field: 'title' },
              subheadline: { source: 'entry', field: 'summary' },
            },
            settings: {},
          },
          {
            id: 'block_detail_body',
            type: 'collection_detail',
            version: '1.0.0',
            props: { layout: 'default' },
            settings: {},
          },
        ],
      },
      {
        id: 'page_draft',
        title: 'Draft page',
        slug: '/draft-only-page',
        status: 'draft',
        type: 'static',
        seo: { title: 'Draft page', description: 'Should stay out of publish.' },
        blocks: [
          {
            id: 'block_draft',
            type: 'text_section',
            version: '1.0.0',
            props: {
              heading: 'Draft only',
              body: 'This page should not render publicly.',
            },
            settings: {},
          },
        ],
      },
    ],
  }
}

describe('buildPublishedArtifactBundle', () => {
  it('emits one HTML page per published entry under the collection slug', () => {
    const bundle = buildPublishedArtifactBundle(buildBaseInput(buildSnapshot()))
    const filesByPath = new Map(bundle.files.map((file) => [file.path, file]))

    expect(filesByPath.has('pages/index.html')).toBe(true)
    expect(filesByPath.has('pages/services/scaffolding/index.html')).toBe(true)
    expect(filesByPath.has('pages/services/painting/index.html')).toBe(true)
    expect(filesByPath.has('pages/services/draft-only/index.html')).toBe(false)
    expect(filesByPath.has('pages/service-template/index.html')).toBe(false)
  })

  it('applies block bindings to substitute entry field values into rendered HTML', () => {
    const bundle = buildPublishedArtifactBundle(buildBaseInput(buildSnapshot()))
    const scaffoldingFile = bundle.files.find(
      (file) => file.path === 'pages/services/scaffolding/index.html',
    )
    expect(scaffoldingFile).toBeDefined()
    expect(scaffoldingFile?.body).toContain('Scaffolding rentals')
    expect(scaffoldingFile?.body).toContain(
      'Heavy-duty scaffolding for facade projects.',
    )
    // Default literal prop value must NOT appear once the binding overrides it.
    expect(scaffoldingFile?.body).not.toContain('Default headline')
  })

  it('renders the collection_list block on a static page with links to entry URLs', () => {
    const bundle = buildPublishedArtifactBundle(buildBaseInput(buildSnapshot()))
    const homeFile = bundle.files.find(
      (file) => file.path === 'pages/index.html',
    )
    expect(homeFile).toBeDefined()
    expect(homeFile?.body).toContain('Featured services')
    expect(homeFile?.body).toContain('/services/scaffolding')
    expect(homeFile?.body).toContain('/services/painting')
    expect(homeFile?.body).toContain('Scaffolding rentals')
  })

  it('records the site default locale in the manifest', () => {
    const snapshot = buildSnapshot()
    snapshot.site.defaultLocale = 'is-IS'
    const bundle = buildPublishedArtifactBundle(buildBaseInput(snapshot))
    const manifestFile = bundle.files.find(
      (file) => file.path === 'manifest.json',
    )
    const manifest = JSON.parse(manifestFile!.body) as {
      defaultLocale?: string
    }
    expect(manifest.defaultLocale).toBe('is')
  })

  it('falls back to an Icelandic meta description for Icelandic sites', () => {
    const snapshot = buildSnapshot()
    snapshot.site.defaultLocale = 'is'
    // Strip every description source so the locale fallback is exercised.
    snapshot.site.seo = { title: 'Norðurljós', description: '' }
    snapshot.pages = snapshot.pages.map((page) =>
      page.slug === '/'
        ? { ...page, seo: { title: 'Heim', description: '' } }
        : page,
    )
    const bundle = buildPublishedArtifactBundle(buildBaseInput(snapshot))
    const manifestFile = bundle.files.find(
      (file) => file.path === 'manifest.json',
    )
    const manifest = JSON.parse(manifestFile!.body) as {
      pages: Array<{ pagePath: string; description: string }>
    }
    const home = manifest.pages.find((page) => page.pagePath === '/')
    expect(home?.description).toBe('Heimsæktu Demo Studio.')
  })

  it('lists every entry URL in the manifest and sitemap', () => {
    const bundle = buildPublishedArtifactBundle(buildBaseInput(buildSnapshot()))
    const manifestFile = bundle.files.find(
      (file) => file.path === 'manifest.json',
    )
    expect(manifestFile).toBeDefined()
    const manifest = JSON.parse(manifestFile!.body) as {
      pages: Array<{ pagePath: string; canonicalUrl: string }>
      files: string[]
    }
    const paths = manifest.pages.map((page) => page.pagePath)
    expect(paths).toContain('/')
    expect(paths).toContain('/services/scaffolding')
    expect(paths).toContain('/services/painting')
    expect(paths).not.toContain('/service-template')
    expect(paths).not.toContain('/services/draft-only')

    expect(manifest.files).toContain('pages/services/scaffolding/index.html')

    const sitemap = bundle.files.find((file) => file.path === 'sitemap.xml')
    expect(sitemap?.body).toContain('https://demo.example.com/services/scaffolding')
    expect(sitemap?.body).toContain('https://demo.example.com/services/painting')
    expect(sitemap?.body).not.toContain('https://demo.example.com/service-template')
  })

  it('uses per-entry SEO metadata for the manifest', () => {
    const bundle = buildPublishedArtifactBundle(buildBaseInput(buildSnapshot()))
    const manifestFile = bundle.files.find(
      (file) => file.path === 'manifest.json',
    )
    const manifest = JSON.parse(manifestFile!.body) as {
      pages: Array<{ pagePath: string; title: string; description: string }>
    }
    const entryPage = manifest.pages.find(
      (page) => page.pagePath === '/services/scaffolding',
    )
    expect(entryPage?.title).toBe('Scaffolding rentals')
    expect(entryPage?.description).toBe(
      'Heavy-duty scaffolding for facade projects.',
    )
  })

  it('skips draft pages and their navigation links from published artifacts', () => {
    const bundle = buildPublishedArtifactBundle(buildBaseInput(buildSnapshot()))
    const filesByPath = new Map(bundle.files.map((file) => [file.path, file]))

    expect(filesByPath.has('pages/draft-only-page/index.html')).toBe(false)

    const homeFile = filesByPath.get('pages/index.html')
    expect(homeFile?.body).not.toContain('Draft page')

    const manifestFile = filesByPath.get('manifest.json')
    const manifest = JSON.parse(manifestFile!.body) as {
      pages: Array<{ pagePath: string }>
    }
    expect(manifest.pages.map((page) => page.pagePath)).not.toContain(
      '/draft-only-page',
    )
  })
})
