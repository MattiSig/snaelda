# Implementation Plan

Refreshed 2026-05-21 from 16 spec-vs-code audits plus the newly-authored `specs/20-ai-authoring-ux.md`. The MVP shape is unchanged: anonymous prompt → generated site → editable draft → published hosted subdomain, with optional account claim and Stripe billing. Open items are sorted by priority tier; "Recently Confirmed Complete" preserves shipped history.

## Recently Confirmed Complete

- [x] Collections module Phase 2 (spec 19): `collection_detail` templates now expand at publish time into one rendered HTML page per published entry under `/{collection.slug}/{entry.slug}`, with `block.bindings` substituting entry field values into the template's bound props before SSR. `collection_list` and `collection_index` blocks resolve their entry list from the snapshot at render time, link to the per-entry URLs, and the `stats` block ships its missing renderer alongside. Publish validation expands each template into expected entry paths, refuses templates whose collection has no published entries, and the manifest + sitemap include the per-entry URLs.
- [x] Asset upload and image-library UI exist in the builder, including uploaded-asset selection in block editors.
- [x] Contact-form submission storage and the chosen MVP moderation flow exist; email forwarding remains optional follow-up work, not unfinished core behavior.
- [x] The 10-page limit is already enforced in validation, generation repair, the database, and publish preflight.
- [x] Main builder loading, empty, and error states exist for login, site list, site detail, preview, publish history, assets, and submissions.
- [x] Page-level SEO editing plus publish-time `sitemap.xml`, `robots.txt`, canonical metadata, and basic social metadata exist.
- [x] Refresh-token rotation is server-side and hashed; publish/rollback cache invalidation already exists.
- [x] Added [specs/16-runtime-lifecycles-and-analytics.md](./specs/16-runtime-lifecycles-and-analytics.md) to define public visibility rules, domain/runtime semantics, and MVP analytics scope.
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
- [x] `GET /api/sites/{siteId}/analytics?window=7d|30d|all` plus builder analytics view at `/app/sites/{siteId}/analytics`.
- [x] Transactional email foundation exists in `internal/email/` with stdout, Mailpit SMTP, Resend HTTP, and memory transports; paired templates; `email_send_attempts` migration; Mailpit in local compose.
- [x] Navigation primary list is first-class editable canonical data with `PUT /api/sites/{siteId}/navigation` and builder editor for rename, external links, reorder, remove.
- [x] Backend-owned starter imagery via Pexels in `internal/imagery`, re-hosted as `assets` rows with `provenance` and credits on public pages.
- [x] Durable spam handling for public forms: honeypot, deterministic spam scoring, `form_submission_attempts` table.
- [x] Authoring-lifecycle audit events for `site.create`, `site.delete`, `page.delete`, `block.delete`, `site.generate`, `site.reprompt`, `page.reprompt`, `asset.upload`, `asset.delete`.
- [x] All 12 spec-required block types implemented in `internal/siteconfig/blocks.go` with registry contract test, per-block prop schemas, plain-text enforcement, URL allowlist, and form-field allowlist.
- [x] Preview tokens (hashed, TTL, revocable) and public render via hostname or slug.
- [x] Block CRUD (DnD + button reorder), page CRUD with SEO and nav inclusion, draft/preview/publish with versions/rollback, site- and page-level reprompt with undo.
- [x] CSRF middleware, HttpOnly+Secure+SameSite=Lax cookies, durable per-IP rate limiting, URL allowlist, plain-text-rejects-HTML.
- [x] Public + authenticated response-header policy enforced in `internal/api/server.go` with explicit CSP, HSTS, frame/type/referrer headers, and `Cache-Control: private, no-store`.
- [x] Guest-trial / claim subsystem end to end (specs 17, 06, 10, 12): `guest_sessions`, `magic_links`, guest-authored preview tokens, unified session resolution, recovery-key issuance/restoration, magic-link claim/login.
- [x] Billing module scaffold: real `internal/billing/` module, `billing_customers`/`billing_subscriptions`/`billing_entitlements`/`billing_events` tables, checkout/portal/entitlements/webhook endpoints with Stripe signature verification and idempotency, paid plan gating for new sites/prompt allowance/asset storage, billing routes in `apps/web/src/routes/app.billing.*`.
- [x] Once-over purchase flow persists `once_over_requests`, updates `workspaces.once_over_status`, sends `once_over_intake_ready` from Stripe webhook.
- [x] Trial publish to hosted subdomain unblocked (spec 17): publish removed from `blockedTrialRequest` so unclaimed trial users can publish to their hosted subdomain while custom-domain writes stay gated.
- [x] `writeSessionCookies` now refreshes guest-session and CSRF cookies for active trial users (regression test covers trial vs authenticated sessions).
- [x] Stripe webhook (`/api/billing/webhook`) added to the CSRF exemption list in `internal/api/server.go`, with a regression test confirming POSTs no longer 403.
- [x] Publish and rollback gate on `billing.EnforceSiteLimit` (mirroring `internal/sites/handler.go`) and surface `plan_limit_exceeded` to the builder publish panel.
- [x] Per-IP rate limiting on auth endpoints (`magic-link request`, `magic-link verify`, `recovery-restore`, `recovery-issue`) via durable `auth_rate_limit_attempts` table; verify path tightened to spec-18 3/hour.
- [x] Public artifact serving now requires manifest membership: `loadPublishedArtifact` validates the requested path against `manifest.Files` (with legacy fallback to known well-known artifacts) and the bundle builder emits the explicit allowlist.
- [x] Collections module Phase 1 foundation (spec 19): `000013_collections.sql` adds `collections` + `collection_entries` tables plus `pages.type`, `pages.collection_id`, and `block_instances.bindings`; new `internal/collections/` module with CRUD handlers mounted at `/api/sites/:siteId/collections{,/:id}{,/entries}{,/:entryId}{,/reorder}`; `siteconfig` adds the closed `FieldDefinition` registry (15 field types), `Collection`, `CollectionEntry`, `Page.Type`, `Page.CollectionID`, `BlockInstance.Bindings`, and snapshot-time validation that bindings only appear in `collection_detail` templates and target compatible field types; new `collection_list`, `collection_index`, `collection_detail`, and `stats` blocks; sites mutator surfaces `Type` and `CollectionID` on page create/update; SiteDraft load/save persists collections + entries atomically; published snapshots include collections with only `status=published` entries; frontend gets `Collection`, `CollectionEntry`, `FieldDefinition` types, full API client, and a builder route at `/app/sites/:siteId/collections` with collection list, schema editor, and entry editor (create / publish / delete). Per-entry URL rendering moves to the spec-19 Phase 2 line item above.
- [x] Productionize publish-artifact pipeline (specs 09, 16):
  - Replaced the `npm run` per-publish shell-out with a long-lived Node render worker (newline-delimited JSON over stdin/stdout) in `internal/publishing/worker_renderer.go`, restarting on crash.
  - Added `internal/publishing/s3_artifacts.go` plus a `PUBLISHED_ARTIFACTS_BACKEND=s3` toggle so published artifacts are persisted to the S3/SeaweedFS bucket already in compose.
  - Publish now cleans up orphan artifacts when the commit fails, and rollback refuses to promote a version whose manifest is missing from the store.
  - Artifact responses now carry an `ETag` keyed on the published version plus tiered `Cache-Control` (HTML revalidates quickly; static crawl files cache longer) with built-in `If-None-Match` → 304 support and an injectable `CDNPurger` hook fired on publish/rollback.
  - Cache exposes `InvalidateHostname`/`InvalidateSite` so the upcoming custom-domain write API can invalidate without depending on a publish event.
  - `validateArtifactBundle` now asserts each rendered page's HTML body via balanced-tag and closing-tag structural checks, rejecting truncated or empty renders before they ship.

## Core Spec Gaps

- [ ] Custom-domain attach/verify/TLS (specs 13, 10).
  - Extend `internal/domains/` `DomainService` beyond `List` with create/verify/delete; populate the existing `verification_token` column; integrate certmagic or autocert.
  - Add `POST/PATCH/DELETE /api/sites/:siteId/domains` in `internal/api/server.go`.
  - Settings UI under `apps/web/src/routes/app.sites.$siteId.settings.*` with DNS-TXT instructions and verification state, gated by paid entitlement (replace the static "locked" hint).
  - Acceptance: paid user attaches `example.com`, verifies via DNS TXT, TLS issues automatically, public render serves on that host.

- [ ] Introduce the `brand` entity as a first-class typed object (specs 3, 5, 11).
  - Add a `BrandConfig` Go struct, `brand` jsonb column on `sites`, top-level `brand` in `SiteDraft` (Go + TS), and a validator.
  - Wire `brand.primaryColor` into theme regeneration so palette derivation per spec 11 is actually possible.
  - Plumb brand through generation input (along with `preferredLanguage` / `optionalHints` — see Generation Hardening).

- [ ] Align theme tokens with spec 11 vocabulary.
  - Replace `--site-*` vars in `apps/web/src/lib/styles.ts` and themed renderer paths with `--color-*`, `--font-*`, `--radius-*`, `--space-sectionPaddingX/Y`.
  - Add `headingWeight` / `bodyWeight` typography tokens and split single `sectionSpacing` into `sectionPaddingX` + `sectionPaddingY`.
  - Update `internal/siteconfig/themes.go` and the per-site `theme.css` artifact (already emitted) to use the new vocabulary.

- [ ] Add footer navigation (specs 4, 11).
  - Extend `NavigationConfig` in `internal/siteconfig/` beyond `Primary` to include `Footer`; surface in the navigation editor and renderer; thread through validation.
  - Replace `validateFooterProps` (knows only `siteName`/`tagline`/`contactLine`) with structured `contact.{address,phone,email,hours}` + `showBrand` so LocalBusiness JSON-LD is generatable.

- [ ] Make the block registry contract test cover every spec-required type by name, not just fixtures (the `stats`, `collection_list`, `collection_index`, and `collection_detail` blocks now exist but the contract test still asserts only sorted order, not spec coverage by name).

- [ ] Make image `alt` required end-to-end (specs 4, 7).
  - `optionalImage` in `internal/siteconfig/blocks.go:923` allows missing alt; spec requires it at every layer including backend repair. Renderer must not silently fall back to `altFallback`.

- [ ] Emit publish-time LocalBusiness JSON-LD and `og:image` (spec 9).
  - Generate JSON-LD from the new structured footer/contact fields; add default `og:image` derivation from the hero asset.
  - Fix sitemap namespace from `https://www.sitemaps.org` to `http://www.sitemaps.org` per spec.

- [ ] Propagate `Page.status` (specs 3, 5, 6).
  - Column exists in DB but is never lifted into `PageDraft` (Go or TS); add to draft contract, generation output, validation, and the builder page settings UI.

## AI-First UX (Spec 20)

Entire spec is greenfield. None of it is implemented yet.

- [ ] Generation-progress streaming.
  - SSE/job-tracking endpoint backed by a new `generation_jobs` table; emit the 8 spec-defined step labels from `internal/generation/service.go`.
  - Builder consumer in `apps/web/src/routes/` to render progress on the anonymous-prompt and site-reprompt paths.

- [ ] Reprompt history + diff view.
  - New `reprompt_history` table capturing prompt, before/after draft hash, scope (site/page/block), and timestamp.
  - API: list and fetch-diff endpoints; revert hook reusing existing undo plumbing.
  - UI: history panel with diff renderer in the builder.

- [ ] AI-suggest dropdowns on blocks.
  - Per-block "suggest alternatives" affordance hitting a new generation endpoint that returns N variants for the selected block, scoped to its prop schema.

- [ ] "Find a better image" affordance on image-bearing blocks.
  - Reuse `internal/imagery/` Pexels integration plus model-side query rewriting; surface a picker in the block editor.

- [ ] Page-suggest empty state.
  - Empty-page CTA that asks the model for likely page candidates (Pricing, About, Contact, etc.) given current site context.

- [ ] Ambient site suggestions.
  - Lightweight periodic suggestions in the builder shell (e.g., "Add an FAQ section?") driven by a low-frequency model call over the current draft.

## Hardening

- [ ] Generation hardening (spec 7).
  - Turn on OpenAI strict-mode for the site-generation schema (currently only on for theme regen) in `internal/generation/openai.go`.
  - Tighten per-block prop schemas at `internal/generation/openai.go:354` (currently `additionalProperties: true`); add JSON-schema fragments per block type in `internal/siteconfig/blocks.go`.
  - Invert page-reprompt behavior in `internal/generation/service.go:1158-1178` so the model is primary and template is fallback (currently template overwrites the AI plan).
  - Add per-user / per-workspace generation rate limit + cost guard in `internal/generation/handler.go` using the durable pattern from `form_submission_attempts`.
  - Add `preferredLanguage` / `optionalHints` / `brand` plumbing through the generation input contract.
  - Scaffold `migrateFromPrevious` per block type so future block versions ship without breaking snapshots.
  - Add a frontend URL safety pass in `apps/web/src/components/SiteDraftRenderer.tsx:1352-1397` rather than trusting backend allowlist alone.

- [ ] Magic-link verify rate limit (spec 18).
  - `GET /api/auth/magic` currently uses login limits (5/15min, 20/24h); spec 18 mandates 3/hour for verification. Tighten in `internal/auth/handler.go`.

- [ ] Idempotency key on `once_over_intake_ready` email (spec 18).
  - Webhook replays would double-send. Key the email send on Stripe event ID, matching the billing-receipt pattern.

- [ ] Align sessions route naming (specs 17, 10).
  - Spec 17 says `GET /api/sessions/current` and `POST /api/sessions/attach-email`; code uses `/me` and `/claim`. Update the spec to match shipped reality (code is already wired and tested).

## Polish & Follow-Ups

- [ ] Finish the Once-over delivery workflow (specs 15, 18).
  - `SendOnceOverDelivered` exists but is never called; `UpdateOnceOver` input has no `videoUrl`/`deliveredAt`; `sanitizeOnceOverVideoURL` is dead code.
  - Build an operator/admin route to mark delivery and trigger the email; remove the dead helper or wire it.

- [ ] Trial education affordances (spec 17).
  - "Continue your draft" landing affordance for cookie-detected sessions.
  - First-generation education modal explaining L0 trial state.
  - Add "Already have an account → magic-link login" option to the "Save your workspace" panel.

- [ ] Builder polish (spec 8).
  - Wire `updateAsset` / `deleteAsset` (already in `apps/web/src/lib/api.ts`) into the asset library UI with rename + delete affordances.
  - Add a dedicated submissions route at `apps/web/src/routes/app.sites.$siteId.submissions.tsx` with list/detail two-pane, delete endpoint + UI, CSV export; backend additions in `internal/forms/`.
  - Add "set homepage" UI under page settings, persisting via a sites mutator update.
  - Add external-link well-formedness validation on nav items.
  - Preserve hidden-block ordering across hide/show + reorder (today they always append at end).
  - Add a pre-publish validation panel surfacing broken nav, missing homepage, draft-only references, etc.
  - Scope page reprompt undo correctly (today it hits the site undo endpoint) and expand beyond single-slot history.

- [ ] Remove dead placeholders and prune unused env (specs 2, 10).
  - Delete `internal/workspaces/`, `internal/pages/`, `internal/blocks/` packages (each only contains `module.go` with `Name()`) and their `mountAuthenticatedPlaceholderModule` lines at `internal/api/server.go:187,197,198`. Spec 2 explicitly says "no workspaces module for MVP."
  - Remove unused env declarations from `.env.example`: `UNSPLASH_*`, `RAILWAY_API_TOKEN`, `API_BASE_URL` (and any `STRIPE_*`/`BILLING_*` not actually read).

- [ ] Finish remaining transactional-email follow-ups (spec 18).
  - Hook `internal/email/` into the remaining Once-over delivery/admin call sites once the operator workflow ships.
  - Enforce spec 18 send windows when the delivery workflow lands.
  - Acceptance verification: Mailpit/Resend round-trips for magic-link login, form forwarding, billing notices, Once-over intake-ready.

- [ ] Acceptance test the billing tri-block.
  - claim → checkout → webhook → entitlement flip → publish unblocked; portal cancel → entitlement downgrade.

## Spec Debt

- [ ] Backfill spec 6 and fix schema drift.
  - Add `auth_sessions`, `draft_revisions`, `site_preview_tokens`, `form_submission_attempts`, `guest_sessions`, `magic_links`, `billing_*`, `once_over_requests` to `specs/06-database-design.md`.
  - Decide and document: `page_view_daily.page_id` nullable (for site-wide totals) vs. current NOT NULL — pick one and align migration or spec.

- [ ] Backfill spec 10 with shipped-but-undocumented routes.
  - `POST /api/sites`, navigation PUT/reorder, preview-token routes, public asset routes, public render/artifact routes, sessions endpoints with the agreed naming.

- [ ] Align spec 17 with shipped sessions naming (see Hardening item above).

## Lower-Priority Product Follow-Ups

- [ ] Add optional early blocks only if user testing shows real demand: logo cloud, map/location, article teaser, or allowlisted embeds.
- [ ] Add safe placeholders or gradients for missing imagery if uploaded/starter assets are not present.
- [ ] Add site-level SEO editing and richer metadata workflows if page-level SEO plus publish-generated metadata stop being enough.
- [ ] Add form email forwarding follow-ups once Once-over delivery is live.
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
- [ ] Do not build complex CMS collections beyond the spec 19 scope.
- [ ] Do not build multi-language sites.
- [ ] Do not build advanced teams, roles, or client collaboration until the single-workspace MVP works.
- [ ] Do not build per-customer frontend deployments.
- [ ] Do not add raw analytics event storage unless aggregated daily counts are insufficient.
