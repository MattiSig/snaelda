import { Link, Outlet, createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/app')({
  component: AppLayout,
})

function AppLayout() {
  return (
    <main className="app-shell">
      <aside className="app-sidebar">
        <Link to="/app">Sites</Link>
        <Link to="/app/sites/$siteId" params={{ siteId: 'draft' }}>
          Draft
        </Link>
        <Link to="/app/sites/$siteId/preview" params={{ siteId: 'draft' }}>
          Preview
        </Link>
      </aside>
      <section className="app-content">
        <Outlet />
      </section>
    </main>
  )
}
