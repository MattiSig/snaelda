# Transactional Email

## Purpose

Several other specs need a reliable way to deliver email: magic-link login and email verification (Spec 17), billing receipts and payment-failure notices (Spec 15), form-submission forwarding to site owners (Spec 16), and operator notifications for the Once-over add-on ([docs/once-over-workflow.md](../docs/once-over-workflow.md)). This spec owns the email transport contract, the local development path, and the production provider so other specs can depend on it without re-deciding.

## Transport Decision

- **Production:** [Resend](https://resend.com) via its HTTP API.
- **Local development:** [Mailpit](https://github.com/axllent/mailpit) as a local SMTP sink with a web UI.
- **Tests and CI:** an in-memory transport that captures sent messages for assertions.

Resend was chosen over Postmark and SES because of a generous free tier that covers MVP volume (3,000 emails/month, 100/day), simple Go integration, and DKIM/SPF setup that aligns automatically off the sending domain. Self-hosted SMTP was rejected for MVP: the deliverability cost — IP warming, blocklist monitoring, DKIM/SPF/DMARC tuning, bounce handling — is disproportionate to the launch volume, and the first email Snaelda sends is a magic-link login where a spam-folder delivery breaks the whole funnel.

The choice is reversible: the `Mailer` interface below isolates the provider so swapping to Postmark, SES, or self-hosted SMTP is a transport-layer change, not a product change.

## Mailer Interface

A single Go interface in `internal/email/` is the only contract product code touches. Three implementations live behind it.

```go
package email

type Message struct {
    To          []Address       // required, non-empty
    Cc          []Address
    Bcc         []Address
    From        Address         // optional; defaults to configured "from"
    ReplyTo     *Address
    Subject     string          // required
    TextBody    string          // required if HTMLBody is empty
    HTMLBody    string          // required if TextBody is empty
    Tags        map[string]string // optional, for provider-side tagging
    IdempotencyKey string       // optional; provider-side dedupe
}

type Address struct {
    Email string
    Name  string
}

type SendResult struct {
    ProviderMessageID string
    AcceptedAt        time.Time
}

type Mailer interface {
    Send(ctx context.Context, msg Message) (SendResult, error)
}
```

Errors are typed at the package level so callers can distinguish recoverable from terminal failures:

- `ErrInvalidMessage` — the message is malformed (no recipient, empty subject, no body, etc.). Do not retry.
- `ErrRateLimited` — provider returned a 429-equivalent. Retryable.
- `ErrProviderUnavailable` — 5xx from the provider, network failure, timeout. Retryable.
- `ErrPermanent` — provider rejected the message permanently (bad address, suppressed domain). Do not retry; record and surface.

All other errors should be treated as `ErrProviderUnavailable` by callers.

## Transports

Selected at process boot from a single env var, `EMAIL_TRANSPORT`. Unknown values fail boot fast.

### `stdout` (default in development)

Writes the rendered message to the process log. Useful for the CLI loop where the developer just wants to see the magic link printed and click it. No SMTP, no Docker dependency. Returns a synthetic `ProviderMessageID` so call sites work unchanged.

### `mailpit` (opt-in for local development)

SMTP transport pointed at a local Mailpit container. The Mailpit web UI at `http://localhost:8025` shows the full message tree, headers, HTML, text alternative, and any attachments. Use this when changes are visually iterating on the templates.

Mailpit ships as a single binary; the project's existing `docker-compose` will run it alongside Postgres and SeaweedFS:

```yaml
# docker-compose.yml — illustrative; final wiring lives with the dev-up tooling
mailpit:
  image: axllent/mailpit:latest
  ports:
    - "1025:1025"  # SMTP
    - "8025:8025"  # web UI
```

The Go transport speaks plain SMTP to `localhost:1025`. No auth, no TLS — Mailpit is a developer tool, not a relay.

### `resend` (production)

HTTP transport against the Resend API. Uses an idempotency key derived from `Message.IdempotencyKey` when present so that retries inside the application don't double-send.

Bounces and complaints come back via a Resend webhook (out of scope to wire end-to-end for MVP, but the spec leaves the door open):

- `POST /api/email/webhook` — verify signature, mark the destination address as suppressed in a small `email_suppressions` table, and surface in the workspace billing/settings if the suppressed address is one Snaelda relies on for login.

For MVP, lack of webhook handling means a bounced magic link is invisible to the platform; the user will see the "didn't get the email?" affordance described in Spec 17 and can retry with a fixed address.

### `memory` (tests only)

Captures all sent messages on a slice. Tests assert on subjects, recipients, and rendered bodies without spinning up SMTP. Exposed only to test code; cannot be selected from `EMAIL_TRANSPORT`.

## Templates

Templates live in `internal/email/templates/` as Go `text/template` and `html/template` files, paired by name. Each template defines a `Render(data any) (subject, text, html string, err error)` function generated at compile time. No runtime template loading from disk.

MVP templates required:

- `magic_link_login` — magic-link login (Spec 17)
- `magic_link_verify` — first-time email verification on L2 attach (Spec 17)
- `billing_receipt` — Stripe-driven receipt confirmation (Spec 15)
- `billing_payment_failed` — payment failure with portal link (Spec 15)
- `once_over_intake_ready` — the customer's Once-over is purchased, intake link enclosed (docs/once-over-workflow.md)
- `once_over_delivered` — the operator-recorded walkthrough is ready, link enclosed
- `form_submission_forwarded` — a public form submission is forwarded to the site owner (Spec 16)

Each template must produce both a text body and an HTML body. Text is not optional — it's both an accessibility floor and a deliverability signal.

## Rate Limiting

Per-address and per-IP rate limits use the same durable pattern as `form_submission_attempts` (Spec 12). One additional table:

```sql
create table email_send_attempts (
  id            uuid primary key default gen_random_uuid(),
  address_hash  text not null,        -- sha256 of normalized email
  purpose       text not null,        -- e.g., 'magic_link_login'
  occurred_at   timestamptz not null default now()
);
create index email_send_attempts_addr_idx
  on email_send_attempts (address_hash, purpose, occurred_at desc);
```

Defaults:

- magic-link login: 5 sends per address per 15 minutes, 20 per address per 24 hours
- email verification: 3 sends per address per hour
- form forwarding: 30 sends per destination address per hour
- billing receipts and payment-failure mails: unbounded, they are Stripe-driven and idempotent

Rate-limit denials never leak whether the address exists; the response from the API is the same whether the address is known or not (matches the Spec 17 anti-enumeration stance).

## Configuration

Environment variables:

```
EMAIL_TRANSPORT=stdout            # stdout | mailpit | resend
EMAIL_FROM_ADDRESS=hi@snaelda.app
EMAIL_FROM_NAME=Snaelda
EMAIL_REPLY_TO=                   # optional
RESEND_API_KEY=                   # required when EMAIL_TRANSPORT=resend
MAILPIT_SMTP_ADDR=localhost:1025  # used when EMAIL_TRANSPORT=mailpit
```

Bootstrap validates that `RESEND_API_KEY` is non-empty when `EMAIL_TRANSPORT=resend`, and refuses to boot the API otherwise. The Mailpit transport does not need credentials.

## Backend Responsibilities

The `internal/email` package owns:

- the `Mailer` interface and the four implementations above
- template rendering (compile-time-bound, type-checked data structs per template)
- rate-limit enforcement via `email_send_attempts`
- a thin helper layer per use case (`SendMagicLink`, `SendBillingReceipt`, etc.) so call sites don't manually build `Message` structs

Callers do not select a transport; the wired `Mailer` is injected at the call site via the existing platform module wiring used elsewhere in `internal/`.

## Frontend Responsibilities

No direct surface. The builder triggers sends via existing endpoints (login, claim, billing, forms). The only user-visible affordance owned by email work is the "Didn't get the email?" link on the magic-link prompt, which calls the same send endpoint and respects the rate limit. Repeat-press surfaces an inline message after the cap is reached, without revealing whether the address exists.

## Testing

- unit tests against the `memory` transport asserting per-template rendering, including text/HTML parity
- rate-limit tests covering the per-address-per-purpose windows and the anti-enumeration response shape
- a contract test for the Resend transport that hits a recorded HTTP fixture (no live network in CI)
- a smoke test that spins up Mailpit in CI, sends one of each template, and asserts the SMTP body lands in Mailpit's API
- a boot test asserting `EMAIL_TRANSPORT=resend` without `RESEND_API_KEY` fails fast

## Out of Scope for MVP

- inbound email parsing
- Resend webhook handling for bounces and complaints (interface is reserved, not wired)
- per-workspace custom from-addresses
- email scheduling or queued send retries beyond a single in-process retry on `ErrProviderUnavailable`
- transactional batching
- localization of templates
