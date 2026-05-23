import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { GenerationProgressCard, type GenerationProgressItem } from './GenerationProgressCard'

const steps: GenerationProgressItem[] = [
  { step: 'prompt.normalize', label: 'Reading your prompt' },
  { step: 'plan.pages', label: 'Planning pages and structure' },
  { step: 'plan.theme', label: 'Picking colors and typography' },
  { step: 'plan.blocks', label: 'Choosing blocks for each page' },
  { step: 'assets.fetch', label: 'Finding starter imagery' },
  { step: 'copy.write', label: 'Writing copy' },
  { step: 'validate.repair', label: 'Checking and repairing' },
  { step: 'persist', label: 'Saving your draft' },
]

describe('GenerationProgressCard', () => {
  it('hides optional starter imagery when the stream reports a shorter step set', () => {
    render(
      <GenerationProgressCard
        eyebrow="New site"
        title="Weaving your draft..."
        description="Snaelda is generating your first draft."
        steps={steps}
        activeStep="copy.write"
        activeTotal={7}
      />,
    )

    expect(screen.queryByText('Finding starter imagery')).toBeNull()
    expect(screen.getByText('Writing copy')).toBeTruthy()
  })
})
