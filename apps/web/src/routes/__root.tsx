/// <reference types="vite/client" />
import {
  HeadContent,
  Link,
  Scripts,
  createRootRoute,
} from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import type { ReactNode } from 'react'
import { DefaultCatchBoundary } from '~/components/DefaultCatchBoundary'
import { NotFound } from '~/components/NotFound'
import appCss from '~/styles/app.css?url'

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      { title: 'Snaelda' },
      {
        name: 'description',
        content: 'Structured website drafts, editing, preview, and publishing.',
      },
    ],
    links: [{ rel: 'stylesheet', href: appCss }],
  }),
  errorComponent: DefaultCatchBoundary,
  notFoundComponent: () => <NotFound />,
  shellComponent: RootDocument,
})

function RootDocument({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <HeadContent />
      </head>
      <body>
        <header className="topbar">
          <Link to="/" className="brand" activeOptions={{ exact: true }}>
            Snaelda
          </Link>
          <nav aria-label="Primary navigation">
            <Link to="/login" search={{ redirect: '/app' }}>
              Sign in
            </Link>
            <Link to="/app">Builder</Link>
            <Link to="/preview/$token" params={{ token: 'local' }}>
              Preview
            </Link>
            <Link to="/public/$siteSlug" params={{ siteSlug: 'local' }}>
              Public
            </Link>
          </nav>
        </header>
        {children}
        <TanStackRouterDevtools position="bottom-right" />
        <Scripts />
      </body>
    </html>
  )
}
