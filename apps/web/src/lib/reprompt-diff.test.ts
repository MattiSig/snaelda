import { describe, expect, it } from 'vitest'

import type { DraftRevisionRecord, RepromptHistoryRecord, SiteDraft } from './api'
import { buildRepromptDiff } from './reprompt-diff'

function buildDraft(overrides: Partial<SiteDraft>): SiteDraft {
  return {
    site: {
      id: 'site-1',
      name: 'North Light Studio',
      slug: 'north-light-studio',
      status: 'draft',
    },
    brand: {
      businessName: 'North Light Studio',
      primaryColor: '#F4A261',
    },
    theme: {
      version: 'theme.v1',
      tokens: {
        colors: { primary: '#F4A261' },
        typography: {},
        layout: {},
        shape: {},
      },
    },
    navigation: { primary: [] },
    pages: [],
    ...overrides,
  }
}

function buildRevision(id: string, draft: SiteDraft): DraftRevisionRecord {
  return {
    id,
    scope: 'page',
    targetId: 'page-1',
    prompt: 'Tighten the pricing page.',
    draft,
    createdAt: '2026-05-23T10:30:00Z',
  }
}

describe('buildRepromptDiff', () => {
  it('highlights changed fields for a page-scoped reprompt', () => {
    const previous = buildRevision(
      'revision-1',
      buildDraft({
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
                  headline: 'Flexible packages',
                  subheadline: 'Pick the pace that fits your project.',
                },
              },
            ],
          },
        ],
      }),
    )
    const result = buildRevision(
      'revision-2',
      buildDraft({
        pages: [
          {
            id: 'page-1',
            title: 'Pricing',
            slug: '/pricing',
            blocks: [
              {
                id: 'block-2',
                type: 'hero',
                version: 'block.v1',
                props: {
                  headline: 'Clear packages, clearer outcomes',
                  subheadline: 'Short engagements, tidy pricing, no surprises.',
                },
              },
            ],
          },
        ],
      }),
    )
    const reprompt: RepromptHistoryRecord = {
      id: 'reprompt-1',
      scope: 'page',
      targetId: 'page-1',
      prompt: 'Tighten the pricing page.',
      previousRevisionId: previous.id,
      resultRevisionId: result.id,
      createdAt: '2026-05-23T10:30:00Z',
    }

    const diff = buildRepromptDiff(reprompt, previous, result)

    expect(diff).toHaveLength(1)
    expect(diff[0].blocks).toHaveLength(1)
    expect(diff[0].blocks[0].status).toBe('modified')
    expect(diff[0].blocks[0].fields).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: 'headline',
          before: 'Flexible packages',
          after: 'Clear packages, clearer outcomes',
        }),
      ]),
    )
  })

  it('includes added and removed blocks for site-wide reprompts', () => {
    const previous = {
      ...buildRevision(
        'revision-1',
        buildDraft({
          pages: [
            {
              id: 'page-1',
              title: 'Home',
              slug: '/',
              blocks: [
                {
                  id: 'block-1',
                  type: 'hero',
                  version: 'block.v1',
                  props: { headline: 'Welcome' },
                },
              ],
            },
          ],
        }),
      ),
      scope: 'site' as const,
      targetId: undefined,
    }
    const result = {
      ...buildRevision(
        'revision-2',
        buildDraft({
          pages: [
            {
              id: 'page-2',
              title: 'Home',
              slug: '/',
              blocks: [
                {
                  id: 'block-2',
                  type: 'hero',
                  version: 'block.v1',
                  props: { headline: 'Welcome' },
                },
                {
                  id: 'block-3',
                  type: 'faq',
                  version: 'block.v1',
                  props: { heading: 'Questions' },
                },
              ],
            },
          ],
        }),
      ),
      scope: 'site' as const,
      targetId: undefined,
    }
    const reprompt: RepromptHistoryRecord = {
      id: 'reprompt-2',
      scope: 'site',
      prompt: 'Add an FAQ section.',
      previousRevisionId: previous.id,
      resultRevisionId: result.id,
      createdAt: '2026-05-23T11:00:00Z',
    }

    const diff = buildRepromptDiff(reprompt, previous, result)

    expect(diff).toHaveLength(1)
    expect(diff[0].blocks[0].status).toBe('added')
    expect(diff[0].blocks[0].afterBlock?.type).toBe('faq')
  })
})
