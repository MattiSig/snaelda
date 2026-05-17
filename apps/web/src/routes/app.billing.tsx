import { Outlet, createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/app/billing')({
  component: BillingLayout,
})

function BillingLayout() {
  return <Outlet />
}
