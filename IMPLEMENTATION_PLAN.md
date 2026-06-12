# Implementation Plan

Refreshed 2026-06-09 from a spec-to-source audit of `specs/*`, `internal/*`, `apps/*`, and `cmd/*`. This is an execution queue, not a shipped-work changelog. Every open item below was confirmed against the current source. Items are sorted by priority; correctness, security, data integrity, and paid conversion come before feature expansion.

## P0 — Launch Blockers

- [x] Prevent draft mutations from damaging the active published version.
  - Draft persistence now archives removed pages out of the current draft instead of deleting rows still referenced by published form submissions and analytics.
  - Current-draft reads and site page counts ignore archived rows, while the database preserves published-page identity so later republish and rollback flows can still resolve historical submissions and analytics correctly.
  - Added unit coverage plus a real-Postgres integration test for live forms, analytics, later republish, and rollback.

- [x] Add concurrency control to canonical draft writes.
  - Draft persistence now uses a canonical `sites.draft_revision` compare-and-swap precondition so overlapping save attempts fail with `draft_conflict` instead of silently overwriting newer changes.
  - The builder now serializes inline saves per block and reloads canonical state on conflict; collections surfaces also refresh cleanly when a stale write is rejected.

- [x] Honor page publication status.
  - Exclude `status=draft` pages from published snapshots, navigation, sitemap, artifacts, collection routes, forms, and analytics while keeping them editable and previewable.

- [x] Make magic-link redemption atomically single-use.
  - Lock and consume the token and create the authenticated session in one transaction; require exactly one consumed row.
  - Add replay, concurrent redemption, expiry, purpose, session-creation failure, and rollback tests.

- [x] Centralize prompt reservation and accounting for every model-backed action.
  - Added a shared prompt-action manager that atomically admits generation jobs against trial and paid allowances before model work begins, then settles trial usage only on successful completion.
  - Wired site generation, site/page reprompt, block suggest, image apply, theme regeneration, collection drafting, and entry drafting through shared prompt reservations and generation-job persistence.
  - Extended model-backed route gating/rate limits and added collection AI audit events plus atomic batch entry persistence so quota errors and job state now stay consistent across the current AI surface.

- [x] Fix form-submission read authorization.
  - Listing submissions must require an authenticated workspace member, matching the update route and the L2-or-paid privacy contract; cookie-only trial sessions must not read inquiries.

- [x] Validate and serve every asset-reference location.
  - Include brand logos and collection-entry assets in workspace/site ownership validation and published asset allowlisting.
  - Add regressions for public brand logos, collection-bound imagery, foreign-workspace assets, and workspace-level assets.

- [ ] Establish one canonical billing plan catalog.
  - Align displayed prices, Stripe setup prices, site limits, prompt allowances, storage allowances, and entitlement snapshots.
  - Derive the purchased plan from trusted Stripe price IDs and propagate plan metadata to subscriptions so Pro Checkout cannot become Basic.
  - Add collection/entry URL entitlements before programmatic content can exceed plan limits.

- [x] Fail production startup when launch-critical services are incomplete.
  - Require HTTPS app/public/billing URLs, non-local public domains, Stripe secret + webhook + configured plans, and production email transport/key.
  - Treat explicitly configured S3 artifact storage failures as fatal instead of silently falling back to local disk.

- [x] Complete the Once-over delivery workflow.
  - Added an operator-authorized pending queue route in the builder plus backend queue and delivery endpoints.
  - Delivery now persists video URL, next steps, and `delivered_at`; flips the workspace status in one transaction; sends `once_over_delivered` with an idempotency key; and records an audit event.

## P1 — Core Product And AI Experience

- [ ] Make site-wide reprompting revise existing content.
  - Matching page slugs currently preserve all blocks, so a successful site reprompt can leave copy unchanged.
  - Use the current draft as context while still applying the requested direction to affected pages.

- [ ] Wire page reprompts through the existing block-aware change-set pipeline.
  - Preserve unaffected blocks and IDs; apply keep/edit/remove/insert operations; retain targeted undo/history and recover cleanly from partial model failure.

- [ ] Make AI history and diffs trustworthy.
  - Show block- and entry-scoped AI operations in the History panel with summaries and individual revert.
  - Replace positional raw-prop diffs with identity-aware rendered before/after comparisons.
  - Add keyboard navigation, Escape behavior, focus trapping/restoration, and before/after/both modes.

- [ ] Put collection and entry AI behind shared generation governance.
  - Persist generated batches atomically instead of entry by entry.
  - Create jobs, revisions, history, audit events, quota usage, and clear failure states.
  - Keep the shipped “Prompt up a collection” and “Prompt entries” UX, with generated entries saved as drafts.

- [ ] Complete collection-template authoring.
  - Expose page type and collection selection when creating/editing pages.
  - Extend block mutation contracts with validated `bindings`.
  - Add typed binding controls and preserve bindings when duplicating blocks.

- [ ] Build the complete entry workspace.
  - Add edit, duplicate, reorder, SEO, status, and typed validation flows.
  - Use real controls for rich text, assets, asset lists, references, dates, locations, enum multi-select, email, phone, and URLs.
  - Add entry-level AI rewrite with revision history and undo.

- [ ] Make collection schema changes safe.
  - Add schema versions plus migration preview/apply APIs.
  - Require explicit mappings for rename/type/remove changes and block destructive direct replacement until entries migrate successfully.

- [ ] Finish collection runtime behavior.
  - Implement `defaultSort`, `exposeDetailUrls`, collection SEO templates, detail-route gating, and plan limits.
  - Decide the canonical collection-detail slug/uniqueness model, then align schema, validation, persistence, routes, and specs.

- [ ] Reconcile the collection block contracts.
  - Add real collection pickers and correctly typed number/boolean controls.
  - Implement field mapping, sorting, filtering, layouts, and visitor-facing filters promised by the registry/spec.
  - Version incompatible changes rather than silently changing old snapshots.

- [ ] Make asset lifecycle operations safe and usable.
  - Wire rename/alt/delete UI to the existing APIs.
  - Show usage locations, refuse referenced deletion, and avoid object/database inconsistency when deletion fails.

- [ ] Fix public URL and SEO correctness.
  - Generate canonical, sitemap, robots, and artifact URLs for the active hostname.
  - Handle custom-domain activation, site-slug changes, and rollback without stale hosts or broken asset URLs.
  - Return real HTTP 404/other statuses for missing public sites and pages instead of soft-404 HTML.

- [ ] Preserve analytics correctness through SSR and draft changes.
  - Forward the visitor user agent so crawler filtering sees the requester rather than the Node runtime.
  - Keep historical published page labels stable and distinguish collection-entry paths from their template page.

- [ ] Define a truthful custom-domain activation state machine.
  - Separate ownership verification, DNS routing readiness, TLS provisioning, active, and failed states.
  - Automate provider registration/certificates or require an explicit operator activation gate.
  - Document wildcard DNS, trusted proxies, forwarded host/protocol handling, and certificate ownership.

- [ ] Strengthen tenant integrity in Postgres.
  - Add composite constraints for block/page/site, entry/collection/site, page/collection/site, asset/workspace/site, and published-version/site relationships.

- [ ] Preserve magic-link anti-enumeration behavior.
  - Return indistinguishable responses for known, unknown, rate-limited, and mailer-failure cases.
  - Rate-limit the submitted normalized address independently of whether an account exists.

- [ ] Make inline authoring keyboard- and touch-accessible.
  - Make block selection keyboard-operable, expose actions on focus/touch, use adequate target sizes, announce saves/errors, and focus-manage menus, drawers, and dialogs.

- [ ] Implement the structured Footer contact contract.
  - Migrate free-form address/hours to structured address and daily hours.
  - Emit correct `PostalAddress` and opening-hours JSON-LD.

- [ ] Add billing and email acceptance coverage.
  - Use signed realistic Stripe payloads for plan mapping, replay, cancellation/downgrade, failed payment, portal, Once-over isolation, and entitlement enforcement.
  - Test every email template/call site, text/HTML parity, Mailpit round-trip, Resend error classification, and retry behavior.

## P2 — Hardening And Product Depth

- [ ] Make builder loading resilient.
  - Load the canonical draft independently from billing, domains, assets, submissions, history, theme, and versions; give optional panels their own skeleton, retry, and error state.

- [ ] Decide draft-preview form semantics.
  - Either add an authenticated draft-submission path or disable submission in draft preview with accurate explanatory copy.

- [ ] Expose “published with draft changes”.
  - Compare the active snapshot with the current draft and make republish status visible in site lists and the builder.

- [ ] Harden caching and artifact storage.
  - Add bounded cache size/TTL, explicit public-render cache headers, cross-replica invalidation or a real CDN purger, and cleanup of partial S3 bundle uploads.

- [ ] Make rate-limit admission atomic.
  - Replace count-then-insert races for auth, forms, and generation; document fail-open/fail-closed policy and test concurrent requests.

- [ ] Complete block versioning.
  - Dispatch renderers by `type@version`, run registered migrations deliberately, and preserve historical snapshot rendering.
  - Reconcile the Stats contract as a new version and add only validated responsive/theme dimensions that materially help generated sites.

- [ ] Finish generation progress lifecycle.
  - Report imagery work in the correct order, add job cancellation, and support persistent background completion/failure feedback.
  - Upstream token streaming is optional after measuring the current decomposed outline/layout/content pipeline; progressive page materialization already exists.

- [ ] Extend initial generation to structured content.
  - Generate collections, entries, collection pages, and bindings when the prompt calls for them.
  - Add asset-aware project creation, location fanout, FAQ-from-services, single-entry rewrite, and empty-page suggestions only after shared governance is in place.

- [ ] Finish trial and email UX.
  - Standardize exhausted-trial error codes/CTA metadata.
  - Remove “claim before publishing” misinformation, add “Already have an account” and resend affordances, and decide whether first-generation education remains a modal or becomes inline guidance.

- [ ] Improve inquiry management.
  - Add search, pagination, status filters, spam signals, delete/export only if validated by user need, and accessible async announcements.

- [ ] Restore contract and browser coverage.
  - Regenerate and review the block-registry golden fixture; `go test ./...` currently fails only on this contract.
  - Strengthen tests for editor value types, page status, collection types/bindings, rapid edits, preview forms, assets, keyboard/touch, mobile, dark mode, domains, and billing transitions.

- [ ] Decompose oversized modules after behavior stabilizes.
  - Split the builder route, public renderer, generation service, and OpenAI adapter along existing domain boundaries without changing contracts.

## P3 — Cleanup

- [ ] Remove obsolete `/api/workspaces`, `/api/pages`, and `/api/blocks` placeholder packages, routes, and tests now superseded by `internal/sites`.

- [ ] Remove unused environment declarations after verifying production deployment requirements, including stale Unsplash and Railway variables.

- [ ] Record billing, domain, and Once-over access-changing events in the application audit log.

## Spec Debt

- [ ] Update Specs 01–03 for the shipped clarification interview, iterative AI workflow, global header/navigation chrome, current block registry, and trial publishing.

- [ ] Finish reconciling Specs 07 and 20 with block/image AI behavior and the remaining governance/history gaps; the decomposed pipeline, partial SSE events, shadow draft, and Pexels drift are now documented.

- [ ] Finish reconciling Specs 10, 17, and 19 with the current restore/trial banner flow; nested routes, session naming, and shipped collection/entry prompt routes are now documented.

- [ ] Reconcile collection-template slug and uniqueness rules across Specs 05, 06, and 12 before changing migrations.

- [ ] Backfill Spec 06 from all deployed migrations and document the chosen stable page identity model for publishing, forms, and analytics.

- [ ] Update Spec 13 with the actual hosting, proxy, wildcard-domain, custom-domain, and TLS operational contract.

- [ ] Resolve Once-over purchase/intake/delivery policy drift across Spec 15, Spec 18, and `docs/once-over-workflow.md`.

## Confirmed Shipped Baseline

- [x] Landing-page continuation for an active workspace and interrupted guest-generation recovery.
- [x] Conditional clarification interview before generation.
- [x] Decomposed outline, per-page layout/content generation, progressive SSE partials, polling recovery, and shadow-draft UI.
- [x] Static page CRUD/SEO/navigation; block add/edit/hide/duplicate/delete/reorder; inline text/image editing.
- [x] Site/page reprompt history foundation, revisions, revert, block AI rewrite, and image replacement.
- [x] Theme editing/regeneration, required dark mode, asset upload/library, submission triage, preview links, publishing, rollback, analytics, custom-domain CRUD, billing foundation, and transactional-email transports.
- [x] Collection CRUD, schema editing foundation, entry CRUD foundation, public collection rendering, and generic AI collection/entry drafting.
