import { Outlet, createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/app/sites/$siteId')({
  component: SiteLayout,
})

function SiteLayout() {
  return <Outlet />
}
