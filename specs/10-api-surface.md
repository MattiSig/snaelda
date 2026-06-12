# API Surface

A site is the only top-level authoring resource. Pages and blocks are aggregates of a site — they have no independent lifecycle, cannot be addressed without their parent, and share the site's transactional boundary. They are therefore exposed as nested routes under `/api/sites/:siteId`, not as separate top-level resources.

## Sites

```http
POST /api/sites/generate
POST /api/sites/:siteId/reprompt
POST /api/sites/:siteId/undo
GET /api/sites
GET /api/sites/:siteId
PATCH /api/sites/:siteId
DELETE /api/sites/:siteId
```

## Pages

```http
POST /api/sites/:siteId/pages
PATCH /api/sites/:siteId/pages/:pageId
DELETE /api/sites/:siteId/pages/:pageId
POST /api/sites/:siteId/pages/reorder
POST /api/sites/:siteId/pages/:pageId/reprompt
```

## Blocks

```http
POST /api/sites/:siteId/pages/:pageId/blocks
PATCH /api/sites/:siteId/pages/:pageId/blocks/:blockId
DELETE /api/sites/:siteId/pages/:pageId/blocks/:blockId
POST /api/sites/:siteId/pages/:pageId/blocks/reorder
POST /api/sites/:siteId/pages/:pageId/blocks/:blockId/duplicate
```

## Theme

```http
GET /api/sites/:siteId/theme
PATCH /api/sites/:siteId/theme
POST /api/sites/:siteId/theme/regenerate
```

## Collections

Collections and their entries are nested under sites, since a collection has no lifecycle outside its parent site.

```http
GET    /api/sites/:siteId/collections
POST   /api/sites/:siteId/collections
POST   /api/sites/:siteId/collections/draft-from-prompt
GET    /api/sites/:siteId/collections/:collectionId
PATCH  /api/sites/:siteId/collections/:collectionId
DELETE /api/sites/:siteId/collections/:collectionId
```

`POST .../collections/draft-from-prompt` is the shipped AI-assisted collection-schema flow. It validates and persists the proposed collection immediately. It must use the shared generation quota, job, rate-limit, audit, and usage-accounting infrastructure.

Schema migration remains a required addition:

```http
POST   /api/sites/:siteId/collections/:collectionId/schema/migrate
```

## Collection Entries

```http
GET    /api/sites/:siteId/collections/:collectionId/entries
POST   /api/sites/:siteId/collections/:collectionId/entries
GET    /api/sites/:siteId/collections/:collectionId/entries/:entryId
PATCH  /api/sites/:siteId/collections/:collectionId/entries/:entryId
DELETE /api/sites/:siteId/collections/:collectionId/entries/:entryId
POST   /api/sites/:siteId/collections/:collectionId/entries/reorder
POST   /api/sites/:siteId/collections/:collectionId/entries/draft-from-prompt
```

`POST .../entries/draft-from-prompt` is the shipped bulk drafting flow. It persists generated entries with `status=draft`. It counts against prompt allowances and must persist the batch atomically.

Entry-level replacement remains a required addition:

```http
POST   /api/sites/:siteId/collections/:collectionId/entries/:entryId/reprompt
```

## Assets

```http
POST /api/assets/upload-url
POST /api/assets/complete
GET /api/sites/:siteId/assets
PATCH /api/assets/:assetId
DELETE /api/assets/:assetId
```

## Preview and Publish

```http
POST /api/sites/:siteId/preview-token
POST /api/sites/:siteId/publish
GET /api/sites/:siteId/versions
POST /api/sites/:siteId/rollback/:versionId
```

## Domains

```http
POST   /api/sites/:siteId/domains
PATCH  /api/sites/:siteId/domains/:id
DELETE /api/sites/:siteId/domains/:id
```

Custom domain routes require an active subscription on the workspace.

## Forms

```http
POST /api/public/forms/:siteId/:blockId/submit
GET /api/sites/:siteId/form-submissions
PATCH /api/form-submissions/:submissionId
```

## Analytics

```http
GET /api/sites/:siteId/analytics?window=7d|30d|all
```

A single endpoint returns total views, per-page views, and a gap-filled daily trend for the requested window.

## Sessions And Recovery

```http
POST   /api/sessions/anonymous             # create or reuse a cookie-bound trial workspace
GET    /api/sessions/me                    # current session state: layer, prompts_used, trial_ends_at, subscribed flag
POST   /api/sessions/restore               # consume a recovery key, set a fresh cookie
POST   /api/sessions/recovery-key          # mint or regenerate the workspace recovery link
DELETE /api/sessions/recovery-key          # revoke the current recovery link
POST   /api/sessions/claim                 # promote session to L2; creates users row, sends verify magic link
```

## Authentication

```http
POST /api/auth/magic-link                  # request a login magic link by email
GET  /api/auth/magic                       # consume a magic-link token, set session cookie
POST /api/auth/logout
```

See [Spec 17](./17-guest-authoring-and-claim.md) for the trial session model, identity layers (L0/L1/L2), and the subscribe flow.

## API Notes

- The standard builder authoring routes accept either a trial-session cookie or an authenticated user session. Authorization in both cases verifies the session's workspace owns the target resource.
- Trial sessions must additionally pass the 4-day window check and, on generation routes, the 25-prompt budget check, per Spec 17.
- Custom-domain routes are paid-only and must reject trial sessions even within the 4-day window.
- Form-submission management remains workspace-member only and is unreachable from cookie-only sessions that have not attached an email.
- Public form submission routes must be rate-limited and validated.
- Publish and rollback routes always operate on fully validated snapshots.
