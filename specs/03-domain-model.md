# Domain Model

## Workspace

A workspace owns sites. For the MVP, a user can have one default workspace. Later this model can support teams, roles, billing, and client collaboration.

## Site

A site is the top-level website entity.

A site has:

- name
- slug
- status
- owner workspace
- draft config
- published version pointer
- default locale
- brand
- theme
- pages
- navigation
- domains

## Brand

Brand identity is site-level data, sibling to theme. It carries the minimum a real business needs to look like itself:

- `businessName`
- `logo` — an asset reference with alt text
- `primaryColor` — the source color theme tokens are derived from

Brand feeds two systems:

- **Theme** derives its palette from `brand.primaryColor` rather than inventing one; see [Spec 11](./11-theme-navigation-and-assets.md).
- **Header and Footer blocks** resolve `logo` and `businessName` from brand at render time. They do not carry duplicate per-block props for these fields, which keeps the brand consistent across every page without re-entering it.

Generation accepts brand as a first-class input; see [Spec 07](./07-generation-engine.md).

## Page

A page belongs to a site.

A page has:

- title
- slug
- SEO metadata
- order
- status
- type — one of `static`, `collection_index`, `collection_detail`
- collection reference (required when `type` is collection-bound)
- blocks (block bindings to entry fields are valid in `collection_detail`)
- settings (per-type settings)

A `static` page is hand-authored and serves one URL.

A `collection_index` page lists entries from one collection.

A `collection_detail` page is a template that renders one URL per entry in its collection. URL pattern: `/{collection.slug}/{entry.slug}`.

Programmatic-SEO patterns like "service × city" are handled at the **entry** level: AI generates one full entry per variant (e.g. `Byggnadsställningar Göteborg`, `Byggnadsställningar Kungsbacka`), and the same `collection_detail` template renders each. There is no separate location page type. See [Spec 19](./19-collections-and-content-types.md).

MVP limit: maximum 10 editor-visible pages per site. URLs produced by collection-bound templates do not count against the page cap; they are gated by entry count via plan entitlements.

## Block Definition

A block definition is a maintained component type owned by the application codebase.

Examples:

- `hero`
- `features_grid`
- `gallery`
- `contact_form`

Each block definition includes:

- type
- version
- display name
- category
- JSON schema for props
- default props
- editor schema
- renderer component mapping
- allowed child or slot behavior if needed
- migration rules from older versions

## Block Instance

A block instance is a concrete use of a block on a page.

Each block instance has:

- block type
- block version
- page id
- sort order
- props JSON
- visibility settings
- limited responsive options for MVP
- optional bindings JSON that maps individual props to fields on the current collection entry context (only valid on `collection_detail` pages); see [Spec 19](./19-collections-and-content-types.md)

## Collection

A collection is a site-scoped typed list of entries. It has:

- slug
- singular and plural labels
- a schema of field definitions drawn from a closed field-type registry (text, long_text, rich_text, number, boolean, date, url, email, phone, location, enum, enum_multi, asset, asset_list, reference)
- settings (default sort, whether entries expose public detail URLs, SEO template)

## Collection Entry

A collection entry is one row inside a collection. It has:

- collection id
- slug (unique per collection, used in detail URLs)
- fields keyed by the collection schema's field keys
- SEO metadata
- status: `draft` or `published`
- sort order

Only published entries are publicly routable through collection_detail templates.

See [Spec 19](./19-collections-and-content-types.md) for the full collection and field-type model.

## Theme

A theme is structured design-token data, not arbitrary CSS.

Theme areas include:

- color palette
- font pairing
- spacing scale
- border radius
- shadow intensity
- button style
- section width
- image style
- navigation style
- tone or mood metadata

The renderer converts theme tokens into CSS variables.

## Version

A version is an immutable snapshot of the site at publish time.

- Draft state changes freely
- Published state remains stable and reproducible
- Publishing creates a new `site_versions` row with the complete renderable snapshot
