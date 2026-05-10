import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import contractFixture from '../../../../internal/siteconfig/testdata/block_registry_contract.json'
import type { BlockDefinition, SiteDraft } from '@/lib/api'
import { BlockEditor } from './BlockEditor'
import { SiteDraftRenderer } from './SiteDraftRenderer'

const fixture = contractFixture as {
  blockRegistry: BlockDefinition[]
  draft: SiteDraft
}

const fixtureBlocks = new Map(
  (fixture.draft.pages[0]?.blocks ?? []).map((block) => [
    `${block.type}@${block.version}`,
    block,
  ]),
)

afterEach(() => {
  cleanup()
})

describe('Go block registry contract', () => {
  it('renders every Go-defined prototype block without fallback UI', () => {
    render(<SiteDraftRenderer site={fixture.draft} />)

    expect(screen.queryByText('Unsupported block')).toBeNull()

    for (const definition of fixture.blockRegistry) {
      expect(fixtureBlocks.has(`${definition.type}@${definition.version}`)).toBe(true)
    }
  })

  it.each(
    fixture.blockRegistry.map((definition) => [
      `${definition.type}@${definition.version}`,
      definition,
    ] as const),
  )('renders editor controls for %s', (_key, definition) => {
    const block = fixtureBlocks.get(`${definition.type}@${definition.version}`)
    if (!block) {
      throw new Error(`Missing fixture block for ${definition.type}@${definition.version}`)
    }

    render(
      <BlockEditor
        block={block}
        definition={definition}
        isSaving={false}
        errorMessage=""
        statusMessage=""
        onSave={vi.fn()}
      />,
    )

    expect(screen.getByRole('heading', { name: definition.displayName })).toBeTruthy()
    expect(
      screen.queryByText('This block does not expose editable fields yet.'),
    ).toBeNull()

    for (const field of definition.editorSchema ?? []) {
      expect(screen.getAllByText(field.label).length).toBeGreaterThan(0)
    }
  })
})
