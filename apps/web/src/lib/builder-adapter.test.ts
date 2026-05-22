import { describe, expect, it } from 'vitest'
import type { SiteDraft } from '@/lib/api'
import {
  buildCanonicalBlockOrder,
  draftToEditorCanvasPage,
  reorderEditorCanvasBlocks,
} from './builder-adapter'

describe('builder adapter', () => {
  it('splits visible and hidden blocks for the canvas', () => {
    const editorPage = draftToEditorCanvasPage(buildDraft(), 'page-home')
    expect(editorPage?.visibleBlocks.map(({ block }) => block.id)).toEqual([
      'block-hero',
      'block-cta',
    ])
    expect(editorPage?.hiddenBlocks.map(({ block }) => block.id)).toEqual([
      'block-hidden',
    ])
  })

  it('builds canonical reorder payloads that preserve hidden blocks off-canvas', () => {
    const editorPage = draftToEditorCanvasPage(buildDraft(), 'page-home')
    expect(editorPage).toBeTruthy()

    const visibleIDs = reorderEditorCanvasBlocks(
      editorPage!.visibleBlocks,
      'block-cta',
      0,
    )

    expect(buildCanonicalBlockOrder(editorPage!, visibleIDs ?? [])).toEqual([
      'block-cta',
      'block-hero',
      'block-hidden',
    ])
  })
})

function buildDraft(): SiteDraft {
  return {
    site: {
      id: 'site-1',
      name: 'Loom & Light',
      slug: 'loom-light',
      status: 'draft',
    },
    brand: {
      businessName: 'Loom & Light',
      primaryColor: '#8ee2d1',
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
          sectionPaddingX: '24px',
          sectionPaddingY: '96px',
          contentWidth: '720px',
        },
        shape: {
          radius: '28px',
        },
      },
    },
    navigation: {
      primary: [{ label: 'Home', pageId: 'page-home' }],
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
            version: '1.0.0',
            props: {
              headline: 'Publish a proper site before lunch.',
            },
          },
          {
            id: 'block-hidden',
            type: 'text_section',
            version: '1.0.0',
            props: {
              heading: 'Hidden note',
            },
            settings: {
              hidden: true,
            },
          },
          {
            id: 'block-cta',
            type: 'cta_band',
            version: '1.0.0',
            props: {
              heading: 'Ready to ask a question?',
            },
          },
        ],
      },
    ],
  }
}
