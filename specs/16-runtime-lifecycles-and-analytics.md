# Runtime Lifecycles, Public Visibility, and Analytics

## Purpose

This spec closes a few MVP gaps that were only implied in earlier docs:

- runtime state and visibility rules for draft vs published content
- public access rules for forms and assets
- domain lifecycle semantics
- lightweight analytics counting and builder reporting

This document does not replace the earlier architecture, publish, security, or hosting specs. It clarifies how those specs behave at runtime.

## Draft And Published Runtime Rules

The builder may read and edit draft data directly.

Public traffic must never depend on mutable draft rows.

That means:

- hosted public pages resolve from the active published version only
- public form submissions resolve against the active published version only
- public asset delivery must only expose assets that are referenced by the active published version
- preview-token traffic may read draft data, but only through the explicit preview-token flow

If a block, page, form, or asset exists only in draft state, it is not publicly reachable until publish succeeds.

## Site Publication Semantics

For MVP, the important site states are:

- `draft_only`: the site has no `published_version_id`
- `published`: the site has a live `published_version_id`
- `published_with_draft_changes`: the site has a live `published_version_id` and the current draft differs from that live version
- `archived`: reserved for later soft-retirement behavior and not required for the first production milestone

The UI does not need to expose these exact labels, but it must expose the underlying meaning:

- whether a site has ever been published
- whether the current draft differs from the live site
- when the last publish happened
- which version is currently live

## Page Runtime Semantics

Pages in the draft model may exist before publish.

For public serving:

- only pages present in the active published version are routable
- deleted draft pages do not disappear from the public site until a new publish succeeds
- newly created draft pages do not appear on the public site until a new publish succeeds
- page slug changes only affect the public site after publish

MVP does not require automatic redirect history for changed slugs.

## Collection Runtime Semantics

Collections and entries follow the same draft-vs-published split as pages. See [Spec 19](./19-collections-and-content-types.md) for the page-type and binding model.

For public serving:

- only collections and entries present in the active published version are routable
- only entries with `status = 'published'` (as captured in the snapshot at publish time) serve public URLs
- a `collection_detail` template URL `/{collection.slug}/{entry.slug}` resolves only when the snapshot contains that collection and a published entry with that slug
- entry slug changes only affect the public site after publish
- removing or unpublishing an entry only takes effect at the next publish

Sitemap generation must include one URL per published entry on every `collection_detail` template.

The published snapshot must include every collection schema and every published entry that any template binds to. The renderer must not need to re-query draft tables to serve a collection URL.

## Domain Lifecycle

The default hosted subdomain is the first public address for every published site.

For MVP:

- the default hosted subdomain must be derived from the site slug and base public domain
- the default hosted subdomain becomes active only after the first successful publish
- republishing may update the default hosted subdomain if the site slug changes
- hostname resolution must always go through `site_domains`

Custom domains are a later-but-planned capability and should use these statuses:

- `pending_verification`
- `active`
- `failed`
- `disabled`

When a custom domain is active:

- the default hosted subdomain still remains valid unless product policy changes later
- caches for hostname lookup and public page output must be invalidated on activation, deactivation, or hostname change

## Public Form Semantics

Public form submission is part of the published site surface, not the draft authoring surface.

Rules:

- a public form submit must target a `contact_form` block that exists in the active published version
- if the block is only present in draft, the public submit endpoint must reject it
- preview-token routes may submit against draft forms only if preview-token form submission is explicitly supported
- success and validation behavior must come from the published block definition for that version

MVP moderation states remain:

- `new`
- `reviewed`
- `resolved`
- `spam`

MVP notification posture remains:

- store submissions in-app
- email forwarding is optional follow-up work, not a requirement for the current prototype milestone

## Public Asset Visibility

Assets belong to a workspace and may be attached to a site, but public delivery needs a stricter rule:

- authenticated builder routes may access any site-owned asset the current user is authorized to use
- public routes may only access assets referenced by the active published version
- preview-token routes may access assets referenced by the token-resolved draft

Published asset URLs should be stable enough to support caching.

MVP does not require a full image-transformation service, but the contract must allow later support for:

- variant generation
- responsive image sets
- stronger cache headers for immutable published assets

## Lightweight Analytics

The MVP analytics model is intentionally small.

Track page views for published public pages only.

Do not count:

- authenticated builder views
- draft preview views behind authenticated builder routes
- obvious health checks or crawler noise when detection is straightforward

Counting rules:

- counting should happen after a successful public page resolution
- counting must be non-blocking for page delivery
- writes should aggregate into `page_view_daily`
- one table row represents one site, one page, and one UTC calendar day

MVP bot filtering may stay heuristic:

- ignore known health-check paths
- ignore requests with clearly automated user agents when that signal is reliable
- keep the logic simple and auditable

## Analytics API And UI Scope

The first analytics surface in the builder should answer only these questions:

- total views for the selected recent window
- views by page for the selected recent window
- daily views trend for the selected recent window

Recommended initial windows:

- last 7 days
- last 30 days
- all time

MVP does not require:

- visitor identity
- sessions
- traffic sources
- client-side event tracking
- funnels

## Cache And Invalidation Implications

Public caches should be invalidated when:

- a site is published (covers all collection-derived URLs in the new snapshot)
- a site is rolled back
- a hosted subdomain changes
- a custom domain is activated, deactivated, or changed
- a published asset mapping changes because a new version becomes live

Authenticated builder responses must remain non-cacheable by shared/public caches.
