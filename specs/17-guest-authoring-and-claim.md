# Trial Sessions, Recovery, And Subscription

## Purpose

GTM requires the homepage prompt to produce a real site draft with no signup. This spec describes the cookie-bound trial session model, the optional recovery paths (workspace link or email) that let a free user secure their work, and the subscription transition that turns a trial session into a paying account.

This spec extends — it does not replace — Spec 02 (System Architecture), Spec 06 (Database Design), Spec 10 (API Surface), Spec 12 (Security), Spec 14 (User Flow), and Spec 15 (Billing).

The legacy phrase "guest user" in this codebase refers to a user on a trial session. The terms are synonymous; this spec prefers "trial session".

## Product Constraints

- The homepage prompt is the primary entry point. No signup is required to enter a prompt, generate a draft, edit, preview, or publish to the hosted subdomain.
- A trial session has 4 calendar days from first activity and a lifetime budget of 25 prompts before subscription is required to continue editing or prompting.
- Inside the builder, a trial user can do everything a paid user can except: attach a custom domain, prompt beyond 25, edit after the 4-day window expires, and invite teammates.
- Identity binding is optional and free. A trial user may attach a copy-able workspace link, an email, both, or neither. Attaching an email creates a real `users` row immediately so cross-browser login works the moment the address is verified.
- Subscription always requires an email. If the user does not already have one attached, Stripe Checkout collects it.
- The 4-day clock and the 25-prompt counter stop counting the moment a workspace has an active subscription.

## Identity Layers

Identity is a three-layer progression. All layers share the same builder capabilities; they differ only in how the workspace can be recovered.

### L0 — Cookie only

- The default after the first homepage prompt.
- Access depends entirely on the browser's cookie. Clearing it loses the workspace.
- The user is told this at the first-generation education moment and offered a way to upgrade to L1 or L2.

### L1 — Cookie plus workspace link

- The user clicks `Copy workspace link` and the backend returns a one-time-display URL of the form `{builder-host}/restore?k=<opaque-token>`.
- The token's hash is stored on `guest_sessions.recovery_key_hash`. The plaintext is never persisted.
- The link is multi-use until the session is claimed by an email (L2 transition) or the user explicitly regenerates it.
- Pasting or visiting the link in another browser sets a fresh cookie bound to the same `guest_sessions` row and lands the user back in the workspace.
- The link is the only recovery mechanism for users who don't want to share an email. Treat the recovery URL as bearer-secret in product copy.

### L2 — Cookie plus email

- The user submits an email from the builder's "Save your workspace" flow.
- Backend immediately creates a `users` row, inserts a `workspace_members` row joining that user to the workspace as `owner`, and writes `guest_sessions.claimed_by_user_id`.
- A magic link is sent to the address. Clicking it marks the email verified and the existing browser session continues working without interruption.
- On any other browser, the user gets back in via the standard magic-link login (see "Login Flow" below).
- Adding an email invalidates the L1 recovery link by nulling `guest_sessions.recovery_key_hash`. From this point on, magic link is the cross-browser recovery path.

Users move L0 → L1 → L2 in any order and at any time. L0 → L2 directly is fine and creates no L1 link. L1 → L2 invalidates the L1 link as described above.

## Trial Window And Prompt Budget

Two hard caps apply to any workspace without an active subscription.

### 4-Day Trial Window

- `guest_sessions.trial_started_at` is set on session creation (default: row `created_at`).
- The window expires at `trial_started_at + interval '4 days'`.
- After expiry, all write actions on the workspace return a structured `subscription_required` error. Read access to the builder and to preview-token responses continues to work so the user can still see what they made and decide to subscribe.
- The window resumes counting from where it left off if the user later cancels a subscription (per Spec 15).

### 25-Prompt Lifetime Budget

- `guest_sessions.prompts_used` increments on each successful generation job initiated against the workspace, including the initial generation and every site- or page-level re-prompt.
- When `prompts_used >= 25`, generation routes return the same `subscription_required` error. Non-generation edits remain allowed until the 4-day window expires.
- Quota is not refunded for failed generations beyond what Spec 07's failure handling already specifies.

### Behavior On Subscribe

- When a workspace gains an active subscription, both gates lift immediately.
- Subscription does not reset `prompts_used` — paid prompting is governed by the subscription's monthly allowance in Spec 15, not by the trial counter.
- Subscription does not clear `trial_started_at`; the field is retained for analytics and for the cancellation-resume rule above.

## Capabilities

Trial sessions (any of L0/L1/L2 without a subscription) may, during the active trial window:

- generate a new site from a prompt
- re-prompt the site or a page within the 25-prompt budget
- edit block content and theme tokens
- reorder, add, and remove blocks within the registry
- manage pages and navigation
- upload assets and use the asset library
- preview the draft
- **publish the site to its hosted subdomain**
- roll back to a prior published version
- view their own light analytics for the published subdomain

Trial sessions may not:

- attach a custom domain
- invite teammates
- generate beyond the 25-prompt budget
- perform write actions after the 4-day trial expires

Subscribed workspaces gain custom domains, ongoing prompting per their plan allowance, lifted trial window, and team invites (when team support ships).

All blocked actions must return a structured error the builder can map to the appropriate CTA: `Save your workspace` for cookie-loss risk, `Subscribe` for paid-only or trial-exhausted states.

## Multi-Site Within A Workspace

A workspace may contain more than one site. Each homepage-prompt submission creates a new `sites` row inside the caller's workspace:

- Anonymous visitor with no cookie: create workspace + first site.
- Trial visitor with an existing session: create a new site in the existing workspace.
- Authenticated user: create a new site in their workspace.

Plan limits in Spec 15 cap the number of *active* sites a workspace may keep. Going over the cap returns a `plan_limit_exceeded` error with a CTA to subscribe or to archive an existing site.

## Login Flow

The landing page exposes a single `Log in` action separate from the prompt field. Login is only relevant for L2 users.

1. User clicks `Log in` on the landing page.
2. Builder presents an email input.
3. User enters their email; backend creates a one-time magic-link token in `magic_links`, hashes it, and emails the plaintext as a URL.
4. User clicks the link, which hits `GET /api/auth/magic`. Backend validates the token, marks it consumed, and issues an authenticated session cookie scoped to the same browser.
5. Backend resolves the user's workspace memberships and lands the user in the most recently active workspace's builder.

Magic links:

- expire 15 minutes after issuance
- are single-use
- are scoped to a single user and a single intent (login or email verification)
- are issued via authenticated mailer, not via the public form-submission path

If an unknown email is entered at the login screen, the backend should still respond with a generic "Check your email" message and not send mail; this prevents email enumeration.

## Subscribe Flow

Subscription always lifts the trial gates and requires a verified email.

1. User triggers a paid action (custom domain, prompt past 25, edit after trial expires).
2. Builder calls `POST /api/billing/checkout` with the target workspace.
3. Backend creates or reuses a Stripe customer for the workspace and returns a Stripe Checkout session URL.
4. User completes Checkout. Stripe collects email if the workspace has no L2 binding.
5. Stripe webhook fires; backend either:
   - confirms the existing `users` row (L2 already attached, email matches), or
   - creates a `users` row from the Checkout email, inserts the `workspace_members` row, and records `guest_sessions.claimed_by_user_id` and `claimed_at`.
6. Backend writes `billing_subscriptions` and refreshes the workspace's `billing_entitlements` per Spec 15.
7. The previously blocked action proceeds on the next user attempt or, where feasible, is auto-retried server-side.

Email mismatch handling: if the Checkout email differs from a pre-attached L2 email, the backend keeps the L2-attached `users` row as the workspace owner and stores the Checkout email only as the Stripe customer email. Surface this in the builder so the user can reconcile if they want.

## Recovery From Lost Browser State

The recovery path depends on the user's last identity layer.

- L0: no recovery. The workspace persists in the database but is unreachable until garbage collection. Product copy at the education moment must make this clear.
- L1: paste the workspace link on the landing page's `Restore workspace` action, or visit it directly. Backend looks up by hashed token and issues a fresh cookie.
- L2: enter email at `Log in`. Magic link delivers a new authenticated session.
- Subscribed: same as L2.

There is no admin recovery flow for MVP.

## Public Surface Behavior

A trial workspace can publish to its hosted subdomain. The hosted public site service must treat a trial-published site the same as any other published site:

- domain resolution goes through `site_domains`
- public delivery resolves only against the active published version per Spec 16
- forms, analytics, and assets follow the runtime rules in Spec 16

The trial state never leaks to the public-facing surface. Public visitors cannot distinguish a trial-published site from a paid one except via the lack of a custom domain.

If the trial expires (4 days, no subscription), already-published versions continue to serve from the hosted subdomain. The author simply cannot edit, re-prompt, or publish a new version. Subscribing resumes both editing and republishing without changing the live snapshot.

If a trial site has not yet been published, expiry has no public consequence.

## Authorization Rules

Spec 12's authorization checks expand. For every write the backend must verify:

1. A valid session cookie is present and resolves to a `guest_sessions` row (or to an authenticated user session whose user owns the target workspace).
2. The session's `workspace_id` owns the target site, page, block, or asset.
3. The action is allowed in the current capability state:
   - if the workspace has an active subscription, the paid capability set applies
   - otherwise the trial capability set applies, and the request must additionally pass the 4-day window check and, for generation routes, the 25-prompt budget check
4. The action is in the post-trial capability set if the trial has expired and the workspace has no active subscription. For MVP that set is empty for writes and full for reads.

A request that carries both a cookie session and an authenticated user session for different workspaces is treated as the authenticated user's session; the cookie is ignored except by the claim, restore, and login endpoints.

## Schema

Two changes from the current spec.

### `guest_sessions` (extended)

```sql
create table guest_sessions (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references workspaces(id) on delete cascade,
  cookie_token_hash text not null unique,
  recovery_key_hash text unique,
  prompts_used int not null default 0,
  trial_started_at timestamptz not null default now(),
  claimed_by_user_id uuid references users(id),
  claimed_at timestamptz,
  created_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now()
);

create index guest_sessions_workspace_idx on guest_sessions(workspace_id);
```

### `magic_links` (new)

```sql
create table magic_links (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references users(id) on delete cascade,
  token_hash text not null unique,
  purpose text not null check (purpose in ('login', 'verify_email')),
  expires_at timestamptz not null,
  consumed_at timestamptz,
  created_at timestamptz not null default now()
);

create index magic_links_user_idx on magic_links(user_id);
```

No other schema changes are required. The existing nullable `created_by` columns on `workspaces`, `assets`, `generation_jobs`, `site_versions`, and `audit_events` already accommodate trial-authored rows. Trial-initiated `audit_events` should set `user_id = NULL` until claim and include `metadata.guest_session_id`.

## API Surface

Routes accepting either a trial cookie or an authenticated user session (the unified builder API):

```http
POST   /api/sites/generate
POST   /api/sites/:siteId/reprompt
POST   /api/sites/:siteId/pages/:pageId/reprompt
POST   /api/sites/:siteId/undo
GET    /api/sites
GET    /api/sites/:siteId
PATCH  /api/sites/:siteId
DELETE /api/sites/:siteId
POST   /api/sites/:siteId/pages
PATCH  /api/sites/:siteId/pages/:pageId
DELETE /api/sites/:siteId/pages/:pageId
POST   /api/sites/:siteId/pages/reorder
POST   /api/sites/:siteId/pages/:pageId/blocks
PATCH  /api/sites/:siteId/pages/:pageId/blocks/:blockId
DELETE /api/sites/:siteId/pages/:pageId/blocks/:blockId
POST   /api/sites/:siteId/pages/:pageId/blocks/reorder
POST   /api/sites/:siteId/pages/:pageId/blocks/:blockId/duplicate
GET    /api/sites/:siteId/theme
PATCH  /api/sites/:siteId/theme
POST   /api/sites/:siteId/theme/regenerate
GET    /api/sites/:siteId/collections
POST   /api/sites/:siteId/collections
POST   /api/sites/:siteId/collections/draft-from-prompt
GET    /api/sites/:siteId/collections/:collectionId
PATCH  /api/sites/:siteId/collections/:collectionId
DELETE /api/sites/:siteId/collections/:collectionId
GET    /api/sites/:siteId/collections/:collectionId/entries
POST   /api/sites/:siteId/collections/:collectionId/entries
GET    /api/sites/:siteId/collections/:collectionId/entries/:entryId
PATCH  /api/sites/:siteId/collections/:collectionId/entries/:entryId
DELETE /api/sites/:siteId/collections/:collectionId/entries/:entryId
POST   /api/sites/:siteId/collections/:collectionId/entries/reorder
POST   /api/sites/:siteId/collections/:collectionId/entries/draft-from-prompt
POST   /api/sites/:siteId/preview-token
POST   /api/sites/:siteId/publish
GET    /api/sites/:siteId/versions
POST   /api/sites/:siteId/rollback/:versionId
POST   /api/assets/upload-url
POST   /api/assets/complete
GET    /api/sites/:siteId/assets
PATCH  /api/assets/:assetId
DELETE /api/assets/:assetId
GET    /api/sites/:siteId/analytics/views
GET    /api/sites/:siteId/analytics/views/pages
```

Paid-only routes (require an active subscription on the workspace):

```http
POST   /api/sites/:siteId/domains          # custom domain attachment
PATCH  /api/sites/:siteId/domains/:id
DELETE /api/sites/:siteId/domains/:id
```

Trial-session lifecycle and recovery routes:

```http
POST   /api/sessions/anonymous             # create or reuse a cookie-bound trial workspace
POST   /api/sessions/restore               # consume a recovery key, set a fresh cookie
POST   /api/sessions/recovery-key          # mint or regenerate the workspace recovery link (L1)
DELETE /api/sessions/recovery-key          # revoke the current recovery link
POST   /api/sessions/claim                 # promote to L2; creates users row, sends verify magic link
GET    /api/sessions/me                    # return current session state: layer, prompts_used, trial_ends_at, subscribed flag
```

Schema migration and single-entry reprompt routes remain required additions. All model-backed collection routes must share the same prompt accounting, rate limiting, job tracking, and audit behavior as site/page generation.

Authentication routes:

```http
POST   /api/auth/magic-link                # request a login magic link by email
GET    /api/auth/magic                     # consume a magic-link token, set session cookie
POST   /api/auth/logout
```

Billing routes (unchanged scope, included for completeness; see Spec 15):

```http
POST   /api/billing/checkout
POST   /api/billing/portal
GET    /api/billing/entitlements
```

Form-submission management remains workspace-member-authenticated and is reachable only by L2-or-paid owners of the workspace:

```http
GET   /api/sites/:siteId/form-submissions
PATCH /api/form-submissions/:submissionId
```

## Builder UX Hooks

The builder must surface:

- a persistent trial banner showing days remaining and prompts remaining, hidden once subscribed
- a `Save your workspace` action in the builder chrome that opens a small flow with three options: `Copy workspace link`, `Add an email`, `Already have an account` (which switches to magic-link login)
- a first-generation education modal explaining cookie-only state and the three save options
- inline upgrade CTAs on `subscription_required` errors that include the failing action's name
- a `Restore workspace` affordance on the landing page when a recovery URL is pasted
- an `Open workspace` / `Continue editing` affordance on the landing page when a cookie session is detected
- a `Log in` affordance on the landing page that opens the magic-link form

## Out Of Scope For MVP

- merging two cookie-bound sessions in the same browser
- admin recovery for users who lose cookie, link, and email
- granular per-action quotas beyond the 25-prompt and 4-day caps
- recovery-key rotation policy beyond manual regenerate
- team invites of any kind
- automatic abandonment cleanup of expired trial workspaces
