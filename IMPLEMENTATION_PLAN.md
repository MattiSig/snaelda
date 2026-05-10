# Implementation Plan

This plan is sequenced for the shortest path to a working prototype first. The prototype should prove the core loop: create a structured site draft from a prompt, edit it, preview it, publish it, and serve it from a stable public URL. Everything else should be added only after that loop works end to end.

## Prototype Success Criteria

- [x] A signed-in user can create or use a default workspace.
- [ ] The backend is a Go modular monolith exposing the product API and owning Postgres persistence.
- [x] The frontend is a React application built with TanStack Start, Tailwind CSS, and shadcn/ui unless a later decision deliberately changes that stack.
- [x] Frontend surfaces support brand-aligned light and dark modes based on `BRANDING.md`, with dark mode required and using the sharper, slightly meaner palette described there.
  Verified on May 10, 2026 by running the builder locally at `http://localhost:3000`, confirming the new shared Tailwind + shadcn shell rendered in Playwright, toggling the app chrome between the dark default and the new warm light mode, and then signing in plus creating and previewing `Color Mode Verification Studio` without current-page console errors.
- [x] A user can enter a prompt and get a valid structured site draft.
  Verified on May 6, 2026 by logging in locally, generating `Loom & Light Studio` from a photography prompt in Playwright, confirming the created draft contained four validated pages in the builder, and loading the generated content on `/app/sites/:siteId/preview`.
- [x] The generated draft uses only known block types, known block versions, valid block props, valid theme tokens, and no arbitrary code.
- [x] The draft can be previewed through the maintained React renderer.
- [x] The user can edit basic block fields and save validated changes.
- [x] The user can publish the draft into an immutable snapshot.
- [x] The published site is reachable at a platform subdomain or local equivalent.
- [x] Published output is served from the published snapshot or generated artifacts, not from mutable draft tables.

## Phase 0: Technical Foundation

- [x] Choose the frontend framework and styling baseline: TanStack Start for the React app, Tailwind CSS for utility styling, and shadcn/ui for reusable app components.
- [x] Scaffold a Go backend as a modular monolith with clear internal packages for `auth`, `workspaces`, `sites`, `pages`, `blocks`, `themes`, `generation`, `publishing`, `domains`, `assets`, and `forms`.
- [x] Scaffold a React frontend app with separate route areas for marketing/auth entry, authenticated builder, draft preview, and public rendering experiments.
- [x] Decide whether the React app lives in the same repo as the Go backend as a monorepo, or as a separate app package with shared generated API types.
- [x] Configure the TanStack Start app with Tailwind CSS, shadcn/ui, the `@/*` import alias, shared `cn` utility, and a small starting set of UI primitives.
- [x] Set up Postgres connection, migrations, seed tooling, and local development environment for the Go backend.
- [x] Set up local S3-compatible object storage with SeaweedFS for image uploads and published artifact development.
- [x] Add Go API routing, middleware, config loading, request logging, validation error responses, and health checks.
- [x] Add frontend data fetching conventions for calling the Go API.
- [x] Add JWT-based authentication across the Go API and React frontend.
- [x] Implement Go JWT middleware that validates token signature, expiry, issuer, audience, subject, and required claims.
- [x] Use server-set secure HTTP-only cookies for browser auth tokens; do not store auth tokens in browser local storage.
- [x] Keep token issuance, refresh, logout, and revocation server-side in the Go backend.
- [x] Define access token lifetime, refresh token lifetime, refresh rotation, logout, and token revocation behavior.
  Auth behavior: access tokens default to 15 minutes via `AUTH_ACCESS_TOKEN_TTL`; refresh tokens default to 30 days via `AUTH_REFRESH_TOKEN_TTL`; refresh uses opaque HTTP-only cookie tokens stored only as hashes, rotates the refresh token on every refresh, extends the session expiry, and logout revokes the server-side session before clearing cookies.
- [x] Add React route guards and API client behavior for unauthenticated, expired-token, and forbidden responses.
- [x] Create automatic default workspace creation for each user.
- [x] Add shared authorization helpers that verify workspace membership and resource ownership.
- [x] Add a shared ID, slug, timestamp, and audit utility layer.
- [x] Establish runtime validation with schema tooling for site drafts, published snapshots, block props, theme tokens, navigation, URLs, and form definitions.
  Runtime validation now lives in `internal/siteconfig` with canonical draft/snapshot types, a Go-owned prototype block registry, theme/navigation/URL/form validators, and regression tests for unsafe URLs, unknown blocks, page constraints, theme tokens, and publish snapshot requirements. Asset ownership checks still need to be wired into save-time persistence once asset APIs exist.
- [x] Add backend test setup for schema validation, registry validation, publish validation, persistence, and API authorization.
  Backend coverage now spans `internal/siteconfig` schema + registry validation, `internal/publishing` snapshot/publish behavior, `internal/sites` draft persistence/assembly, and handler/authorization tests across site, generation, theme, and publish routes. Verified on May 10, 2026 with `make test` after adding registry definition/props edge-case coverage for unknown versions, unsafe links, unsupported block props, duplicate definitions, and published contract regressions.
- [x] Add frontend test setup for core builder flows and renderer smoke tests.
  Frontend coverage now runs through Vitest + Testing Library in `apps/web`, with passing tests for nested block-editor saves and hidden-state handling, shared renderer anchor/published-link resolution, and published snapshot loading/error states. Verified on May 10, 2026 with `npm run web:test`, `npm run web:lint`, and `npm run web:build`.
- [x] Reserve a `billing` backend module boundary for Stripe-backed workspace subscriptions, but keep payment implementation out of the first prototype loop.
  The Go API now mounts an authenticated placeholder `billing` module alongside the existing modular boundaries so Stripe-backed subscription work can land later without reshaping the server package layout.

## Phase 1: Data Model And Draft Persistence

- [x] Create `users`, `workspaces`, and `workspace_members`.
- [x] Create `sites` with `workspace_id`, `name`, `slug`, `status`, `default_locale`, `published_version_id`, `generation_prompt`, `generation_summary`, and `settings`.
- [x] Create `site_domains` with hostname, type, status, and verification fields, even if custom domain verification is deferred.
- [x] Create `themes` with site ownership, version, and constrained token JSON.
- [x] Create `pages` with site ownership, title, slug, sort order, status, SEO JSON, and settings JSON.
- [x] Enforce the maximum of 10 active pages per site at the application layer first, and add DB-level protection if practical.
  The canonical `siteconfig` validator rejects drafts with more than 10 pages before persistence starts, and migration `000003_page_limit.sql` adds a Postgres constraint trigger that rejects more than 10 non-archived page rows per site.
- [x] Create `block_instances` with `page_id`, duplicated `site_id`, type, version, sort order, props JSON, settings JSON, and `is_hidden`.
- [x] Create `site_versions` with immutable snapshot JSON, version number, creator, created timestamp, and publish note.
- [x] Create `generation_jobs` for prompt tracking, status, output plan, errors, and input context.
- [x] Create `assets`, `form_submissions`, `page_view_daily`, and `audit_events` tables, but keep most UI around them for later phases.
- [x] Implement draft assembly from normalized rows into a canonical `SiteDraft`.
- [x] Implement draft persistence from canonical input into normalized rows.
  Draft persistence now lives in `internal/sites` as `PostgresWriter.SaveDraft`: it validates canonical `siteconfig.SiteDraft` input, normalizes site/theme/page/block rows, upserts site/theme/pages, replaces block instances transactionally, and preserves block visibility through `block_instances.is_hidden`. Public create/generate APIs still need to call this writer in later phases.
- [x] Implement read APIs for listing sites and loading one complete draft.

## Phase 2: Code-Owned Block Registry

- [x] Define the shared `BlockDefinition` contract with type, version, display name, category, prop schema, default props, editor schema, renderer mapping, and migration hook.
  The Go-owned `siteconfig` registry now carries type, version, display name, category, default props, and recursive editor field metadata for the prototype blocks, while React consumes that contract for block editing and rendering. Renderer mapping remains owned by the React renderer switch until a more formal generated contract is added.
- [x] Decide the registry ownership boundary: Go owns validation schemas and persistence rules; React owns renderer components and editor field components; both must be generated from or checked against the same contract.
  Decision 0003 formalizes the boundary: `internal/siteconfig` remains the canonical source for block type/version metadata, default props, and validation, while React consumes that API-shaped contract for renderer/editor support instead of redefining schemas locally.
- [x] Implement the registry in code rather than the database.
- [x] Add registry lookup by `type` and `version`.
- [x] Add validation for block existence, version existence, props shape, links, asset references, and hidden/settings fields.
- [x] Add the first prototype blocks: `hero`, `text_section`, `image_text`, `features_grid`, and `cta_band`.
- [x] Implement React renderer components for those prototype blocks.
  React now renders the prototype `hero`, `text_section`, `image_text`, `features_grid`, and `cta_band` blocks in a shared `SiteDraftRenderer` used by the authenticated preview route.
- [x] Implement React editor field metadata for those prototype blocks.
- [x] Build React renderer and editor surfaces with Tailwind utilities so preview, builder, and publish output stay visually consistent.
  The shared builder shell, block editor, preview renderer, public snapshot page, and route-level empty/error states now use source-owned Tailwind utility composition from `apps/web/src/lib/styles.ts` plus shadcn-wrapped controls so the authenticated builder and render surfaces stay aligned.
- [x] Add contract tests or generated fixtures proving Go validation accepts exactly what the React renderer/editor expects.
  Shared fixture `internal/siteconfig/testdata/block_registry_contract.json` is generated from the Go registry contract, asserted in Go against the live definitions, and imported by new React tests to prove every Go-defined prototype block renders in `SiteDraftRenderer` and opens in `BlockEditor` without falling back to unsupported UI. Verified on May 10, 2026 with `make test`, `npm run web:test`, `npm run web:lint`, and `npm run web:build`.
- [x] Add registry tests that reject unknown blocks, unknown versions, invalid props, unsafe links, and unsupported settings.
  `internal/siteconfig` now includes registry-focused tests covering invalid block definitions, duplicate registrations, unknown versions, unsafe CTA URLs, unsupported block props, and invalid anchor settings alongside the existing draft/publish validation suite.
- [ ] Defer remaining MVP blocks until the prototype loop works.

## Phase 3: Theme, Navigation, And Snapshot Contracts

- [x] Define `theme.v1` token schema for colors, typography, layout, and shape.
- [x] Define brand baseline tokens from `BRANDING.md`, including `logo.png`-inspired yarn/spindle colors and required light/dark mode token sets.
- [x] Add a small set of safe theme presets such as minimal luxury, playful startup, and calm nordic.
- [x] Implement theme token validation and fallback generation.
- [x] Implement CSS variable output from theme tokens in React rendering.
- [x] Define canonical `SiteDraft`, `PageDraft`, `BlockInstance`, `ThemeConfig`, `NavigationConfig`, and `SeoConfig` types.
  Canonical Go-owned config types now live in `internal/siteconfig/types.go` and are used directly by draft assembly, persistence, generation, theming, and publish validation.
- [x] Define published `site-config.v1` snapshot schema.
  Published snapshots now use the explicit `siteconfig.PublishedSnapshot` contract with `SchemaVersion` set to `site-config.v1` and enforced by `ValidatePublishedSnapshot`.
- [x] Generate or manually maintain frontend TypeScript types from the Go/API schema until automated type generation is added.
  The frontend currently maintains the shared draft, snapshot, version, theme, and block contract types manually in `apps/web/src/lib/api.ts`.
- [ ] Implement navigation storage as explicit data derived from pages by default.
- [ ] Ensure internal navigation prefers stable `pageId` references and renderer resolution to slugs.
- [x] Validate external navigation URLs.
- [x] Add publish preflight validation for homepage `/`, at least one page, max 10 pages, unique slugs, valid blocks, valid navigation, valid theme tokens, and SEO fallbacks.
  `siteconfig.ValidateDraft` and `siteconfig.ValidatePublishedSnapshot` now enforce page/homepage/slug/navigation/theme/block rules, while `buildPublishedSnapshot` fills required SEO fallbacks before publish validation runs.

## Phase 4: React Builder And Manual Prototype Creation

- [x] Implement Go site create, update, delete, and list APIs.
  `internal/sites` now exposes authenticated `POST /api/sites`, `GET /api/sites`, `GET /api/sites/:siteId`, `PATCH /api/sites/:siteId`, and `DELETE /api/sites/:siteId` handlers, backed by deterministic draft creation, slug conflict checks, and tests for handler + mutation flows.
- [x] Implement Go page create, update, delete, and reorder APIs.
- [x] Implement Go block create, update, delete, duplicate, hide/show, and reorder APIs.
  `internal/sites` now exposes authenticated page create/update/delete/reorder routes plus block create/update/delete/duplicate/reorder routes, all backed by canonical draft validation and regression tests across handler + mutator flows.
- [x] Implement Go theme read and update APIs.
- [x] Implement a simple authenticated builder shell with site list and site detail.
  The `/app` workspace route now lists saved sites, creates drafts, and links into a functional site detail screen with metadata, page outline, rename/reslug, and delete actions.
- [x] Use shadcn/ui primitives for builder controls, forms, dialogs, menus, tabs, loading states, and empty/error states before creating bespoke app components.
  The current builder pass standardizes on source-owned shadcn-style `Button`, `Input`, `Textarea`, `Select`, and `Checkbox` primitives for site, page, block, theme, auth, and prompt-entry flows, with shared loading, empty, and error surfaces layered on top.
- [x] Build the React prompt entry page, even if it initially creates a deterministic default site before AI is wired in.
  The builder home now accepts a site name plus brief, calls `POST /api/sites/generate` when a brief is provided, and still falls back to the deterministic starter draft through `POST /api/sites` when the brief is left empty.
- [x] Add a page list and block list.
- [x] Add a simple field editor generated from the block editor schema.
- [x] Add a React preview route that renders the current draft through the same block renderer used by publish.
  `/app/sites/:siteId/preview` now fetches the stored draft and renders it through the shared React block renderer.
- [x] Add a frontend API client layer for typed calls to the Go backend.
  Typed draft/auth/theme/publish client helpers now live in `apps/web/src/lib/api.ts`, including shared auth-refresh and API error handling.
- [x] Add loading, empty, and error states for the site list, builder, save actions, preview, and publish action.
  Route-level loading, save-state, empty-state, and error-state handling now covers login, site list, builder detail, draft preview, published snapshot loading, and publish history interactions.
- [x] Save every block edit through backend validation rather than trusting client state.
- [x] Keep the editor state adapter thin and do not store raw editor/Puck state as canonical data.
  The current builder continues to edit canonical draft/page/block/theme data directly through typed Go API calls and does not persist any raw Puck-style client state as the source of truth.
- [x] Confirm the prototype works without AI generation by creating a site from deterministic defaults.
  Verified on May 6, 2026 by logging in locally, creating a draft for `Moss & Thread Atelier`, editing its site metadata, and loading the authenticated preview route in Playwright.
- [x] Confirm the block editing loop works end to end for the prototype builder.
  Verified on May 6, 2026 by logging in locally, creating `Ribbon & Reed Workshop`, editing the hero headline and CTA label in the block editor, saving through the Go API, and confirming the updated content rendered on `/app/sites/:siteId/preview` in Playwright.
- [x] Confirm page management and advanced block operations work end to end for the prototype builder.
  Verified on May 7, 2026 by logging in locally, creating `Planner Spindle Test`, adding a `Contact` page, editing its slug/SEO/navigation state, reordering pages, adding a `text_section` block, duplicating/reordering/deleting that block, and then deleting the page in Playwright while the corresponding Go API page/block routes returned successful responses with no browser console errors.
- [x] Confirm theme editing works end to end for preview and published rendering.
  Verified on May 7, 2026 by logging in locally, creating `Theme Verification Studio`, switching the builder theme to `Playful Ribbon` + `Studio Sans` + `Airy` + `Pillowy` in Playwright, confirming the saved theme swatches and success state in the builder, then checking computed styles on both `/app/sites/:siteId/preview` and `/public/theme-verification-studio` to confirm the updated palette, font stack, and radius-driven layout rendered without browser console errors.

## Phase 5: Prompt-To-Draft Generation

- [x] Implement `POST /api/sites/generate` in the Go backend.
- [x] Create a `generation_jobs` row for every prompt.
- [x] Define structured model output for `siteName`, `siteSlug`, `siteGoal`, `theme`, `pages`, `navigation`, `assetsNeeded`, and `assumptions`.
- [x] Use the canonical draft schema as the generation output target.
- [x] Limit generation to the prototype block set until the core loop is stable.
- [x] Enforce generation guardrails: max 10 pages, known blocks only, supported versions only, safe URLs only, valid theme tokens only, no scripts, no unsupported embed code, no unsanitized HTML.
- [x] Add deterministic repair for safe issues such as missing optional defaults, excessive page count, duplicate slugs, and missing SEO fallbacks.
  Generation now passes every plan through a Go-side repair step before persistence: it sanitizes plain-text fields, drops unsupported blocks and unsafe CTA URLs, falls back to a valid theme preset, caps page count at 10, repairs duplicate or invalid slugs, restores homepage and SEO defaults, and inserts safe fallback blocks when a page would otherwise become invalid. Verified on May 10, 2026 with `go test ./internal/generation ./internal/siteconfig` and `make test`.
- [ ] Add model repair or retry only after backend validation fails.
- [x] Persist valid generated output as normalized draft rows.
- [x] Store generation prompt, assumptions, provenance metadata, validation outcome, and summary.
- [x] Return the created site draft and send the user to the builder preview.
  Verified on May 6, 2026 by generating a photography-site draft through the authenticated builder in Playwright, confirming a completed `generation_jobs` record plus stored site metadata in the Go-backed flow, and loading the generated draft preview without current-page console errors.

## Phase 6: Publish, Public Serving, And Rollback

- [x] Implement snapshot assembly from the current canonical draft in Go.
- [x] Validate the full snapshot before publish.
- [x] Create a new immutable `site_versions` row with an incremented version number.
- [x] Decide the first artifact generation path: React SSR/render command invoked by Go, React public route rendering from snapshot, or Go serving prebuilt React-generated artifacts.
  Decision recorded on May 10, 2026: the prototype now uses the React public route rendering path backed by immutable published snapshots. The Go API resolves `site_domains` plus `published_version_id`, and the TanStack Start public-serving layer renders the selected snapshot for either the local `/public/:siteSlug` fallback or the hosted-domain request path.
- [ ] Generate page HTML artifacts from the snapshot using the maintained React block renderer.
- [ ] Generate `sitemap.xml`, `robots.txt`, canonical metadata, and basic social metadata.
- [ ] Store artifacts in object storage or a local artifact adapter for the first prototype.
- [x] Update `sites.published_version_id` only after snapshot and artifact creation succeeds.
- [x] Create or update the default subdomain record `{site-slug}.platform.com` or local equivalent.
- [x] Implement public hostname and path resolution through `site_domains` in the Go backend or public-serving layer.
  Local-equivalent published path resolution was first verified on May 7, 2026: after publishing `Public Path Verification Studio` in Playwright, both `/public/public-path-verification-studio` and `/public/public-path-verification-studio/contact` loaded the expected snapshot-backed pages and the in-site navigation moved between them without browser console errors.
  Hostname-based lookup through `site_domains` was verified on May 10, 2026 by publishing the seeded `Nordic Studio` site, loading `http://nordic-studio.localhost:3000/` and `http://nordic-studio.localhost:3000/contact` in Playwright, confirming the hosted route resolved from the assigned domain, checking that the public shell rendered without the builder chrome, and confirming there were no current-page console errors while in-site navigation stayed on root-relative hosted paths.
- [x] Serve public pages from published artifacts or published snapshots, never from draft rows.
- [ ] Add cache keys for domain lookup, published snapshots, and page artifacts.
- [ ] Invalidate public cache on publish and rollback.
- [x] Implement version list and rollback by setting `sites.published_version_id` to an existing version.
- [x] Write audit events for publish and rollback.
  Verified on May 7, 2026 by logging in locally, creating `Rollback Verification Loom`, publishing version 1 from the builder in Playwright, editing and publishing version 2, rolling the live site back to version 1 from the new publish history UI, confirming the builder marked `v1` current again, and reloading `/public/rollback-verification-loom` to confirm it served the original published snapshot while the draft still retained the newer editable headline.

## Phase 7: Builder Fleshing After The Prototype Works

- [ ] Add the remaining MVP blocks: `gallery`, `testimonials`, `pricing_packages`, `contact_form`, `faq`, `team_profile_cards`, and `footer`.
- [ ] Add optional early blocks only if user testing shows demand: logo cloud, map/location, stats/KPIs, article teaser, or allowlisted embeds.
- [ ] Add richer page management: rename, slug edit, SEO edit, include/exclude from navigation, navigation reorder, and deletion safeguards.
- [ ] Add theme controls for palette, font preset, button style, radius, section spacing, and image style.
- [ ] Add Puck or another compact CMS-style editing layer as an MVP requirement for faster visual authoring inside the React builder.
- [ ] Build adapters from canonical draft data to editor state and back.
- [ ] Add site-level re-prompt.
- [ ] Add page-level re-prompt.
- [ ] Capture an undoable draft revision before destructive site-level or page-level re-prompt replacement.
- [ ] Make re-prompt behavior explicit in the UI as a replacement action, not a vague merge.
- [ ] Defer block-level prompting until site-level and page-level prompting are stable.

## Phase 8: Assets And Starter Images

- [ ] Use SeaweedFS S3 API as the default local object storage target.
- [ ] Keep the storage interface S3-compatible so production can use AWS S3, Cloudflare R2, MinIO, or another compatible backend without changing asset code.
- [ ] Implement signed upload URL creation.
- [ ] Implement upload completion and asset metadata persistence.
- [ ] Add asset picker support in block editors.
- [ ] Store asset ownership, storage key, public or signed URL behavior, alt text, dimensions, file type, file size, and upload metadata.
- [ ] Validate that referenced assets belong to the same workspace or site before saving block props.
- [ ] Resolve `assetId` references to optimized URLs during preview and publish.
- [ ] Add safe placeholders, gradients, or stock defaults for missing imagery.
- [ ] Optionally add backend-owned Unsplash search through a constrained `search_unsplash_images(query, orientation, count)` tool.
- [ ] Store source and attribution metadata for starter images.
- [ ] Keep AI image generation out of the MVP unless it becomes a hard product requirement.

## Phase 9: Forms, Submissions, And Notifications

- [ ] Implement the `contact_form` block with allowlisted MVP fields: name, email, optional phone, message, and optional select.
- [ ] Validate public form payloads against the stored block definition.
- [ ] Rate-limit public form submissions.
- [ ] Store submissions in `form_submissions`.
- [ ] Add spam scoring or basic spam filtering.
- [ ] Add submission list and status update APIs.
- [ ] Decide whether MVP sends email notifications, stores submissions only, or does both.
- [ ] Add email forwarding if selected for MVP.
- [ ] Ensure public form responses do not leak site internals.

## Phase 10: Security, Caching, And Observability

- [ ] Verify JWT auth middleware protects every authenticated API route.
- [ ] Keep authorization separate from authentication: a valid JWT proves identity, while workspace membership checks prove access.
- [ ] Store refresh tokens or session identifiers securely if refresh-token rotation is implemented.
- [ ] Add CSRF protection because browser auth uses cookies.
- [ ] Ensure every authenticated write route verifies authentication, workspace membership, and resource ownership.
- [ ] Sanitize rich text input.
- [ ] Validate all URLs and reject unsafe protocols.
- [ ] Restrict embeds to allowlisted providers if embeds are added.
- [ ] Add preview tokens that are random, site-scoped, expiring, and revocable.
- [ ] Add request logging for generation, publish, form submission, and public rendering failures.
- [ ] Add public artifact cache invalidation on publish, rollback, and domain changes.
- [ ] Avoid public caching for authenticated editor responses.
- [ ] Add non-blocking server-side page view counting.
- [ ] Store aggregated daily page views in `page_view_daily`.
- [ ] Add basic bot filtering or rate limiting where practical.
- [ ] Add audit events for site create, generation, re-prompt, publish, rollback, asset upload, and destructive edits.

## Phase 11: Deployment And Hosting

- [ ] Deploy one React builder app for authenticated editing.
- [ ] Deploy one Go backend API service.
- [ ] Deploy one lightweight public site service, either Go serving artifacts or React rendering published snapshots.
- [ ] Provision one Postgres database.
- [ ] Provision object storage buckets for uploads and published artifacts, using SeaweedFS S3 locally and an S3-compatible production provider later.
- [ ] Configure `app.platform.com` or the chosen builder domain.
- [ ] Configure wildcard hosted subdomains for `{site-slug}.platform.com`.
- [ ] Ensure public serving can resolve hostname, find `site_domains`, load `published_version_id`, load the artifact for the path, and return the response.
- [ ] Add environment configuration for model API keys, object storage, app URL, public site base domain, email provider if needed, and Stripe billing keys when billing is enabled.
- [ ] Keep Redis, workers, custom domains, and advanced CDN behavior optional until the simpler deployment is proven.

## Phase 12: MVP Completion

- [ ] Ship the Puck/CMS-style visual editing layer in the MVP builder while keeping canonical site data in the maintained draft schema.
- [ ] Expand generation to all required MVP blocks.
- [ ] Support up to 10 pages per site in generation, editing, validation, and publishing.
- [ ] Add asset upload and image library UI.
- [ ] Add basic SEO editing and publish-generated SEO artifacts.
- [ ] Add contact form submissions and the chosen notification behavior.
- [x] Add version list and user-visible rollback.
- [ ] Add lightweight analytics views for total views, page views, and daily views.
- [ ] Add enough empty, loading, and error states to make the builder usable.
- [ ] Add end-to-end tests for create, generate, edit, preview, publish, public render, submit form, and rollback.
- [ ] Add regression tests for invalid generation output, invalid block props, broken navigation, missing homepage, duplicate slugs, and exceeding 10 pages.
- [ ] Run a production-like smoke test using a real prompt and a real published subdomain.

## Phase 13: Stripe Billing

- [ ] Decide the first billing posture: free beta, paid subscriptions, metered usage, or manual invoicing.
- [ ] Define Stripe products, prices, and local plan entitlements for site count, published sites, custom domains, generation allowance, asset storage, and future seats.
- [ ] Create a `billing` backend module using Stripe for workspace subscriptions and payment collection.
- [ ] Add `billing_customers`, `billing_subscriptions`, `billing_entitlements`, and `billing_events` persistence.
- [ ] Create Checkout session APIs for starting or changing a workspace subscription.
- [ ] Create Customer Portal session APIs for payment method, invoice, and cancellation management.
- [ ] Implement Stripe webhook signature verification and idempotent event processing.
- [ ] Map Stripe customer, subscription, product, price, and invoice/payment states to local workspace billing state.
- [ ] Enforce billing entitlements server-side before generation, publishing, custom domain setup, asset upload, and future team expansion.
- [ ] Add builder billing settings UI for current plan, usage, checkout, and customer portal access.
- [ ] Add blocked-action states for entitlement limits and unpaid or canceled subscriptions.
- [ ] Add Stripe test-mode or Stripe CLI smoke tests plus unit tests for webhook processing and entitlement enforcement.

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
- [ ] Do not build custom domain verification before hosted subdomains work reliably.
- [ ] Do not add raw analytics event storage unless aggregated daily counts are insufficient.
