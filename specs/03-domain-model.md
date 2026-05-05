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
- theme
- pages
- navigation
- domains

## Page

A page belongs to a site.

A page has:

- title
- slug
- SEO metadata
- order
- status
- blocks

MVP limit: maximum 10 pages per site.

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
