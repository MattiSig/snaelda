import { Outlet, createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/public/$siteSlug')({
  component: PublicSite,
})

function PublicSite() {
  return <Outlet />
}
