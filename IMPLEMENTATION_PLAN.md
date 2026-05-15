# Implementation Plan

This file now tracks confirmed remaining work only. Items are sorted by implementation priority, and stale backlog entries that are already present in the codebase have been removed from the open list.

## Recently Confirmed Complete

- [x] Asset upload and image-library UI exist in the builder, including uploaded-asset selection in block editors.
- [x] Contact-form submission storage and the chosen MVP moderation flow exist; email forwarding remains optional follow-up work, not unfinished core behavior.
- [x] The 10-page limit is already enforced in validation, generation repair, the database, and publish preflight.
- [x] Main builder loading, empty, and error states exist for login, site list, site detail, preview, publish history, assets, and submissions.
- [x] Page-level SEO editing plus publish-time `sitemap.xml`, `robots.txt`, canonical metadata, and basic social metadata exist.
- [x] Refresh-token rotation is server-side and hashed; publish/rollback cache invalidation already exists.
- [x] Added [specs/16-runtime-lifecycles-and-analytics.md](./specs/16-runtime-lifecycles-and-analytics.md) to define public visibility rules, domain/runtime semantics, and MVP analytics scope that were previously only implied.
- [x] Public page reads now resolve from stored published artifacts plus `manifest.json` metadata instead of rebuilding from `site_versions.snapshot`, and public `/public/{slug}` routes no longer carry internal publish framing.
- [x] Publish now validates artifact completeness before promoting a version live, including page HTML, crawl files, theme CSS, and artifact manifest metadata.
- [x] Hosted public URLs now use an explicit deployment contract via `PUBLIC_BASE_URL` and `PUBLIC_BASE_DOMAIN`, so publish-time hostnames, canonical URLs, sitemap entries, and builder live links no longer assume `{slug}.localhost` or `/public/{slug}` as the production shape.
- [x] `internal/domains` is now a real authenticated module with a site-domain read API, exposing hosted-domain state from `site_domains` instead of remaining placeholder-only.
- [x] The builder publish panel now surfaces the actual hosted live URL and opens the live hostname directly instead of treating the internal `/public/{slug}` route as the primary customer-facing address.
- [x] Generation now supports a provider-backed structured-output planner through OpenAI when configured, while keeping deterministic fallback behavior for local and unconfigured environments.
- [x] Generation metadata writes and generation-job completion are now mandatory success conditions rather than best-effort side effects.
- [x] Theme regeneration is now a first-class authenticated API plus builder action via `POST /api/sites/:siteId/theme/regenerate`.
- [x] Public form submission now resolves strictly against the active published version's snapshot; unpublished sites and draft-only blocks are rejected at the public submit endpoint.
- [x] Public asset delivery now requires the asset to be referenced by the active published version, and supports both the `siteSlug` path and the hostname-based hosted-public resolution flow (`GET /api/public/assets/{assetId}`).
- [x] Published public page resolution now records non-blocking views into `page_view_daily`, filtering empty/bot user agents and known health-check paths via `analytics.CountableRequest`.
- [x] Added `GET /api/sites/{siteId}/analytics?window=7d|30d|all` returning total views, per-page views, and a gap-filled daily trend for authorized site members.
- [x] Builder now has a dedicated site analytics view at `/app/sites/{siteId}/analytics` with a window selector, total counter, daily trend chart, and per-page breakdown.
- [x] Navigation is now first-class editable canonical data. The mutator preserves user-edited labels across page renames, `PUT /api/sites/{siteId}/navigation` replaces the whole primary list (internal + external items, validated), and the builder gained a richer navigation editor that lets the user rename items, add external links, reorder, and remove items as a single saved unit.
- [x] Backend-owned starter imagery via Pexels is live. `PEXELS_API_KEY` configures a new `internal/imagery` package with a Pexels client plus a per-run dedupe wrapper. Generation now derives 1–2 short search queries per empty image slot from page/block content, downloads the chosen Pexels photo, re-hosts the binary through `assets.Service.ImportExternal` as a normal `assets` row with `provenance` JSON, and falls back silently to the original blank-slot path on rate-limit, network failure, or empty results. Publish enriches the snapshot with `imageCredits` derived from asset provenance, and `SiteDraftRenderer` shows a "Imagery from Pexels · Photos by …" credit band on public pages whenever any Pexels asset is used. Builder asset picker and library now surface starter provenance ("Starter from Pexels · Photo by …").
- [x] Durable spam handling for public forms. Public form submit now derives a deterministic spam score from a server-side honeypot (`hp_*` payload keys), excessive link counts, embedded `<script>`/`<a>` markup, all-caps long messages, repeated-character runs, mixed Cyrillic/Latin scripts, and a spam-keyword list. Scores ≥ 1.0 are stored as `status='spam'` with the signal list in `form_submissions.spam_signals`. Rate limiting moved from a process-local map to a durable `form_submission_attempts` table keyed on (site_id, block_id, client_ip_hash) so the 5-per-10-minute limit survives restarts and works across replicas. The public renderer now ships an off-screen honeypot field. Schema additions: `form_submissions.client_ip_hash`, `form_submissions.spam_signals`, the new `form_submission_attempts` table, and indexes; see `internal/platform/database/migrations/000006_form_spam_handling.sql`.
- [x] Authoring-lifecycle audit events are now recorded alongside the existing publish/rollback events. The sites mutator records `site.create`, `site.delete`, `page.delete`, and `block.delete`; the generation service records `site.generate`, `site.reprompt`, and `page.reprompt`; the asset service records `asset.upload` (on successful complete) and `asset.delete`. The API server wires a shared `audit.Recorder` into the sites, generation, and assets modules, and audit recording failures are logged best-effort so they never block the underlying authoring operation.

## Priority Backlog

- [ ] Reconcile the implemented API surface with the spec and remove placeholder module drift.
  Confirmed gap: `workspaces`, `pages`, `blocks`, and `billing` are still mounted as placeholder modules even though some page/block behavior is implemented through `sites`; either the API/resource boundaries need to be implemented as separate modules or the specs need to be narrowed to the consolidated shape.

- [ ] Add a real `workspaces` module or explicitly reduce workspace scope in the product/API spec.
  Confirmed gap: users get a default workspace, but there is no non-placeholder workspace API surface.

- [ ] Decide whether `pages` and `blocks` should remain consolidated under `sites` or become first-class modules, then align the code and specs.
  Confirmed gap: route shapes differ materially from the current API spec.

- [ ] Harden rich-text and embed safety on every remaining content surface.
  Confirmed gap: link validation exists, but the remaining sanitization/allowlist posture still needs explicit implementation verification for all editable text and future embed fields.

- [ ] Add production-style end-to-end coverage for create, generate, edit, preview, publish, public render, rollback, assets, preview-token sharing, and contact submissions.
  Confirmed gap: there are strong unit/integration tests and manual Playwright verification notes, but not a consolidated automated end-to-end suite covering the main product loop.

- [ ] Add regression coverage for invalid generation output, invalid publish artifacts, draft-only public form access, draft-only public asset access, broken navigation, missing homepage, duplicate slugs, and page-limit edges.

- [ ] Run a production-like smoke test against a real model-backed generation flow and a real hosted subdomain shape.
  This should happen only after the artifact-serving and hosted-domain work above is complete.

- [ ] Finalize production deployment topology and configuration.
  Remaining work includes the public base domain contract, wildcard subdomains, object storage for published artifacts, environment configuration, and the choice of public-site runtime.

- [ ] Implement Stripe billing once the product loop above is stable.
  Remaining work includes billing tables, config, Checkout, Customer Portal, webhooks, local entitlements, builder billing UI, blocked-action states, and enforcement before generation/publish/custom domains/assets.

## Lower-Priority Product Follow-Ups

- [ ] Add optional early blocks only if user testing shows real demand: logo cloud, map/location, stats/KPIs, article teaser, or allowlisted embeds.
- [ ] Add safe placeholders or gradients for missing imagery if uploaded/starter assets are not present.
- [ ] Add site-level SEO editing and richer metadata workflows if page-level SEO plus publish-generated metadata stop being enough.
- [ ] Add basic asset-management controls for edit/delete in the builder now that upload/list/pick already exist.
- [ ] Preserve hidden-block positions when users hide/show and reorder blocks.
- [ ] Consider block-level prompting only after site-level and page-level prompting are stable in real usage.

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
