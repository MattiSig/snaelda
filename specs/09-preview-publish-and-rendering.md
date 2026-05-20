# Preview, Publish, and Rendering

## Draft Preview

Preview renders current draft state.

Possible routes:

- `/app/sites/:siteId/preview`
- `/preview/:token`

Preview should assemble normalized draft state into render input on request. This preview can live inside the builder experience and does not need to match the public serving path exactly.

## Publish Workflow

Publishing should:

1. Load current draft state
2. Assemble the full site config snapshot
3. Validate the full snapshot
4. Create a new `site_versions` row
5. Render page output artifacts from that snapshot
6. Store those artifacts in object storage
7. Set `sites.published_version_id`
8. Invalidate public cache for the site/domain
9. Write an audit event

## Public Request Flow

1. Request arrives at `hostname/path`
2. Public service resolves hostname in `site_domains`
3. System finds the site and `published_version_id`
4. System loads the published artifact for the requested path
5. Response is returned

## Publish Artifact Model

The important bridge between the builder and the live site is the publish step.

Recommended direction:

- draft data is editable
- published snapshot is immutable
- page artifacts are generated from the snapshot at publish time
- artifacts are stored in blob/object storage

That lets the live site stay very lightweight.

## SEO Artifacts

Publish should generate not just page HTML but also core SEO artifacts.

Recommended MVP output:

- page HTML
- `sitemap.xml`
- `robots.txt`
- canonical URL metadata in page output
- basic social meta tags in page output
- LocalBusiness JSON-LD on every page that includes a Footer carrying structured `address` and/or `hours` (see [Spec 04](./04-block-registry.md)). The structured shape exists specifically to enable this — generic free-text contact info doesn't qualify for LocalBusiness markup, which is the actual SEO win for the small-business ICP.

## Error Handling Rules

- invalid blocks should be caught before publish
- preview can show safe builder warnings
- published pages should not depend on draft-time repair logic
