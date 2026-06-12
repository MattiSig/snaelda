import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { RepromptHistoryRecord } from '@/lib/api'
import { RepromptHistoryPanel } from './RepromptHistoryPanel'

describe('RepromptHistoryPanel', () => {
  it('filters to the selected block scope and disables reverted entries', () => {
    const onActiveScopeChange = vi.fn()

    render(
      <RepromptHistoryPanel
        reprompts={[
          buildReprompt({
            id: 'site-1',
            scope: 'site',
            prompt: 'Make the whole site warmer.',
            changeSummary: 'Adjusted the overall tone.',
          }),
          buildReprompt({
            id: 'page-1',
            scope: 'page',
            targetId: 'page-contact',
            prompt: 'Refresh the contact page.',
            changeSummary: 'Added a stronger CTA.',
          }),
          buildReprompt({
            id: 'block-1',
            scope: 'block',
            targetId: 'block-hero',
            prompt: 'Tighten the hero copy.',
            changeSummary: 'Tightened the hero copy.',
          }),
          buildReprompt({
            id: 'block-2',
            scope: 'block',
            targetId: 'block-hero',
            prompt: 'Swap the image.',
            changeSummary: 'Replaced the hero image.',
            undoneAt: '2026-05-23T12:30:00Z',
          }),
        ]}
        activeScope="block"
        selectedPageId="page-contact"
        selectedPageTitle="Contact"
        selectedBlockId="block-hero"
        selectedBlockLabel="Hero block"
        onActiveScopeChange={onActiveScopeChange}
        onShowDiff={vi.fn()}
        onRevert={vi.fn()}
      />,
    )

    expect(screen.getByText('Hero block')).toBeTruthy()
    expect(screen.queryByText('Adjusted the overall tone.')).toBeNull()
    expect(screen.queryByText('Added a stronger CTA.')).toBeNull()
    expect(screen.getByText('Tightened the hero copy.')).toBeTruthy()
    expect(screen.getByText('Replaced the hero image.')).toBeTruthy()

    const restoredButton = screen.getByRole('button', { name: 'Restored' })
    expect(restoredButton.getAttribute('disabled')).not.toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Whole site' }))
    expect(onActiveScopeChange).toHaveBeenCalledWith('site')
  })

  it('surfaces every scope under the whole-site view, including entry and theme records', () => {
    render(
      <RepromptHistoryPanel
        reprompts={[
          buildReprompt({
            id: 'site-1',
            scope: 'site',
            changeSummary: 'Rebuilt the whole site.',
          }),
          buildReprompt({
            id: 'page-1',
            scope: 'page',
            targetId: 'page-pricing',
            changeSummary: 'Refreshed the pricing page.',
          }),
          buildReprompt({
            id: 'block-1',
            scope: 'block',
            targetId: 'block-hero',
            changeSummary: 'Tightened the hero copy.',
          }),
          buildReprompt({
            id: 'entry-1',
            scope: 'entry',
            targetId: 'entry-team-erin',
            changeSummary: 'Refreshed Erin’s bio.',
          }),
          buildReprompt({
            id: 'theme-1',
            scope: 'theme',
            changeSummary: 'Warmer palette and tighter type.',
          }),
        ]}
        activeScope="site"
        onActiveScopeChange={vi.fn()}
        onShowDiff={vi.fn()}
        onRevert={vi.fn()}
      />,
    )

    expect(screen.getByText('Rebuilt the whole site.')).toBeTruthy()
    expect(screen.getByText('Refreshed the pricing page.')).toBeTruthy()
    expect(screen.getByText('Tightened the hero copy.')).toBeTruthy()
    expect(screen.getByText('Refreshed Erin’s bio.')).toBeTruthy()
    expect(screen.getByText('Warmer palette and tighter type.')).toBeTruthy()
  })
})

function buildReprompt(
  overrides: Partial<RepromptHistoryRecord>,
): RepromptHistoryRecord {
  return {
    id: 'reprompt-1',
    scope: 'site',
    prompt: 'Refresh the copy.',
    previousRevisionId: 'revision-1',
    resultRevisionId: 'revision-2',
    createdAt: '2026-05-23T12:00:00Z',
    ...overrides,
  }
}
