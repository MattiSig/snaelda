# API Surface

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
POST /api/pages/:pageId/reprompt
PATCH /api/pages/:pageId
DELETE /api/pages/:pageId
POST /api/sites/:siteId/pages/reorder
```

## Blocks

```http
POST /api/pages/:pageId/blocks
PATCH /api/blocks/:blockId
DELETE /api/blocks/:blockId
POST /api/pages/:pageId/blocks/reorder
POST /api/blocks/:blockId/duplicate
```

## Theme

```http
GET /api/sites/:siteId/theme
PATCH /api/sites/:siteId/theme
POST /api/sites/:siteId/theme/regenerate
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

## Forms

```http
POST /api/public/forms/:siteId/:blockId/submit
GET /api/sites/:siteId/form-submissions
PATCH /api/form-submissions/:submissionId
```

## Analytics

```http
GET /api/sites/:siteId/analytics/views
GET /api/sites/:siteId/analytics/views/pages
```

## API Notes

- Authenticated routes must verify workspace membership and resource ownership
- Public form submission routes must be rate-limited and validated
- Publish and rollback routes must always operate on fully validated snapshots
