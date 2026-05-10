/// <reference types="vite/client" />
import {
  HeadContent,
  Link,
  Scripts,
  createRootRoute,
} from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { DefaultCatchBoundary } from '~/components/DefaultCatchBoundary'
import { NotFound } from '~/components/NotFound'
import { topbar } from '~/lib/styles'
import appCss from '~/styles/app.css?url'

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1, viewport-fit=cover' },
      { title: 'Snaelda' },
      {
        name: 'description',
        content: 'Structured website drafts, editing, preview, and publishing.',
      },
    ],
    links: [
      { rel: 'stylesheet', href: appCss },
      { rel: 'icon', href: '/logo.png', type: 'image/png' },
    ],
  }),
  errorComponent: DefaultCatchBoundary,
  notFoundComponent: () => <NotFound />,
  shellComponent: RootDocument,
})

function RootDocument({ children }: { children: ReactNode }) {
  useEffect(() => {
    const storedMode = window.localStorage.getItem('snaelda-color-mode')
    const nextMode = storedMode === 'light' ? 'light' : 'dark'
    document.documentElement.classList.toggle('dark', nextMode === 'dark')
  }, [])

  function toggleColorMode() {
    const nextMode = document.documentElement.classList.contains('dark')
      ? 'light'
      : 'dark'
    document.documentElement.classList.toggle('dark', nextMode === 'dark')
    window.localStorage.setItem('snaelda-color-mode', nextMode)
  }

  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <head>
        <HeadContent />
      </head>
      <body>
        <header className={topbar.shell}>
          <Link to="/" className={topbar.brand} activeOptions={{ exact: true }}>
            <img
              src="/logo.png"
              alt=""
              className="size-8 rounded-full object-contain"
            />
            Snaelda
          </Link>
          <div className={topbar.controls}>
            <nav aria-label="Primary navigation" className={topbar.nav}>
              <Link
                to="/login"
                search={{ redirect: '/app' }}
                className={topbar.link}
              >
                Sign in
              </Link>
              <Link to="/app" className={topbar.link}>
                Builder
              </Link>
              <Link
                to="/preview/$token"
                params={{ token: 'local' }}
                className={topbar.link}
              >
                Preview
              </Link>
              <Link
                to="/public/$siteSlug"
                params={{ siteSlug: 'local' }}
                className={topbar.link}
              >
                Public
              </Link>
            </nav>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="max-sm:min-h-11"
              onClick={toggleColorMode}
            >
              Toggle color mode
            </Button>
          </div>
        </header>
        {children}
        <Scripts />
      </body>
    </html>
  )
}
