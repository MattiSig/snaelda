# Security, Validation, and Caching

## Authorization

Every write operation must verify:

- the user is authenticated
- the user belongs to the workspace
- the workspace owns the site, page, block, or asset being modified

Do not rely on client-side checks.

## Row-Level Security

Database row-level security can be added later as defense in depth, even if the backend already enforces authorization.

## Content Safety

For MVP:

- do not allow arbitrary scripts
- sanitize rich text
- restrict embeds to allowlisted providers
- validate URLs
- validate form fields
- rate-limit public form submissions
- store generated content provenance metadata

## Preview Tokens

Preview tokens should be:

- random
- scoped to a site
- optionally expiring
- revocable

## Validation Rules

### Before Saving Block Props

- validate block type exists
- validate version exists
- validate props against schema
- validate links
- validate asset references belong to the same workspace/site

### Before Publishing

- site has at least one page
- site has a homepage with slug `/`
- site has no more than 10 pages
- every page slug is unique
- every block is valid
- every internal navigation link resolves
- theme tokens are valid
- required SEO fields have fallbacks
- sitemap inputs are valid
- canonical paths are valid
- contact form definitions use safe supported fields

## Light Analytics Rules

For MVP analytics:

- prefer server-side page view counting
- avoid heavy client tracking scripts
- rate-limit or filter obvious bot noise where practical
- store aggregated counts when possible
- keep analytics non-blocking for page delivery

## Caching Strategy

The MVP does not need a sophisticated cache layer immediately, but public rendering should be designed for caching.

Recommended:

- cache published snapshots by `site_id:version_id`
- cache published page artifacts by site/version/path
- cache domain-to-site lookup by hostname
- invalidate cache on publish or domain changes
- never publicly cache authenticated editor responses
- cache draft preview less aggressively

If the public site is pre-rendered on publish, caching becomes simpler because requests can return already-generated page output.
