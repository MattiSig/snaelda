import { Link, createFileRoute } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { paddedPanel, text } from '@/lib/styles'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/app/billing/success')({
  component: BillingSuccessPage,
})

function BillingSuccessPage() {
  return (
    <section className={cn(paddedPanel, 'rounded-[14px]')}>
      <p className={text.eyebrow}>Billing</p>
      <h1 className={cn(text.sectionTitle, 'mt-2')}>Checkout completed</h1>
      <p className={cn(text.p, 'mt-3')}>
        Stripe has handed the session back. If the builder still shows trial access, give the
        webhook a moment and refresh this page.
      </p>
      <div className="mt-5 flex flex-wrap gap-3">
        <Button asChild>
          <Link to="/app/billing">Open billing</Link>
        </Button>
        <Button asChild variant="outline">
          <Link to="/app">Back to sites</Link>
        </Button>
      </div>
    </section>
  )
}
