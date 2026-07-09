# Once-Over from the Maker

A one-time, async review of a customer's first site (customer-facing: "Creator once-over" — 13.900 ISK, 999 SEK reserved, `$99` internal reference; see [Spec 15](../specs/15-billing-and-stripe.md)), delivered as a Loom-style walkthrough plus a few in-builder edits. Offered as an optional add-on at Checkout. Bounded to ~30 minutes of work per job so the math holds at early-cohort volume.

See also: [Spec 15 — Billing and Stripe](../specs/15-billing-and-stripe.md).

## What it is

- One pass over the customer's draft, no follow-up rounds.
- Async only. No live calls, no scheduling.
- Delivered as a short recorded walkthrough plus 3–5 high-leverage edits made directly in their builder.
- Sold as a single Stripe one-time charge at Checkout, separate from the subscription.

## What it is not

State this on the Checkout add-on copy so expectations are correct up front:

- Not writing all the copy from scratch.
- Not custom design or new block types.
- Not DNS or domain setup.
- Not multiple rounds. One pass, with a clear list of next steps the customer owns.

## SLA

- Within 3 business days of the customer marking their workspace "ready for review."
- One pass per purchase. Additional passes require a new purchase.

## Lifecycle

1. **Purchase.** Customer ticks the add-on at Checkout. Stripe charges the local-currency price (13.900 ISK) one-time alongside the subscription.
2. **Webhook.** `checkout.session.completed` (mode=payment) flips `workspaces.once_over_status = 'awaiting_intake'` and inserts a row in `once_over_requests`.
3. **Intake email.** Customer receives a transactional email linking to the intake form in their builder.
4. **Intake.** Customer fills five fields and flips a "ready for review" toggle. Toggle writes `once_over_status = 'pending'` and timestamps `intake_submitted_at`.
5. **Operator notification.** Pending requests are surfaced in an admin view (or a daily SQL query).
6. **Pass.** Operator runs the 25–35 min workflow below.
7. **Delivery.** Operator sends the walkthrough link and next-steps list. Status flips to `delivered`. `delivered_at` timestamp written.

## Intake form

Five fields, gated to workspaces with `once_over_status = 'awaiting_intake'`:

1. One sentence: what does your business do?
2. One sentence: who is the visitor?
3. What is the #1 outcome you want from this site? (sale, booking, walk-in, gallery view, inquiry)
4. Anything you've been stuck on?
5. "Ready for review" toggle.

The toggle is the only required action to advance the state. The customer can keep editing after submitting; the operator reads the site as it is at pass time, not at submit time.

## The pass (25–35 minutes per job)

| Time | Step |
|---|---|
| 3 min | Open the customer's preview side by side with their intake form. |
| 5 min | Read the site cold as a first-time visitor: homepage, scroll, click the primary CTA. Note the 3 highest-leverage problems. |
| 15 min | Make 3–5 edits directly in their builder. Typical fixes below. |
| 5 min | Record a 3–5 min walkthrough (Loom or Tella) explaining what changed, why, and the 3 things only the customer can do next. |
| 2 min | Send the canned delivery email with the video link and next-steps list. Mark `delivered_at`. |

### Common high-leverage edits

- Rewrite the hero headline so it names the business and the outcome.
- Move proof (testimonials, photos, "as seen in") above features.
- Fix the primary CTA copy and make sure it appears at least twice (hero plus near the bottom).
- Swap one weak image from the asset library or starter set.
- Set the homepage SEO title and description.

### The next-steps list

The most important deliverable. Three items the customer owns that you cannot do for them. Almost always:

1. Replace placeholder copy in the sections you flagged.
2. Upload their own photos in place of the starter imagery.
3. Connect their custom domain (link to the docs).

This transfers ownership back and prevents scope creep.

## Boundaries and the canned follow-up reply

Some customers will reply asking for more. One canned response:

> Glad it landed. The Once-over is a one-time pass, but everything I changed is in your builder for you to keep tweaking. If you want me to take another look after you've added your copy and photos, send me a note and I'll see what I can do.

Hold the line. The whole point of one-time pricing is that follow-up is the operator's choice, not the customer's.

## Tooling

- **Stripe**: one-time price ID; webhook handler in the billing module distinguishes `mode=payment` from `mode=subscription`.
- **Database**: see Schema below.
- **Builder**: an intake route gated by `once_over_status = 'awaiting_intake'`.
- **Email**: two transactional templates: "your Once-over is ready to start" (intake link) and "your Once-over is done" (video link).
- **Video**: Loom or Tella free tier.
- **Admin view**: a route like `/app/admin/once-over` listing pending requests by `intake_submitted_at`, or just a saved SQL query.

## Schema

```sql
-- new column on workspaces
ALTER TABLE workspaces
  ADD COLUMN once_over_status TEXT
  CHECK (once_over_status IN ('none','awaiting_intake','pending','delivered'))
  NOT NULL DEFAULT 'none';

-- new table
CREATE TABLE once_over_requests (
  id               UUID PRIMARY KEY,
  workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  stripe_payment   TEXT NOT NULL,
  paid_at          TIMESTAMPTZ NOT NULL,
  intake_business  TEXT,
  intake_visitor   TEXT,
  intake_outcome   TEXT,
  intake_stuck_on  TEXT,
  intake_submitted_at TIMESTAMPTZ,
  video_url        TEXT,
  delivered_at     TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX once_over_requests_pending_idx
  ON once_over_requests (intake_submitted_at)
  WHERE delivered_at IS NULL AND intake_submitted_at IS NOT NULL;
```

`workspaces.once_over_status` is the fast-path flag for gating UI and the admin queue. `once_over_requests` is the durable record per purchase so repeat purchases each get their own row.

## Operator log

After each delivered pass, record one paragraph in a private log (a Notion page, a markdown file, anything):

- What was wrong with the generated draft.
- Which fix had the biggest visible impact.
- One thing the customer asked for that the product should do automatically.

This is the closest thing to live user-research the launch will produce. The point of the log is to feed back into generation prompts, default block ordering, and the SEO defaults — not to remember the individual customer.

## Numbers at the cap

- 10–15% take rate on 50 paid users = 5–8 Once-overs across the launch.
- ~30 min per job × 7 ≈ 3.5 hours total.
- `$99 × 7 ≈ $700` add-on revenue across the cohort.
- Effective rate ~`$200/hr`, no recurring obligation.

The hidden upside is the operator log. Block the half-hour right after each pass to write it. Three to four hours of structured, real-customer feedback during launch is worth more than the cash.
