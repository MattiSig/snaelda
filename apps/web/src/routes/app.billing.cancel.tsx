import { Link, createFileRoute } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/app/billing/cancel')({
  component: BillingCancelPage,
})

function BillingCancelPage() {
  return (
    <section className={cn(paddedPanel, 'rounded-[14px]')}>
      <p className={text.eyebrow}>Billing</p>
      <h1 className={cn(text.sectionTitle, 'mt-2')}>Checkout was not completed</h1>
      <p className={cn(text.p, 'mt-3')}>
        Nothing changed in the workspace. You can return to the builder or reopen checkout when
        you are ready.
      </p>
      <div className="mt-5 flex flex-wrap gap-3">
        <Button asChild>
          <Link to="/app/billing">Return to billing</Link>
        </Button>
        <Button asChild variant="outline">
          <Link to="/app">Back to sites</Link>
        </Button>
      </div>
    </section>
  )
}
