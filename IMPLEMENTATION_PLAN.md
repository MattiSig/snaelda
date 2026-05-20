# Implementation Plan

This file tracks confirmed remaining work only, sorted by implementation priority. The MVP shape is: anonymous prompt → generated site → editable draft → published hosted subdomain, with optional account claim and Stripe billing. The trial→claim→billing tri-block is the current release blocker.

## Recently Confirmed Complete

- [x] Asset upload and image-library UI exist in the builder, including uploaded-asset selection in block editors.
- [x] Contact-form submission storage and the chosen MVP moderation flow exist; email forwarding remains optional follow-up work, not unfinished core behavior.
- [x] The 10-page limit is already enforced in validation, generation repair, the database, and publish preflight.
- [x] Main builder loading, empty, and error states exist for login, site list, site detail, preview, publish history, assets, and submissions.
- [x] Page-level SEO editing plus publish-time `sitemap.xml`, `robots.txt`, canonical metadata, and basic social metadata exist.
- [x] Refresh-token rotation is server-side and hashed; publish/rollback cache invalidation already exists.
- [x] Added [specs/16-runtime-lifecycles-and-analytics.md](./specs/16-runtime-lifecycles-and-analytics.md) to define public visibility rules, domain/runtime semantics, and MVP analytics scope that were previously only implied.
- [x] Public page reads now resolve from stored published artifacts plus `manifest.json` metadata; public `/public/{slug}` routes no longer carry internal publish framing.
- [x] Publish validates artifact completeness before promoting a version live (page HTML, crawl files, theme CSS, manifest metadata).
- [x] Hosted public URLs use an explicit deployment contract via `PUBLIC_BASE_URL` and `PUBLIC_BASE_DOMAIN`.
- [x] `internal/domains` exposes a real read API for hosted-domain state from `site_domains`.
- [x] Builder publish panel surfaces the actual hosted live URL.
- [x] Generation supports a provider-backed structured-output planner through OpenAI with deterministic fallback, plus a separate theme regeneration model call.
- [x] Generation metadata writes and job completion are mandatory success conditions.
- [x] Theme regeneration shipped as `POST /api/sites/:siteId/theme/regenerate`.
- [x] Public form submission resolves strictly against the active published version's snapshot.
- [x] Public asset delivery requires the asset to be referenced by the active published version, with hostname-based resolution.
- [x] Public page resolution records non-blocking views into `page_view_daily` via `analytics.CountableRequest`, filtering bots/health checks.
- [x] `GET /api/sites/{siteId}/analytics?window=7d|30d|all` plus builder analytics view at `/app/sites/{siteId}/analytics` with totals, daily trend, per-page breakdown.
- [x] Transactional email foundation exists in `internal/email/` with stdout, Mailpit SMTP, Resend HTTP, and memory transports; paired text/HTML templates; `email_send_attempts` migration; config/env validation; and Mailpit in local compose.
- [x] Navigation is first-class editable canonical data. `PUT /api/sites/{siteId}/navigation` replaces the whole primary list; builder editor supports rename, external links, reorder, remove.
- [x] Backend-owned starter imagery via Pexels in `internal/imagery`, re-hosted as `assets` rows with `provenance`, surfaced as credits on public pages and starter labels in the asset picker.
- [x] Durable spam handling for public forms: honeypot, deterministic spam scoring, `form_submission_attempts` table for cross-replica rate limiting, see `internal/platform/database/migrations/000006_form_spam_handling.sql`.
- [x] Authoring-lifecycle audit events for `site.create`, `site.delete`, `page.delete`, `block.delete`, `site.generate`, `site.reprompt`, `page.reprompt`, `asset.upload`, `asset.delete`.
- [x] All 12 spec-required block types implemented in `internal/siteconfig/blocks.go` with registry contract test, per-block prop schemas, plain-text enforcement, URL allowlist, and form-field allowlist.
- [x] Preview tokens (hashed, TTL, revocable) and public render via hostname or slug are implemented.
- [x] Block CRUD (DnD + button reorder), page CRUD with SEO and nav inclusion, draft/preview/publish with versions/rollback, and site- and page-level reprompt with undo are all shipped.
- [x] CSRF middleware, HttpOnly+Secure+SameSite=Lax cookies, durable per-IP rate limiting, URL allowlist, and plain-text-rejects-HTML are all in place.
- [x] Public + authenticated response-header policy is now enforced in `internal/api/server.go` with explicit CSP, HSTS, frame/type/referrer headers, and `Cache-Control: private, no-store`; spec 12 includes the header table and the public CSP allows published inline styles.

## Priority Backlog

- [x] Build the guest-trial / claim subsystem end to end (specs 17, 06, 10, 12). MVP release blocker.
  - Shipped DB support for `guest_sessions`, `magic_links`, and guest-authored preview tokens in `internal/platform/database/migrations/000008_guest_sessions_magic_links.sql` and `000009_preview_tokens_guest_authors.sql`.
  - Shipped unified session resolution in `internal/auth/` for authenticated and cookie-bound trial workspaces, including prompt-cap gating, post-expiry write blocking, publish-before-claim blocking, recovery-link issuance/restoration, and magic-link claim/login flows.
  - Shipped `POST /api/sessions/anonymous`, `GET /api/sessions/me`, `POST /api/sessions/restore`, `POST|DELETE /api/sessions/recovery-key`, `POST /api/sessions/claim`, `POST /api/auth/magic-link`, and `GET /api/auth/magic`.
  - Replaced the old dev-login flow with an anonymous homepage prompt, trial-state builder banner, save-workspace UI, login-by-email, and dedicated `/restore` recovery route in `apps/web/src/routes/`.
  - Verified anonymous prompt → builder → preview, browser restore → `/app`, and direct endpoint restore + claim behavior.

- [ ] Finish the remaining transactional-email integrations from [specs/18-transactional-email.md](./specs/18-transactional-email.md) (cross-cutting blocker for specs 16 and 15).
  - [x] Auth slice is shipped: `POST /api/auth/magic-link` and `GET /api/auth/magic`, `magic_links` persistence, `internal/email/` transport wiring, and generic anti-enumeration responses for login requests.
  - [x] Public contact-form forwarding is now wired through the shared mailer with `form_submission_forwarded`, durable destination-address rate limiting (`30/hour`), idempotency keyed on submission ID, and non-blocking failure handling so stored submissions still succeed if email delivery fails.
  - [x] Billing receipt and payment-failure notices now send through the shared `internal/email/` package from Stripe webhook handling in `internal/billing/`, with idempotency keyed on Stripe event ID.
  - [x] Once-over purchase flow now persists `once_over_requests`, updates `workspaces.once_over_status`, and sends the `once_over_intake_ready` transactional email from the Stripe checkout webhook.
  - [ ] Hook the existing `internal/email/` package into the remaining Once-over delivery/admin call sites and enforce Spec 18 send windows when the operator delivery workflow ships.
  - [ ] Acceptance follow-up: magic-link login, form forwarding, billing notices, and Once-over intake-ready emails are wired through the shared mailer; remaining verification work is Mailpit/Resend round-trips plus the future Once-over delivery/admin call sites above.

- [ ] Implement the `billing` module against Stripe (spec 15). MVP release blocker.
  - [x] Replaced the stub `internal/billing/module.go` with a real backend module mounted from `internal/api/server.go`, backed by Stripe's Go SDK in `go.mod`.
  - [x] DB foundation shipped in `internal/platform/database/migrations/000010_billing_foundation.sql`: `billing_customers`, `billing_subscriptions`, `billing_entitlements`, `billing_events`, plus `workspaces.plan` and `workspaces.stripe_customer_id`.
  - [x] Endpoints shipped: `POST /api/billing/checkout`, `POST /api/billing/portal`, `GET /api/billing/entitlements`, `POST /api/billing/webhook`, including Stripe signature verification and webhook idempotency on `billing_events`.
  - [x] Env wiring shipped in `internal/platform/config/` for `STRIPE_*` and `BILLING_*`.
  - [x] Auth/session resolution now reads `billing_entitlements.subscription_live`, so trial gating lifts immediately once Stripe-backed entitlements flip live.
  - [ ] Gating: enforce entitlements in `internal/generation/handler.go`, `internal/publishing/handler.go`, `internal/assets/`, `internal/domains/`.
    - Paid plan limits now gate new site creation (`internal/sites/handler.go`), paid prompt allowance (`internal/generation/handler.go`), asset storage (`internal/assets/handler.go`), and domain read responses expose `customDomainsEnabled` for builder gating.
    - Hosted-subdomain publish remains available to trials by spec 17; custom-domain write enforcement still belongs to the spec 13 domain-attach endpoints that are not built yet.
  - [x] Frontend: billing routes under `apps/web/src/routes/app.billing.*`, blocked-action UI in builder, plan badge in shell.
  - [x] Add-on / Once-over follow-up shipped: `STRIPE_PRICE_ONCE_OVER`, payment-mode Checkout, webhook persistence in `once_over_requests`, workspace status tracking, and billing-page intake UI/API.
  - [ ] Acceptance: claim → checkout → webhook → entitlement flip → publish unblocked; portal cancel → entitlement downgrade.

- [ ] Custom-domain attach/verify/TLS (spec 13, spec 10).
  - Backend: extend `internal/domains/` `DomainService` beyond `List` with create/verify/delete; write/read the existing `verification_token` column; integrate certmagic or autocert (decide in spec 13 update).
  - Endpoints: `POST/PATCH/DELETE /api/sites/:siteId/domains` in `internal/api/server.go`.
  - Frontend: domain attach UI under `apps/web/src/routes/app.sites.$siteId.settings.*` with DNS instructions and verification state.
  - Gating: behind paid entitlement from the billing module.
  - Acceptance: paid user attaches `example.com`, verifies via DNS TXT, TLS issues automatically, public render serves on that host.

- [ ] Productionize publish-artifact pipeline (spec 09, spec 16).
  - Replace the `npm run --workspace @snaelda/web render:artifacts` shell-out at `internal/publishing/artifacts.go:80-89` with an in-process renderer or a long-lived render worker; document the chosen path.
  - Move artifact storage from `internal/publishing/local_artifacts.go` to S3/SeaweedFS (assets already use this); update read path in public resolution.
  - Fix `Cache-Control` on artifact responses so CDN caching works (replace `no-store` with versioned immutable caching + purge on publish).
  - Fire cache invalidation on domain activate/deactivate and hostname change, not only on publish/rollback.
  - Replace the in-memory render cache with a shared cache (Redis or per-artifact CDN purge) so replicas stay coherent.
  - Acceptance: a multi-replica deploy serves published artifacts from object storage with correct cache semantics and invalidation.

- [ ] Remove dead placeholders and prune unused env (specs 02, 10).
  - Delete `internal/pages` and `internal/blocks` packages and their `mountAuthenticatedPlaceholderModule` lines in `internal/api/server.go`. (`internal/workspaces` stays as the intentional tenancy placeholder.)
  - Remove unused env declarations from `.env.example`: `UNSPLASH_*`, `RAILWAY_API_TOKEN`, `API_BASE_URL` (and any `STRIPE_*`/`BILLING_*` not actually read after the billing module lands).
  - Spec 10 update: document the in-code-but-spec-missing routes (`POST /api/sites`, navigation PUT/reorder, preview-token routes, public asset routes, public render/artifact routes).

- [ ] Backfill spec 06 and fix schema drift.
  - Add `auth_sessions`, `draft_revisions`, `site_preview_tokens`, `form_submission_attempts` table definitions to `specs/06-database-design.md`.
  - Make `page_view_daily.page_id` nullable in a new migration to match spec semantics for site-wide totals; or update spec 06 to match the current NOT NULL, whichever is correct for the analytics UX.
  - Add the new `guest_sessions`, `magic_links`, and `billing_*` tables once the corresponding modules ship.

- [ ] Builder polish.
  - Wire `updateAsset` / `deleteAsset` (already in `apps/web/src/lib/api.ts`) into the asset library UI with rename + delete affordances.
  - Add a dedicated submissions route under `apps/web/src/routes/app.sites.$siteId.submissions.tsx` with a list/detail two-pane, delete endpoint + UI, and a CSV export action; backend additions in `internal/forms/`.
  - Add "set homepage" UI under page settings, persisting via a sites mutator update; today the homepage is fixed at site creation.
  - Add footer navigation: extend `NavigationConfig` in `internal/siteconfig/` beyond `Primary` to include `Footer`, surface in the navigation editor and renderer.
  - Add external-link well-formedness validation on nav items.
  - Preserve hidden-block ordering across hide/show + reorder (spec 08 expectation; today they always append at end).
  - Add a standalone pre-publish validation panel that surfaces broken nav, missing homepage, draft-only references, etc. instead of waiting for publish-error responses.

- [ ] Generation hardening (spec 07).
  - Add per-user / per-workspace generation rate limit + cost guard in `internal/generation/handler.go` (share the durable rate-limit pattern from `form_submission_attempts`).
  - Tighten per-block prop schemas in the strict-mode OpenAI schema at `internal/generation/openai.go:354` (currently `additionalProperties: true` on block props); add JSON-schema fragments per block type in `internal/siteconfig/blocks.go`.
  - Invert page-reprompt behavior in `internal/generation/service.go:1158-1178` so the model is the primary path and template is the fallback (spec 07 intent).
  - Make image `alt` required at the backend level in `internal/siteconfig/blocks.go:923` (currently only renderer-side fallback).
  - Add a frontend URL safety pass in `apps/web/src/components/SiteDraftRenderer.tsx:1352-1397` rather than trusting backend allowlist alone.
  - Add `preferredLanguage` / `optionalHints` plumbing through the generation input contract.
  - Scaffold `migrateFromPrevious` per block type so future block versions can ship without breaking existing snapshots.

- [ ] Align theme tokens with spec 11.
  - Rename / extend CSS variables in `apps/web/src/lib/styles.ts` and themed renderer paths to expose `--color-mutedText`, `--color-primaryText`, `--font-*` weights, `--radius-buttonRadius`, `--space-sectionPaddingX`, `--space-sectionPaddingY` (currently using `--site-*` and a single `sectionSpacing`).
  - Update theme regeneration output and `internal/siteconfig/themes.go` to emit the new vocabulary.
  - Emit a per-site standalone `theme.css` artifact at publish so external embeds can consume it.

## Lower-Priority Product Follow-Ups

- [ ] Add optional early blocks only if user testing shows real demand: logo cloud, map/location, stats/KPIs, article teaser, or allowlisted embeds.
- [ ] Add safe placeholders or gradients for missing imagery if uploaded/starter assets are not present.
- [ ] Add site-level SEO editing and richer metadata workflows if page-level SEO plus publish-generated metadata stop being enough.
- [ ] Add form email forwarding once the transactional email subsystem is live.
- [ ] Consider block-level prompting only after site-level and page-level prompting are stable in real usage.
- [ ] Add an `archived` site state and an artifact-retention/pruning policy.
- [ ] Split `site.generate` audit events into `generation.complete` / `generation.fail` once analytics needs it.

## Explicit Deferrals

- [ ] Do not build arbitrary user code injection.
- [ ] Do not build custom CSS or custom JavaScript editing.
- [ ] Do not build full drag-and-drop layout freedom.
- [ ] Do not build a Webflow-style design editor.
- [ ] Do not build marketplace or third-party blocks.
- [ ] Do not build e-commerce checkout inside generated customer websites.
- [ ] Do not build complex CMS collections.
- [ ] Do not build multi-language sites.
- [ ] Do not build advanced teams, roles, or client collaboration until the single-workspace MVP works.
- [ ] Do not build per-customer frontend deployments.
- [ ] Do not add raw analytics event storage unless aggregated daily counts are insufficient.
