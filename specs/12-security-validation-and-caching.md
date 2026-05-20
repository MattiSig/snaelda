# Security, Validation, and Caching

## Authorization

Every write operation must resolve the caller to a session and then verify capability.

### Session Resolution

A request carries one of:

- **Authenticated user session cookie** — resolves to a `users` row, then to one or more workspaces via `workspace_members`.
- **Trial session cookie** — resolves to a `guest_sessions` row, which binds to exactly one workspace. The session may additionally be linked to a `users` row via `claimed_by_user_id` (Spec 17 L2 or post-Checkout).
- **No session** — only the public surface and the authentication/recovery routes accept this.

A request that carries both kinds of cookie is treated as an authenticated user; the trial cookie is ignored except by the explicit restore, attach-email, and claim endpoints.

### Capability Check

For any write the backend must verify:

1. The resolved session's workspace owns the target site, page, block, asset, or domain.
2. The action is allowed in the workspace's current state:
   - **Subscribed:** the paid capability set in [Spec 17](./17-guest-authoring-and-claim.md) applies.
   - **Trial, within 4-day window:** the trial capability set applies; generation routes additionally check `guest_sessions.prompts_used < 25`.
   - **Trial, expired window:** all write routes return `subscription_required`.
3. Custom-domain and team-invite routes additionally require an active subscription regardless of trial window.
4. Form-submission management and any other email-bearing route requires the session to have a linked `users` row (L2 or paid).

Do not rely on client-side checks.

## Magic Link And Recovery Token Rules

Magic links and recovery keys are both bearer secrets. They must:

- be generated from a cryptographically strong RNG with at least 32 bytes of entropy
- be stored only as a hash in the database, never in plaintext
- be transmitted to the user exactly once (recovery key via the builder UI display, magic link via authenticated outbound mail)

Magic link rules:

- expire 15 minutes after issuance
- are single-use; mark `consumed_at` on first redemption inside the same transaction that issues the resulting session
- are scoped to a specific `user_id` and `purpose` (`login` or `verify_email`)
- the magic-link request endpoint must return an identical generic response for known and unknown emails to prevent enumeration
- the magic-link request endpoint must be rate-limited per IP and per email

Recovery key rules:

- multi-use until claimed (Spec 17 L2 transition) or explicitly revoked
- the redemption endpoint must be rate-limited per IP
- redemption sets a fresh cookie but does not extend `trial_started_at` or reset `prompts_used`

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
- site has no more than 10 editor-visible pages (templates plus static pages; collection-derived URLs do not count)
- every page slug is unique within its type
- every block is valid
- every block binding targets a field that exists on the bound collection's schema and matches the prop's expected type
- every `collection_detail` template references a collection that has at least one published entry
- every entry validates against its collection's current schema
- every internal navigation link resolves
- theme tokens are valid
- required SEO fields have fallbacks
- sitemap inputs are valid (including one URL per published entry on collection_detail templates)
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

## Response Headers

The API must apply a small explicit header policy rather than relying on framework defaults.

### Header Table

| Surface | Header | Value | Notes |
|---|---|---|---|
| authenticated API routes | `Cache-Control` | `private, no-store` | Prevents shared-cache storage of builder data and session-derived responses. |
| authenticated API routes | `Pragma` | `no-cache` | Legacy compatibility for intermediaries. |
| authenticated API routes | `Expires` | `0` | Legacy compatibility for intermediaries. |
| authenticated API routes | `Content-Security-Policy` | `default-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'; object-src 'none'; script-src 'none'` | JSON API responses should not execute active content if mis-served. |
| public render + public form routes | `Content-Security-Policy` | `default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; font-src 'self' data: https:; connect-src 'self'; object-src 'none'; script-src 'none'` | Keeps published pages non-scriptable while allowing inline theme/style markup from the published HTML artifact. |
| public + authenticated API routes | `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | Only meaningful over HTTPS in production, but set uniformly. |
| public + authenticated API routes | `X-Frame-Options` | `DENY` | Published sites and authenticated responses must not be framed. |
| public + authenticated API routes | `X-Content-Type-Options` | `nosniff` | Prevent content-type sniffing. |
| public + authenticated API routes | `Referrer-Policy` | `strict-origin-when-cross-origin` | Preserves useful same-origin referrers while limiting cross-origin leakage. |

### CSP Notes

- The public CSP must continue to allow the published page HTML returned through `dangerouslySetInnerHTML` in [apps/web/src/components/PublishedSitePage.tsx](/home/mosi/projects/snaelda/apps/web/src/components/PublishedSitePage.tsx:42).
- Inline style allowance is intentional for theme tokens and artifact-rendered markup.
- No public route should require inline or external script execution for MVP rendering.

If the public site is pre-rendered on publish, caching becomes simpler because requests can return already-generated page output.
