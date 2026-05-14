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

## Priority Backlog

- [ ] Replace local-only hosted URL assumptions with real deployment-grade public domain handling.
  Confirmed gap: hosted URLs still assume local-development behavior like `{slug}.localhost` and `/public/{slug}` helpers rather than a true `{site-slug}.{public-base-domain}` contract.

- [ ] Implement a real `domains` module for hosted-domain management and cache invalidation on domain changes.
  Confirmed gap: `internal/domains` is still placeholder-only even though `site_domains` exists and specs depend on hostname-driven resolution.

- [ ] Make public pages feel like customer sites rather than internal publish previews, and surface the actual live URL in publish/rollback UX.
  Confirmed gap: the builder publish flow still does not prominently use the returned hostname/public URL.

- [ ] Replace the deterministic generation template builder with a provider-backed structured AI generation pipeline.
  Confirmed gap: generation still uses a heuristic template planner rather than a model-backed structured-output flow with explicit prompt-to-plan semantics.

- [ ] Make generation-job persistence and generation metadata writes mandatory rather than best-effort.
  Confirmed gap: generation can currently return success even when job-finalization or metadata persistence fails.

- [ ] Add theme regeneration as a first-class API and builder action.
  Confirmed gap: `POST /api/sites/:siteId/theme/regenerate` is specified but not implemented.

- [ ] Decide and implement the MVP starter-image policy.
  Priority decision: either keep uploaded assets plus placeholders/gradients as the only MVP path, or add backend-owned starter-image search with attribution metadata and plan it as core instead of an unowned optional.

- [ ] Make navigation explicitly editable canonical data instead of a mostly page-derived structure.
  Confirmed gap: ordering and inclusion exist, but internal labels/external items are not treated as full first-class editable navigation records.

- [ ] Restrict public form submission to the active published version only.
  Confirmed gap: the public forms service can currently fall back to draft content when a published block is not found.

- [ ] Restrict public asset delivery to assets referenced by the active published version or a valid preview token.
  Confirmed gap: public asset access is broader than the published-site contract and is keyed by site slug rather than the full hosted-public resolution flow.

- [ ] Add durable spam handling for public forms.
  Confirmed gap: public form rate limiting is process-local and `spam_score` is unused; basic scoring/filtering is still missing.

- [ ] Add audit events for site create, generation, re-prompt, asset upload, and destructive edits.
  Confirmed gap: publish and rollback are audited, but the broader authoring lifecycle is not yet fully covered.

- [ ] Add non-blocking published-page view counting, aggregate writes into `page_view_daily`, and expose the first analytics APIs.
  Confirmed gap: the schema is present, but counting and analytics read endpoints are not implemented.

- [ ] Add the first builder analytics views for total views, views by page, and daily trend windows.
  Scope is now defined in [specs/16-runtime-lifecycles-and-analytics.md](./specs/16-runtime-lifecycles-and-analytics.md).

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
