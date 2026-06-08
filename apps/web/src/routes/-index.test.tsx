import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'
import type { BuilderSession } from '@/lib/api'
import { ReturningWorkspacePrompt } from './index'

vi.mock('@tanstack/react-router', async () => {
  const actual = await vi.importActual<typeof import('@tanstack/react-router')>(
    '@tanstack/react-router',
  )

  return {
    ...actual,
    Link: ({
      children,
      to,
      ...props
    }: {
      children: ReactNode
      to: string
      [key: string]: unknown
    }) => (
      <a href={to} {...props}>
        {children}
      </a>
    ),
  }
})

describe('ReturningWorkspacePrompt', () => {
  it('offers a direct return path for an active trial workspace', () => {
    const session: BuilderSession = {
      kind: 'trial',
      workspaceId: 'workspace-1',
      workspaceRole: 'owner',
    }

    render(<ReturningWorkspacePrompt session={session} />)

    expect(screen.getByText('Your workspace is waiting')).toBeTruthy()
    expect(screen.getByText('Your trial workspace')).toBeTruthy()
    expect(screen.getByRole('link', { name: /continue editing/i }).getAttribute('href')).toBe(
      '/app',
    )
  })

  it('identifies an authenticated workspace by its owner', () => {
    const session: BuilderSession = {
      kind: 'authenticated',
      workspaceId: 'workspace-2',
      workspaceRole: 'owner',
      user: {
        id: 'user-1',
        email: 'ana@example.com',
        name: 'Ana',
        workspaceId: 'workspace-2',
        workspaceRole: 'owner',
      },
    }

    render(<ReturningWorkspacePrompt session={session} />)

    expect(screen.getByText('Ana')).toBeTruthy()
  })
})
