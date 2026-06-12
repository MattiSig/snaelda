import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type {
  DraftRevisionRecord,
  RepromptHistoryRecord,
  SiteDraft,
} from '@/lib/api'
import { RevisionDiffModal } from './RevisionDiffModal'

describe('RevisionDiffModal', () => {
  it('cycles between both, before, and after views from the keyboard', async () => {
    const onClose = vi.fn()

    render(
      <RevisionDiffModal
        reprompt={buildReprompt()}
        previousRevision={buildRevision('revision-1', 'Flexible packages')}
        resultRevision={buildRevision(
          'revision-2',
          'Clear packages, clearer outcomes',
        )}
        onClose={onClose}
      />,
    )

    expect(screen.getByTestId('revision-preview-before')).toBeTruthy()
    expect(screen.getByTestId('revision-preview-after')).toBeTruthy()

    fireEvent.keyDown(document, { key: 'Enter' })
    expect(screen.getByTestId('revision-preview-before')).toBeTruthy()
    expect(screen.queryByTestId('revision-preview-after')).toBeNull()

    fireEvent.keyDown(document, { key: 'Enter' })
    expect(screen.queryByTestId('revision-preview-before')).toBeNull()
    expect(screen.getByTestId('revision-preview-after')).toBeTruthy()

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})

function buildReprompt(): RepromptHistoryRecord {
  return {
    id: 'reprompt-1',
    scope: 'page',
    targetId: 'page-1',
    prompt: 'Tighten the pricing page.',
    changeSummary: 'Refined the hero copy.',
    previousRevisionId: 'revision-1',
    resultRevisionId: 'revision-2',
    createdAt: '2026-05-23T10:30:00Z',
  }
}

function buildRevision(id: string, headline: string): DraftRevisionRecord {
  return {
    id,
    scope: 'page',
    targetId: 'page-1',
    prompt: 'Tighten the pricing page.',
    createdAt: '2026-05-23T10:30:00Z',
    draft: buildDraft(headline),
  }
}

function buildDraft(headline: string): SiteDraft {
  return {
    site: {
      id: 'site-1',
      name: 'North Light Studio',
      slug: 'north-light-studio',
      status: 'draft',
      seo: {
        description: 'Friendly project packages',
      },
    },
    brand: {
      businessName: 'North Light Studio',
      primaryColor: '#F4A261',
    },
    theme: {
      version: 'theme.v1',
      tokens: {
        colors: {
          background: '#f9f7f2',
          text: '#2d1b2d',
          surface: '#fffaf1',
          primary: '#f4a261',
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
        id: 'page-1',
        title: 'Pricing',
        slug: '/pricing',
        blocks: [
          {
            id: 'block-1',
            type: 'hero',
            version: 'block.v1',
            props: {
              headline,
              subheadline: 'Pick the pace that fits your project.',
            },
          },
        ],
      },
    ],
  }
}
