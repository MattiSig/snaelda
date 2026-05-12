import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { BlockDefinition, SiteDraft } from '@/lib/api'
import { BlockEditor } from './BlockEditor'

type DraftBlock = SiteDraft['pages'][number]['blocks'][number]

describe('BlockEditor', () => {
  it('renders an empty state when a block has no editor schema', () => {
    render(
      <BlockEditor
        block={buildBlock()}
        isSaving={false}
        errorMessage=""
        statusMessage=""
        assetLibrary={[]}
        onSave={vi.fn()}
      />,
    )

    expect(
      screen.getByText('This block does not expose editable fields yet.'),
    ).toBeTruthy()
  })

  it('submits edited nested props and hidden state', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)

    render(
      <BlockEditor
        block={buildBlock({
          props: {
            headline: 'Original headline',
            summary: 'Original summary',
            primaryCta: {
              label: 'Book now',
              href: '/contact',
            },
            items: [],
          },
        })}
        definition={buildDefinition()}
        isSaving={false}
        errorMessage=""
        statusMessage=""
        assetLibrary={[]}
        onSave={onSave}
      />,
    )

    fireEvent.change(screen.getByLabelText('Headline'), {
      target: { value: 'Fresh headline' },
    })
    fireEvent.change(screen.getByLabelText('Summary'), {
      target: { value: 'Updated summary' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add item' }))

    const itemCard = screen.getByText('Item 1').closest('article')
    if (!itemCard) {
      throw new Error('Expected the repeater item card to render')
    }

    fireEvent.change(within(itemCard).getByLabelText('Title'), {
      target: { value: 'Fast setup' },
    })
    fireEvent.change(within(itemCard).getByLabelText('Body'), {
      target: { value: 'Launch a polished site without a long project.' },
    })
    fireEvent.click(
      screen.getByLabelText('Hide this block in preview and publish output'),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save block' }))

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(
        {
          headline: 'Fresh headline',
          summary: 'Updated summary',
          items: [
            {
              title: 'Fast setup',
              body: 'Launch a polished site without a long project.',
            },
          ],
        },
        true,
      )
    })
  })

  it('selects uploaded assets instead of raw ids', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)

    render(
      <BlockEditor
        block={buildBlock({
          props: {},
        })}
        definition={{
          type: 'image_text',
          version: '1.0.0',
          displayName: 'Image and text',
          category: 'media',
          editorSchema: [
            {
              name: 'image',
              label: 'Image',
              control: 'asset',
            },
          ],
        }}
        isSaving={false}
        errorMessage=""
        statusMessage=""
        assetLibrary={[
          {
            id: 'asset-1',
            workspaceId: 'workspace-1',
            siteId: 'site-1',
            kind: 'image',
            storageKey: 'key-1',
            altText: 'Warm studio portrait',
            metadata: {
              fileName: 'portrait.png',
              contentType: 'image/png',
            },
            createdAt: '2026-05-12T10:00:00Z',
          },
        ]}
        onSave={onSave}
      />,
    )

    fireEvent.change(screen.getByLabelText('Uploaded image'), {
      target: { value: 'asset-1' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save block' }))

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(
        {
          image: {
            assetId: 'asset-1',
            alt: 'Warm studio portrait',
          },
        },
        false,
      )
    })
  })
})

function buildBlock(overrides: Partial<DraftBlock> = {}): DraftBlock {
  return {
    id: 'block-hero',
    type: 'hero',
    version: 'hero.v1',
    props: {
      headline: 'Original headline',
    },
    settings: {
      hidden: false,
    },
    ...overrides,
  }
}

function buildDefinition(): BlockDefinition {
  return {
    type: 'hero',
    version: 'hero.v1',
    displayName: 'Hero',
    category: 'layout',
    editorSchema: [
      {
        name: 'headline',
        label: 'Headline',
        control: 'text',
      },
      {
        name: 'summary',
        label: 'Summary',
        control: 'textarea',
      },
      {
        name: 'primaryCta',
        label: 'Primary CTA',
        control: 'link',
        fields: [
          {
            name: 'label',
            label: 'Label',
            control: 'text',
          },
          {
            name: 'href',
            label: 'Link',
            control: 'text',
          },
        ],
      },
      {
        name: 'items',
        label: 'Items',
        control: 'repeater',
        itemFields: [
          {
            name: 'title',
            label: 'Title',
            control: 'text',
          },
          {
            name: 'body',
            label: 'Body',
            control: 'textarea',
          },
        ],
      },
    ],
  }
}
