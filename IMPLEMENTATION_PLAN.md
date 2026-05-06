# Implementation Plan

This plan is sequenced for the shortest path to a working prototype first. The prototype should prove the core loop: create a structured site draft from a prompt, edit it, preview it, publish it, and serve it from a stable public URL. Everything else should be added only after that loop works end to end.

## Prototype Success Criteria

- [ ] A signed-in user can create or use a default workspace.
- [ ] The backend is a Go modular monolith exposing the product API and owning Postgres persistence.
- [ ] The frontend is a React application built with TanStack Start, Tailwind CSS, and shadcn/ui unless a later decision deliberately changes that stack.
- [ ] A user can enter a prompt and get a valid structured site draft.
- [ ] The generated draft uses only known block types, known block versions, valid block props, valid theme tokens, and no arbitrary code.
- [ ] The draft can be previewed through the maintained React renderer.
- [ ] The user can edit basic block fields and save validated changes.
- [ ] The user can publish the draft into an immutable snapshot.
- [ ] The published site is reachable at a platform subdomain or local equivalent.
- [ ] Published output is served from the published snapshot or generated artifacts, not from mutable draft tables.

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
- [ ] Add JWT-based authentication across the Go API and React frontend.
- [ ] Implement Go JWT middleware that validates token signature, expiry, issuer, audience, subject, and required claims.
- [ ] Use server-set secure HTTP-only cookies for browser auth tokens; do not store auth tokens in browser local storage.
- [ ] Keep token issuance, refresh, logout, and revocation server-side in the Go backend.
- [ ] Define access token lifetime, refresh token lifetime, refresh rotation, logout, and token revocation behavior.
- [ ] Add React route guards and API client behavior for unauthenticated, expired-token, and forbidden responses.
- [ ] Create automatic default workspace creation for each user.
- [ ] Add shared authorization helpers that verify workspace membership and resource ownership.
- [ ] Add a shared ID, slug, timestamp, and audit utility layer.
- [ ] Establish runtime validation with schema tooling for site drafts, published snapshots, block props, theme tokens, navigation, URLs, and form definitions.
- [ ] Add backend test setup for schema validation, registry validation, publish validation, persistence, and API authorization.
- [ ] Add frontend test setup for core builder flows and renderer smoke tests.

## Phase 1: Data Model And Draft Persistence

- [x] Create `users`, `workspaces`, and `workspace_members`.
- [x] Create `sites` with `workspace_id`, `name`, `slug`, `status`, `default_locale`, `published_version_id`, `generation_prompt`, `generation_summary`, and `settings`.
- [x] Create `site_domains` with hostname, type, status, and verification fields, even if custom domain verification is deferred.
- [x] Create `themes` with site ownership, version, and constrained token JSON.
- [x] Create `pages` with site ownership, title, slug, sort order, status, SEO JSON, and settings JSON.
- [ ] Enforce the maximum of 10 active pages per site at the application layer first, and add DB-level protection if practical.
- [x] Create `block_instances` with `page_id`, duplicated `site_id`, type, version, sort order, props JSON, settings JSON, and `is_hidden`.
- [x] Create `site_versions` with immutable snapshot JSON, version number, creator, created timestamp, and publish note.
- [x] Create `generation_jobs` for prompt tracking, status, output plan, errors, and input context.
- [x] Create `assets`, `form_submissions`, `page_view_daily`, and `audit_events` tables, but keep most UI around them for later phases.
- [ ] Implement draft assembly from normalized rows into a canonical `SiteDraft`.
- [ ] Implement draft persistence from canonical input into normalized rows.
- [ ] Implement read APIs for listing sites and loading one complete draft.

## Phase 2: Code-Owned Block Registry

- [ ] Define the shared `BlockDefinition` contract with type, version, display name, category, prop schema, default props, editor schema, renderer mapping, and migration hook.
- [ ] Decide the registry ownership boundary: Go owns validation schemas and persistence rules; React owns renderer components and editor field components; both must be generated from or checked against the same contract.
- [ ] Implement the registry in code rather than the database.
- [ ] Add registry lookup by `type` and `version`.
- [ ] Add validation for block existence, version existence, props shape, links, asset references, and hidden/settings fields.
- [ ] Add the first prototype blocks: `hero`, `text_section`, `image_text`, `features_grid`, and `cta_band`.
- [ ] Implement React renderer components for those prototype blocks.
- [ ] Implement React editor field metadata for those prototype blocks.
- [ ] Build React renderer and editor surfaces with Tailwind utilities so preview, builder, and publish output stay visually consistent.
- [ ] Add contract tests or generated fixtures proving Go validation accepts exactly what the React renderer/editor expects.
- [ ] Add registry tests that reject unknown blocks, unknown versions, invalid props, unsafe links, and unsupported settings.
- [ ] Defer remaining MVP blocks until the prototype loop works.

## Phase 3: Theme, Navigation, And Snapshot Contracts

- [ ] Define `theme.v1` token schema for colors, typography, layout, and shape.
- [ ] Add a small set of safe theme presets such as minimal luxury, playful startup, and calm nordic.
- [ ] Implement theme token validation and fallback generation.
- [ ] Implement CSS variable output from theme tokens in React rendering.
- [ ] Define canonical `SiteDraft`, `PageDraft`, `BlockInstance`, `ThemeConfig`, `NavigationConfig`, and `SeoConfig` types.
- [ ] Define published `site-config.v1` snapshot schema.
- [ ] Generate or manually maintain frontend TypeScript types from the Go/API schema until automated type generation is added.
- [ ] Implement navigation storage as explicit data derived from pages by default.
- [ ] Ensure internal navigation prefers stable `pageId` references and renderer resolution to slugs.
- [ ] Validate external navigation URLs.
- [ ] Add publish preflight validation for homepage `/`, at least one page, max 10 pages, unique slugs, valid blocks, valid navigation, valid theme tokens, and SEO fallbacks.

## Phase 4: React Builder And Manual Prototype Creation

- [ ] Implement Go site create, update, delete, and list APIs.
- [ ] Implement Go page create, update, delete, and reorder APIs.
- [ ] Implement Go block create, update, delete, duplicate, hide/show, and reorder APIs.
- [ ] Implement Go theme read and update APIs.
- [ ] Implement a simple authenticated builder shell with site list and site detail.
- [ ] Use shadcn/ui primitives for builder controls, forms, dialogs, menus, tabs, loading states, and empty/error states before creating bespoke app components.
- [ ] Build the React prompt entry page, even if it initially creates a deterministic default site before AI is wired in.
- [ ] Add a page list and block list.
- [ ] Add a simple field editor generated from the block editor schema.
- [ ] Add a React preview route that renders the current draft through the same block renderer used by publish.
- [ ] Add a frontend API client layer for typed calls to the Go backend.
- [ ] Add loading, empty, and error states for the site list, builder, save actions, preview, and publish action.
- [ ] Save every block edit through backend validation rather than trusting client state.
- [ ] Keep the editor state adapter thin and do not store raw editor/Puck state as canonical data.
- [ ] Confirm the prototype works without AI generation by creating a site from deterministic defaults.

## Phase 5: Prompt-To-Draft Generation

- [ ] Implement `POST /api/sites/generate` in the Go backend.
- [ ] Create a `generation_jobs` row for every prompt.
- [ ] Define structured model output for `siteName`, `siteSlug`, `siteGoal`, `theme`, `pages`, `navigation`, `assetsNeeded`, and `assumptions`.
- [ ] Use the canonical draft schema as the generation output target.
- [ ] Limit generation to the prototype block set until the core loop is stable.
- [ ] Enforce generation guardrails: max 10 pages, known blocks only, supported versions only, safe URLs only, valid theme tokens only, no scripts, no unsupported embed code, no unsanitized HTML.
- [ ] Add deterministic repair for safe issues such as missing optional defaults, excessive page count, duplicate slugs, and missing SEO fallbacks.
- [ ] Add model repair or retry only after backend validation fails.
- [ ] Persist valid generated output as normalized draft rows.
- [ ] Store generation prompt, assumptions, provenance metadata, validation outcome, and summary.
- [ ] Return the created site draft and send the user to the builder preview.

## Phase 6: Publish, Public Serving, And Rollback

- [ ] Implement snapshot assembly from the current canonical draft in Go.
- [ ] Validate the full snapshot before publish.
- [ ] Create a new immutable `site_versions` row with an incremented version number.
- [ ] Decide the first artifact generation path: React SSR/render command invoked by Go, React public route rendering from snapshot, or Go serving prebuilt React-generated artifacts.
- [ ] Generate page HTML artifacts from the snapshot using the maintained React block renderer.
- [ ] Generate `sitemap.xml`, `robots.txt`, canonical metadata, and basic social metadata.
- [ ] Store artifacts in object storage or a local artifact adapter for the first prototype.
- [ ] Update `sites.published_version_id` only after snapshot and artifact creation succeeds.
- [ ] Create or update the default subdomain record `{site-slug}.platform.com` or local equivalent.
- [ ] Implement public hostname and path resolution through `site_domains` in the Go backend or public-serving layer.
- [ ] Serve public pages from published artifacts or published snapshots, never from draft rows.
- [ ] Add cache keys for domain lookup, published snapshots, and page artifacts.
- [ ] Invalidate public cache on publish and rollback.
- [ ] Implement version list and rollback by setting `sites.published_version_id` to an existing version.
- [ ] Write audit events for publish and rollback.

## Phase 7: Builder Fleshing After The Prototype Works

- [ ] Add the remaining MVP blocks: `gallery`, `testimonials`, `pricing_packages`, `contact_form`, `faq`, `team_profile_cards`, and `footer`.
- [ ] Add optional early blocks only if user testing shows demand: logo cloud, map/location, stats/KPIs, article teaser, or allowlisted embeds.
- [ ] Add richer page management: rename, slug edit, SEO edit, include/exclude from navigation, navigation reorder, and deletion safeguards.
- [ ] Add theme controls for palette, font preset, button style, radius, section spacing, and image style.
- [ ] Add Puck or another compact CMS-style editing layer if it improves authoring speed inside the React builder.
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
- [ ] Add environment configuration for model API keys, object storage, app URL, public site base domain, and email provider if needed.
- [ ] Keep Redis, workers, custom domains, and advanced CDN behavior optional until the simpler deployment is proven.

## Phase 12: MVP Completion

- [ ] Expand generation to all required MVP blocks.
- [ ] Support up to 10 pages per site in generation, editing, validation, and publishing.
- [ ] Add asset upload and image library UI.
- [ ] Add basic SEO editing and publish-generated SEO artifacts.
- [ ] Add contact form submissions and the chosen notification behavior.
- [ ] Add version list and user-visible rollback.
- [ ] Add lightweight analytics views for total views, page views, and daily views.
- [ ] Add enough empty, loading, and error states to make the builder usable.
- [ ] Add end-to-end tests for create, generate, edit, preview, publish, public render, submit form, and rollback.
- [ ] Add regression tests for invalid generation output, invalid block props, broken navigation, missing homepage, duplicate slugs, and exceeding 10 pages.
- [ ] Run a production-like smoke test using a real prompt and a real published subdomain.

## Explicit Deferrals

- [ ] Do not build arbitrary user code injection.
- [ ] Do not build custom CSS or custom JavaScript editing.
- [ ] Do not build full drag-and-drop layout freedom.
- [ ] Do not build a Webflow-style design editor.
- [ ] Do not build marketplace or third-party blocks.
- [ ] Do not build e-commerce checkout.
- [ ] Do not build complex CMS collections.
- [ ] Do not build multi-language sites.
- [ ] Do not build advanced teams, roles, billing, or client collaboration until the single-workspace MVP works.
- [ ] Do not build per-customer frontend deployments.
- [ ] Do not build custom domain verification before hosted subdomains work reliably.
- [ ] Do not add raw analytics event storage unless aggregated daily counts are insufficient.
