# Implementation Plan

Refreshed 2026-05-27 after landing the AI-suggest dropdowns on blocks (spec 20 — the biggest open AI-first surface), the full-page hero variant, the Stripe setup script, and the first wave of landing/preview/generation UX hardening. Prior refresh (2026-05-21) followed 16 spec-vs-code audits and the newly-authored `specs/20-ai-authoring-ux.md`. The MVP shape is unchanged: anonymous prompt → generated site → editable draft → published hosted subdomain, with optional account claim and Stripe billing. Open items are sorted by priority tier; "Recently Confirmed Complete" preserves shipped history.

## Next Up (re-prioritized 2026-05-27)

Highest-leverage open work, in order:

1. **"Find a better image" on image-bearing blocks** (spec 20) — small surface, reuses `internal/imagery/` Pexels pipeline plus a model-side query rewrite. Strong wow for image-led sites (which our ICP is full of).
2. **Trial education affordances** (spec 17) — "Continue your draft" landing affordance and first-generation modal explaining L0 trial state. Required by the GTM story; today users hit the cookie-only failure mode silently.
3. **Generation streaming for perceived speed** (extends spec 20) — switch the OpenAI call to streaming so pages/blocks materialize as the model writes them, instead of a 30-60s dead-wait at `plan.blocks`. Highest perceived-speed win available without changing the model.
4. **Once-over delivery workflow** (specs 15, 18) — finish the half-built path. Operator UI to mark delivery + trigger `SendOnceOverDelivered`. Unlocks the $99 add-on revenue.
5. **Page-suggest empty state** (spec 20) — empty-page CTA that asks the model for likely candidates (Pricing, About, Contact). Smaller scope than block-suggest.

Defer: ambient site suggestions (lowest urgency), automated TLS issuance (platform-dependent — Railway/Render/Fly handle this for us).

## Recently Confirmed Complete

- [x] AI-suggest dropdowns on blocks (spec 20): new `POST /api/sites/:siteId/blocks/:blockId/suggest` endpoint, backed by a new `BlockSuggester` interface implemented by `OpenAIPlanner.SuggestBlockProps` that rewrites only the block's props through a strict structured-output JSON schema derived from the block's existing `PropSchema` (same type/version preserved by construction). The service captures before/after `draft_revisions`, writes a `reprompt_history` row with `scope='block'` and a model-authored `change_summary`, and tracks the work in `generation_jobs` under the new `JobKindBlockSuggest` kind. Migration `000018_block_suggest.sql` extends the `reprompt_history.scope`, `draft_revisions.scope`, and `generation_jobs.kind` check constraints. The builder's `BlockEditor` now surfaces an "Improve with AI" dropdown (Tighten / Expand / Change tone → friendlier/professional/playful/direct / Rewrite from prompt) on text-bearing blocks; the dropdown closes on Esc and click-outside, the local prop state resets to the AI result via React 19's render-time state-derivation pattern, and toast status messages surface success. Wired through `PuckBuilder.tsx` and `apps/web/src/routes/app.sites.$siteId.index.tsx` with paid-plan/billing guard reuse. Backend + frontend tests added; verified end-to-end via Playwright against a freshly generated florist site (hero headline tightened, reprompt history shows the block-scoped entry, no console errors).
- [x] Full-page hero variant (specs 4, 7): hero blocks now carry a `variant` enum (`standard` / `full-page`) in addition to the existing `layout` field. `internal/siteconfig/blocks.go` validates the new enum, `apps/web/src/components/SiteDraftRenderer.tsx` renders the immersive variant as a true 100vh section that overlaps the page header on first paint (image background, vertical gradient overlay, headline + optional CTAs anchored at the bottom), and the page header recolors to light when the first block is a full-page hero. The OpenAI planner's system prompt teaches the model when to choose `full-page` (image-led brands, magazine-style openers); `repairHeroProps` normalizes the enum on the legacy fallback path. `specs/04-block-registry.md` updated; the block-registry contract golden file regenerated; LLM-aware verified against an Icelandic florist generation.
- [x] Stripe setup automation: new `cmd/stripe-setup` Go binary creates products + prices (Basic / Pro / Once-over) and the webhook endpoint with exactly the six events `internal/billing/stripe.go` handles, then prints env vars to paste into `.env`. Idempotent via Stripe `lookup_key` (`snaelda_basic_monthly`, `snaelda_pro_monthly`, `snaelda_once_over`) — safe to re-run. Refuses `sk_live_…` keys without `-allow-live` plus a typed "yes" confirmation. Wired into the Makefile as `make stripe-setup`.
- [x] Immersive preview share route (specs 9, 14): `apps/web/src/routes/preview.$token.tsx` no longer wraps the renderer in editor-style chrome; the `SiteDraftRenderer` renders as the entire page with a small non-blocking "Draft preview" pill pinned to the bottom-right. `apps/web/src/routes/__root.tsx` now excludes `/preview/` (like `/public/` and `/app`) from the app topbar, so the share link experience matches what the recipient will see when the site is published.
- [x] Generation progress hardening (spec 20): `internal/generation/progress.go` made `progressTracker.emit` monotonic (records highest step index reached, drops backward emits). Fixes the regression where the planner's retry loop would rewind the visible step from `plan.blocks` back to `plan.pages`. `internal/generation/openai.go` adds a heartbeat goroutine that walks through `plan.theme` (+7s) and `plan.blocks` (+16s) during the LLM HTTP call so the UI shows deliberate motion before the LLM returns; `newOrderedEmitter` guarantees idempotent emission so the post-HTTP catch-up is safe.
- [x] Landing-page simplification (spec 17): the "Enter" dropdown that offered Email / Restore session / Continue as guest sub-choices was replaced with a single subtle "Log in" link in the top-right. The prompt form already lands directly in a trial workspace via `startAnonymousSession()` + `navigate({ to: '/app', search: { prompt }})`. `?restore=` URL auto-restore still works; the restore-failed message now renders inline above the prompt form. Aligns the surface with spec 17's "no signup required" promise.

- [x] Reprompt history + diff view (spec 20): page- and site-scoped reprompts now capture immutable before/after `draft_revisions` plus durable `reprompt_history` rows through `000017_reprompt_history.sql`, with `GET /api/sites/:siteId/reprompts`, `GET /api/sites/:siteId/revisions/:revisionId`, and `POST /api/sites/:siteId/reprompts/:id/revert` layered into `internal/generation`. The legacy `/api/sites/:siteId/undo` path now restores the latest non-undone reprompt checkpoint via the same history plumbing. In the builder, prompt iteration now includes a scoped History panel with whole-site vs selected-page filters, per-entry revert, and a diff modal that compares stored revision snapshots block-by-block before the user keeps or rolls back a rebuild.
- [x] Generation hardening (spec 7): the OpenAI planner now sends a strict block-union JSON schema built from per-block prop fragments in `internal/siteconfig/blocks.go`, replacing the old `additionalProperties: true` hole in generated block props. Page reprompt is now model-first, using the AI-authored page when available and only falling back to deterministic templates when planning fails or returns nothing usable. Generation requests now pass through a durable `generation_attempts` limiter with per-user and per-workspace burst/day windows plus a prompt-length cost guard in `internal/generation/handler.go`, and `BlockDefinition` now carries per-type `MigrateProps` scaffolding so future block-version migrations have a first-class home. The frontend renderer also now sanitizes outbound hrefs before rendering, so even malformed drafts degrade to `#` instead of trusting unsafe protocols client-side.
- [x] Custom-domain management foundation (specs 13, 10): `internal/domains` now supports `POST/PATCH/DELETE /api/sites/:siteId/domains` on top of the existing read path, with paid-plan enforcement via billing entitlements, hostname validation, DNS-TXT verification using `_snaelda-verify.{hostname}`, and safe refusal of hosted `subdomain` records. Domain list responses now expose pending verification instructions plus active-domain public URLs, and active custom domains become the preferred live URL in the builder. The publish/runtime cache now shares a domain cache between `internal/publishing` and `internal/domains`, so attach/verify/delete invalidates stale hostname lookups immediately. The builder publish panel now includes a production-facing custom-domain manager with add, verify, remove, DNS instructions, and paid-plan lock messaging.
- [x] Theme token vocabulary now matches spec 11 end to end. `internal/siteconfig/theme_presets.go` emits `sectionPaddingX` + `sectionPaddingY` instead of the old single `sectionSpacing`, while `DetectThemeSelection` and the validator stay backward-compatible with legacy snapshots that still carry `sectionSpacing`. The public renderer and published `assets/theme.css` now use the spec-facing CSS variable families (`--color-*`, `--font-*`, `--radius-*`, `--space-sectionPaddingX/Y`) instead of the old `--site-*` contract, and theme CSS now exposes `headingWeight` / `bodyWeight` alongside the existing font-family tokens.
- [x] `once_over_intake_ready` email sends are now idempotent on Stripe webhook event ID (spec 18 hardening). `internal/email/helpers.go` accepts an idempotency key for the once-over intake-ready template, `internal/billing/once_over.go` passes `once_over_intake_ready:{event_id}`, and billing tests assert the key is present so webhook replays do not double-send through providers that honor idempotency headers.
- [x] Brand as a first-class typed object (specs 3, 5, 11): new `BrandConfig` Go struct + TypeScript type carry `businessName`, optional `{assetId, alt}` logo, and `primaryColor`; SiteDraft and PublishedSnapshot expose `brand` alongside theme; new `000014_site_brand.sql` adds a `brand jsonb` column on `sites` and the sites reader/writer persist it atomically with the rest of the draft. The siteconfig validator enforces hex colors and required-on-publish; published snapshots fall back to `site.name` + theme primary so legacy drafts still ship. `siteconfig.BuildThemeWithBrand` overrides the palette's `primary` token with `brand.primaryColor` so theme update + regenerate continue to use brand as the source of the rendered primary, and theme regeneration passes the current brand to the model as a constraint. Generation seeds brand from the site name and selected palette primary, the generation input contract carries an optional brand from callers, and `applySiteIdentity` preserves brand across site reprompt.
- [x] Brand follow-through (specs 3, 5, 7, 11): `BuildThemeWithBrand` now derives `secondary`, `accent`, `surface`, `surfaceMuted`, `border`, `muted`, and `ring` deterministically from `brand.primaryColor` instead of only swapping `primary`; the site settings panel now edits `brand.businessName`, `brand.primaryColor`, and `brand.logo`; and generation input now carries `preferredLanguage`, `optionalHints`, and `brand` through the API/service/OpenAI payload path with strict structured-output enabled for the main site-plan call.
- [x] Page status contract (specs 3, 5, 6): `PageDraft` now carries `status`, the sites reader/writer persist the existing `pages.status` column end to end, generation seeds pages as `draft`, the API accepts status updates, and the builder page-setup panel exposes a draft/published selector.
- [x] Image alt is now required end to end (specs 4, 7): image refs and brand logos require non-empty `alt`, validator coverage rejects missing alt text, renderer no longer silently falls back to generated labels, and repair/starter-imagery paths always emit an alt string so publish-time validation stays strict.
- [x] Block registry contract coverage now asserts every spec-required type by name, including `stats`, `collection_list`, `collection_index`, and `collection_detail`, instead of only relying on sorted fixture order.
- [x] Magic-link verify rate limit (spec 18): `internal/auth/rate_limiter.go` enforces the shipped 3/hour verification rule and tests cover the dedicated `magic_link_verify` bucket.

- [x] Collections module Phase 2 (spec 19): `collection_detail` templates now expand at publish time into one rendered HTML page per published entry under `/{collection.slug}/{entry.slug}`, with `block.bindings` substituting entry field values into the template's bound props before SSR. `collection_list` and `collection_index` blocks resolve their entry list from the snapshot at render time, link to the per-entry URLs, and the `stats` block ships its missing renderer alongside. Publish validation expands each template into expected entry paths, refuses templates whose collection has no published entries, and the manifest + sitemap include the per-entry URLs.
- [x] Footer/navigation/SEO spec-11 follow-through: `NavigationConfig` now carries both `primary` and `footer` link lists end to end, the builder navigation editor saves both sections, and the Footer renderer resolves its links from canonical site navigation instead of footer-local props. Footer blocks now use structured `contact.{address,phone,email,hours}` plus `showBrand`, and Header/Footer resolve `brand.businessName`/`brand.logo` from site context at render time. Published artifact manifests now carry derived `ogImageUrl` and `localBusinessJsonLd`, the public page head emits `og:image`, `twitter:image`, and `LocalBusiness` JSON-LD when the footer includes structured address/hours, and sitemap XML now uses the spec-required `http://www.sitemaps.org` namespace.
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

- [ ] Automated TLS issuance for custom domains when the app terminates TLS directly (spec 13).
  - Attach/verify/delete flows, paid gating, DNS-TXT instructions, and active custom-domain public routing are now shipped.
  - Remaining work is only the deployment-specific ACME/certificate automation layer (`certmagic`, `autocert`, or equivalent) for environments where Snaelda itself, rather than an upstream proxy/platform, is responsible for certificate issuance.

## AI-First UX (Spec 20)

Entire spec is greenfield. None of it is implemented yet.

- [x] Generation-progress streaming (SSE).
  - SSE/job-tracking endpoint backed by a new `generation_jobs` table; emit the spec-defined step labels from `internal/generation/service.go`, keep jobs alive if the SSE client disconnects, and fall back to `GET /api/generation/jobs/:jobId` polling from the frontend.
  - Builder consumers now render streamed progress on the anonymous-prompt, site-reprompt, page-reprompt, and theme-regeneration paths, with the shared `GenerationProgressCard` adapting to shorter step sets when imagery/copy phases are skipped.
  - Hardened 2026-05-27: `progressTracker.emit` is now monotonic so the planner's retry loop cannot rewind the visible step; `OpenAIPlanner.BuildPlan` runs a heartbeat goroutine through `plan.theme`/`plan.blocks` during the LLM HTTP call so the UI shows motion before the response arrives.

- [x] Reprompt history + diff view.
  - New `reprompt_history` table plus immutable before/after `draft_revisions` for current site/page rebuild scopes.
  - API: list, revision fetch, and revert endpoints; legacy undo now delegates to the newest non-undone history entry.
  - UI: history panel with scoped filters and a block-by-block diff modal in the builder.

- [ ] **Stream the OpenAI response itself.** (Next Up #4)
  - Today's heartbeat masks the 30-60s wait at `plan.blocks` but the LLM call is still non-streaming (`io.ReadAll` on the response body in `internal/generation/openai.go`).
  - Switching to SSE streaming on the upstream call and incremental JSON parsing would let pages/blocks materialize as the model writes them, instead of all-at-once. Single biggest perceived-speed win available without changing model.

- [x] AI-suggest dropdowns on blocks.
  - `POST /api/sites/:siteId/blocks/:blockId/suggest` accepts `{action: tighten|expand|tone|rewrite, tone?, instruction?}` and returns the updated draft plus jobId.
  - Backend constrains the model output to the block's existing `PropSchema` so block type/version cannot change; `OpenAIPlanner.SuggestBlockProps` issues a strict structured-output completion with a per-action system prompt that forbids HTML/Markdown and preserves enums and image/link fields unless the action requires touching them.
  - `BlockSuggester` interface keeps the service unit-testable without an OpenAI client.
  - Reprompt history scope expanded to `'block'`; each AI edit captures before/after `draft_revisions` and is individually revertable through the existing `/api/sites/:siteId/reprompts/:id/revert` endpoint. `generation_jobs.kind` now allows `'block_suggest'`. Billing prompt limit and the existing per-scope rate limiter are enforced.
  - Builder dropdown surfaces Tighten / Expand / Change tone (friendlier / more professional / more playful / more direct) / Rewrite from prompt on text-bearing blocks; Esc + click-outside close the dropdown; local editor state is reset to the AI result via render-time state derivation so the user sees the new copy immediately.

- [ ] **"Find a better image" affordance on image-bearing blocks.** (Next Up #2)
  - Reuse `internal/imagery/` Pexels integration plus model-side query rewriting derived from page name + nearby headline + block intent; surface a picker in the block editor.

- [ ] **Page-suggest empty state.** (Next Up #6)
  - Empty-page CTA that asks the model for likely page candidates (Pricing, About, Contact, etc.) given current site context.

- [ ] Ambient site suggestions.
  - Lightweight periodic suggestions in the builder shell (e.g., "Add an FAQ section?") driven by a low-frequency model call over the current draft.
  - Lowest urgency — defer until the higher-leverage AI surfaces above are shipped.

## Hardening

- [ ] Align sessions route naming (specs 17, 10).
  - Spec 17 says `GET /api/sessions/current` and `POST /api/sessions/attach-email`; code uses `/me` and `/claim`. Update the spec to match shipped reality (code is already wired and tested).

## Polish & Follow-Ups

- [ ] Finish the Once-over delivery workflow (specs 15, 18). **(Next Up #5)**
  - `SendOnceOverDelivered` exists but is never called; `UpdateOnceOver` input has no `videoUrl`/`deliveredAt`; `sanitizeOnceOverVideoURL` is dead code.
  - Build an operator/admin route to mark delivery and trigger the email; remove the dead helper or wire it.

- [ ] Trial education affordances (spec 17). **(Next Up #3)**
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
