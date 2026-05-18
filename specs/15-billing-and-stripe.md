# Billing and Stripe

## Scope

Billing is required for MVP. Snaelda uses Stripe for platform billing: subscriptions, invoices, payment collection, customer portal access, and entitlement enforcement for Snaelda accounts. Without it the trial-to-paid transition described below has no exit, so it ships alongside the core create/edit/publish loop rather than after it.

This is separate from e-commerce checkout inside generated customer websites. Customer storefront payments remain out of scope until the core website builder and hosting product is stable.

## Billing Model

MVP billing supports:

- one Stripe customer per billable workspace
- one active subscription per workspace
- plan entitlements stored in Snaelda, derived from Stripe subscription state
- Stripe Checkout for starting or changing a paid subscription
- Stripe Checkout in `mode=payment` for one-time add-ons sold alongside or after the subscription
- Stripe Customer Portal for payment method, invoice, and cancellation management
- webhook-driven subscription state updates

Possible early entitlements:

- active site count
- custom domain availability
- monthly generation/re-prompt allowance
- asset storage allowance
- team seat count when team support is added

## Trial Vs Paid

Every workspace begins in a trial state defined by [Spec 17](./17-guest-authoring-and-claim.md):

- 4 days from first activity
- 25 lifetime prompts
- publish to the hosted subdomain is allowed
- custom domains are blocked
- team invites are blocked

Subscription transitions the workspace out of trial state:

- the 4-day window and 25-prompt cap no longer apply
- custom domain attachment unlocks
- ongoing prompting becomes governed by the plan's monthly allowance
- team invites unlock once team support ships

Cancellation drops the workspace back into a trial-equivalent state with the previously consumed trial window and prompt count retained. Already-published sites continue to serve from the hosted subdomain; editing and new publishing are gated by the resumed trial limits.

## One-Time Add-Ons

Some offerings are sold as one-time charges rather than recurring entitlements. They use the same Stripe customer and the same Checkout surface, but are billed in `mode=payment` and produce no subscription record.

The first such add-on is the **Once-over from the maker**: a `$99` async review of the customer's first site, offered at Checkout. Operational details live in [docs/once-over-workflow.md](../docs/once-over-workflow.md); this spec only owns the billing contract.

Backend requirements for one-time add-ons:

- a separate Stripe price/product per add-on, distinct from subscription prices
- the same webhook handler, but branching on `mode` to avoid treating a one-time payment as a subscription change
- a durable record of the purchase, including the Stripe payment identifier, paid timestamp, and which workspace it belongs to, written from the webhook so the record survives even if the success redirect is lost
- an add-on-specific status column or table the product code can read for gating (for example `workspaces.once_over_status`)
- no impact on existing subscription entitlements unless the add-on explicitly grants one

Add-ons do not extend the trial, do not bypass subscription-gated entitlements, and are not refunded automatically on subscription cancellation.

## Data Ownership

Stripe owns payment details, invoices, payment methods, tax handling, and card data. Snaelda stores only Stripe identifiers, subscription state needed for authorization decisions, and local entitlement snapshots.

Recommended tables:

- `billing_customers`: workspace to Stripe customer mapping
- `billing_subscriptions`: Stripe subscription status, price/product references, billing period, cancellation status
- `billing_entitlements`: local workspace entitlement snapshot used by product code
- `billing_events`: processed Stripe webhook event IDs for idempotency and auditability
- per-add-on records as needed (for the Once-over, `once_over_requests` plus a `once_over_status` column on `workspaces`); see [docs/once-over-workflow.md](../docs/once-over-workflow.md) for schema

## Backend Responsibilities

The Go backend should:

- create or reuse a Stripe customer for the authenticated workspace
- create Checkout sessions for plan selection
- create Checkout sessions in `mode=payment` for one-time add-ons
- create Customer Portal sessions for self-service billing management
- verify Stripe webhook signatures before processing events
- process webhooks idempotently, branching on session `mode` so one-time payments do not collide with subscription events
- map Stripe subscription status to local access state
- enforce paid entitlements server-side before generation, publishing, custom domain setup, asset upload, or team expansion
- record audit events for billing changes that affect product access, including one-time add-on purchases

## Frontend Responsibilities

The React builder should:

- show current plan and entitlement usage in workspace settings
- start Checkout from a backend-created session URL
- open the Customer Portal from a backend-created session URL
- show billing-related blocked states where an entitlement prevents an action
- avoid handling raw payment details directly

## Configuration

Required environment variables should be added when Stripe is implemented:

- `STRIPE_SECRET_KEY`
- `STRIPE_WEBHOOK_SECRET`
- `STRIPE_PRICE_*` or a structured plan configuration, including one-time add-on price IDs such as `STRIPE_PRICE_ONCE_OVER`
- `BILLING_SUCCESS_URL`
- `BILLING_CANCEL_URL`
- `BILLING_PORTAL_RETURN_URL`

## Testing

Billing work should include:

- webhook signature verification tests
- idempotent event processing tests
- subscription status to entitlement mapping tests
- authorization tests for blocked actions when limits are exceeded
- one-time add-on tests: `mode=payment` Checkout session creation, webhook recording of the purchase, and correct add-on status transitions without touching subscription entitlements
- integration smoke tests against Stripe test mode or the Stripe CLI
