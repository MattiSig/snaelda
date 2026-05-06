# Billing and Stripe

## Scope

Snaelda should use Stripe for platform billing: subscriptions, invoices, payment collection, customer portal access, and usage or entitlement enforcement for Snaelda accounts.

This is separate from e-commerce checkout inside generated customer websites. Customer storefront payments remain out of scope until the core website builder and hosting product is stable.

## Billing Model

The first paid version should support:

- one Stripe customer per billable workspace
- one active subscription per workspace for the initial product
- plan entitlements stored in Snaelda, derived from Stripe subscription state
- Stripe Checkout for starting or changing a paid subscription
- Stripe Customer Portal for payment method, invoice, and cancellation management
- webhook-driven subscription state updates

Possible early entitlements:

- active site count
- published site count
- custom domain availability
- generation or re-prompt monthly allowance
- asset storage allowance
- team seat count when team support is added

## Data Ownership

Stripe owns payment details, invoices, payment methods, tax handling, and card data. Snaelda stores only Stripe identifiers, subscription state needed for authorization decisions, and local entitlement snapshots.

Recommended tables:

- `billing_customers`: workspace to Stripe customer mapping
- `billing_subscriptions`: Stripe subscription status, price/product references, billing period, cancellation status
- `billing_entitlements`: local workspace entitlement snapshot used by product code
- `billing_events`: processed Stripe webhook event IDs for idempotency and auditability

## Backend Responsibilities

The Go backend should:

- create or reuse a Stripe customer for the authenticated workspace
- create Checkout sessions for plan selection
- create Customer Portal sessions for self-service billing management
- verify Stripe webhook signatures before processing events
- process webhooks idempotently
- map Stripe subscription status to local access state
- enforce paid entitlements server-side before generation, publishing, custom domain setup, asset upload, or team expansion
- record audit events for billing changes that affect product access

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
- `STRIPE_PRICE_*` or a structured plan configuration
- `BILLING_SUCCESS_URL`
- `BILLING_CANCEL_URL`
- `BILLING_PORTAL_RETURN_URL`

## Testing

Billing work should include:

- webhook signature verification tests
- idempotent event processing tests
- subscription status to entitlement mapping tests
- authorization tests for blocked actions when limits are exceeded
- integration smoke tests against Stripe test mode or the Stripe CLI
