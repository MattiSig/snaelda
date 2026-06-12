# Collections and Content Types

## Purpose

This spec introduces user-defined collections of structured entries into the website-as-data model. Collections let a site own typed lists like `services`, `projects`, `staff`, `menu_items`, `events`, or `properties`, and let pages be generated from those entries rather than hand-authored one at a time.

Three product capabilities depend on this:

- service-detail pages: one page per real service the business sells, all rendered from a shared template
- project case studies: structured portfolio entries with consistent fields, rendered from a shared template
- programmatic local SEO: AI generates one entry per `(service × location)` combination as full, independent entries — see [Programmatic SEO via Variant Entries](#programmatic-seo-via-variant-entries) below

Without collections, every service/project page is a hand-built duplicate. With collections, the owner edits the entry once and every page that renders from it updates. Programmatic-SEO variants stay as separate entries by design — see why below.

This spec is the source of truth for the collection model. Other specs reference it for shape, validation, generation, editing, API, and runtime behavior.

## Core Principle

A collection is **typed, schema-defined, and site-scoped**. Entries match the collection's schema. Pages may render from entries. The model is generic — the platform does not hard-code `services` or `projects`; those are just collections an owner (or AI) creates.

This is not a generic database for users. Collections sit inside the same "website is data, not code" guardrail as blocks: a closed field-type registry, validation at every boundary, and immutable published snapshots.

## Domain Shape

### Collection

A collection has:

- `id`
- `siteId`
- `slug` (URL-safe, e.g. `services`, `projects`, `case-studies`)
- `singularLabel` (e.g. `Service`)
- `pluralLabel` (e.g. `Services`)
- `schema` — ordered list of field definitions
- `settings` — collection-level options (default sort, whether entries have public detail URLs, default SEO template)
- `sortOrder` — ordering of collections in the editor

Constraints:

- collection `slug` is unique per site
- a site may own up to N collections (plan-gated; see [Spec 15](./15-billing-and-stripe.md))
- collection `slug` is reserved as a URL prefix when the collection is configured to expose public detail URLs

### Field Type Registry

Field types live in code as a closed registry, the same way block types do. AI and the editor may only choose from this set.

MVP field types:

| Type          | Purpose                                  | Stored value shape                   |
|---------------|------------------------------------------|--------------------------------------|
| `text`        | Short single-line text                   | `string`                             |
| `long_text`   | Multi-line plain text                    | `string`                             |
| `rich_text`   | Constrained markup (headings/lists/links/bold/italic) | normalized doc JSON         |
| `number`      | Numeric value                            | `number`                             |
| `boolean`     | Yes/no flag                              | `boolean`                            |
| `date`        | Calendar date                            | ISO date string                      |
| `url`         | External or internal URL                 | `string`                             |
| `email`       | Email address                            | `string`                             |
| `phone`       | Phone number                             | `string`                             |
| `location`    | Place reference                          | `{ name, region?, country?, lat?, lng? }` |
| `enum`        | Single choice from defined options       | `string` matching one option         |
| `enum_multi`  | Multiple choices from defined options    | `string[]` of options                |
| `asset`       | One image/file from the asset library    | `{ assetId, alt? }`                  |
| `asset_list`  | Ordered gallery                          | `{ assetId, alt? }[]`                |
| `reference`   | Link to another entry in any collection on this site | `{ collectionId, entryId }`  |

Each field definition has:

- `key` — programmatic key (snake_case)
- `label` — human label shown in the editor
- `type` — one of the registry types above
- `required` — boolean
- `description` — optional editor helper text
- `options` — closed-set values for `enum` / `enum_multi`
- `defaultValue` — optional
- `validation` — per-type constraints (max length, min/max number, allowed asset kinds)

Schemas are versioned on the collection. A non-additive schema change (rename, type change, remove required field) requires a migration step that maps old entries forward.

### Collection Entry

An entry has:

- `id`
- `collectionId`
- `siteId`
- `slug` — URL-safe identifier, unique per collection (used for detail URLs)
- `fields` — values keyed by field definition `key`, validated against the schema
- `seo` — `{ title, description, ogImage? }` (used by detail pages)
- `status` — `draft` | `published`
- `sortOrder` — position in default listing

Entries are normalized rows. They are not stored as opaque blobs on the collection.

## Pages and Collections

Pages have a `type`. The default `static` type matches today's hand-authored pages. Two new page types bind to collections:

### `static`

A hand-authored page with hand-placed blocks. Today's default. Unchanged.

### `collection_index`

A single page that lists entries from one collection. Optional filter/sort controls drive from `enum` / `enum_multi` fields. URL is the page's own slug (e.g. `/services`).

Settings on this page type:

- `collectionId`
- default sort
- per-filter visibility
- pagination (deferred; MVP shows all entries)

### `collection_detail`

A page template that renders one entry. The template exists as one page row but serves one URL per published entry in its collection.

URL pattern: `/{collection.slug}/{entry.slug}` by default. Optionally `/{page.slug}/{entry.slug}` if the owner wants a custom prefix.

The template's blocks may bind to entry fields (see [Block Bindings](#block-bindings) below). At render time the binding pulls the entry's field value into the block's prop.

Settings on this page type:

- `collectionId`
- SEO template (uses entry `seo` with site-level fallback)

### Page Cap Reframed

The old "10 pages per site" cap in [Spec 01](./01-product-summary-and-scope.md) was about authoring scope, not URL count. With collections, the cap applies to **editor-visible page rows** (templates + static pages), not the URLs those templates produce. URL count is gated by entry count, which is governed by plan entitlements in [Spec 15](./15-billing-and-stripe.md).

## Programmatic SEO via Variant Entries

The case study calls for many search-targeted URLs from a single service ("Hyra byggställning Göteborg", "Hyra byggställning Kungsbacka", "Hyra byggställning Mölndal"). The platform handles this **without a dedicated page type**: each variant is its own entry in the same collection.

```
Byggnadsställningar              →  /services/byggnadsstallningar
Byggnadsställningar Göteborg     →  /services/byggnadsstallningar-goteborg
Byggnadsställningar Kungsbacka   →  /services/byggnadsstallningar-kungsbacka
Byggnadsställningar Mölndal      →  /services/byggnadsstallningar-molndal
```

Each variant entry has full, AI-rewritten copy for its city — not a templated near-duplicate. This is deliberate: search engines rank pages that look like genuine per-location content, not copy-with-the-city-name-swapped. Inheritance-style overrides would have quietly pushed users toward the spammy near-duplicate failure mode.

The AI maintenance action that produces variants is documented in [Spec 07](./07-generation-engine.md):

> "Generate location variants for {entry} in {Göteborg, Kungsbacka, Mölndal}" — creates N draft entries in the same collection, each with location-specific copy. User reviews and publishes.

Optional grouping: variant entries may set a `reference` field on themselves pointing to the base entry, so the editor can render them as children of "Byggnadsställningar". This uses the existing field-type machinery — no new concept.

This pattern is not limited to locations. It works for any axis the user prompts for (industry segment, customer type, language, season). Treating variants as data rather than as a structural axis on the template keeps the data model flat and the AI surface flexible.

## Block Bindings

Existing blocks (hero, image+text, gallery, FAQ, CTA, etc.) need to work inside detail and location templates without duplicating their schemas. A binding is an optional per-prop config that says "pull this prop's value from the current entry's field."

Binding shape:

```json
{
  "id": "block_hero_2",
  "type": "hero",
  "version": "1.0.0",
  "props": {
    "headline": "Service headline default",
    "image": null
  },
  "bindings": {
    "headline": { "source": "entry", "field": "title" },
    "image":    { "source": "entry", "field": "cover" }
  }
}
```

Rules:

- bindings are only valid in `collection_detail` templates
- a binding overrides the literal prop value at render time
- the binding target field must match the prop's expected type (text → text/long_text; image → asset; etc.) — validated at save and at publish

A small `collection_list` block (new in [Spec 04](./04-block-registry.md)) renders N entries on a static page or homepage, with optional filter chips.

## Site Configuration

Collections are part of the site config snapshot (see [Spec 05](./05-site-configuration-model.md)):

```json
{
  "schemaVersion": "site-config.v1",
  "site": { "...": "..." },
  "theme": { "...": "..." },
  "navigation": { "...": "..." },
  "collections": [
    {
      "id": "col_services",
      "slug": "services",
      "singularLabel": "Service",
      "pluralLabel": "Services",
      "schema": [
        { "key": "title", "label": "Title", "type": "text", "required": true },
        { "key": "summary", "label": "Summary", "type": "long_text" },
        { "key": "details", "label": "Details", "type": "rich_text" },
        { "key": "cover", "label": "Cover image", "type": "asset" },
        { "key": "gallery", "label": "Gallery", "type": "asset_list" },
        { "key": "categories", "label": "Categories", "type": "enum_multi",
          "options": ["residential", "commercial", "industrial"] }
      ],
      "entries": [
        {
          "id": "entry_scaffolding",
          "slug": "scaffolding",
          "fields": {
            "title": "Byggnadsställningar",
            "summary": "Skalbar ställning för fastigheter och byggprojekt.",
            "cover": { "assetId": "asset_..." },
            "categories": ["commercial"]
          },
          "seo": { "title": "...", "description": "..." },
          "status": "published",
          "sortOrder": 0
        }
      ]
    }
  ],
  "pages": [
    { "id": "page_services_index", "type": "collection_index",
      "slug": "/services", "collectionId": "col_services", "blocks": [] },
    { "id": "page_service_detail", "type": "collection_detail",
      "collectionId": "col_services", "blocks": [/* with bindings */] }
  ]
}
```

## Generation

The generation engine (see [Spec 07](./07-generation-engine.md)) may produce collections when the prompt implies them. The case study examples imply:

- trades / construction: `services` (multi-entry), `projects` (multi-entry)
- signage / print: `services`, `portfolio`
- hospitality / rental: `properties` (often single-entry), `experiences` (optional)
- restaurants: `menu_items`, `events`

The model is not required to use any specific collection name. It chooses what fits the prompt and selects from the closed field-type registry.

Generation rules:

- collection schemas must validate against the field-type registry
- enum options must be enumerated up front, not invented at entry-write time
- AI-seeded entries must validate against their collection's schema
- starter imagery for `asset` / `asset_list` fields follows the Pexels integration rules in [Spec 11](./11-theme-navigation-and-assets.md)

AI maintenance actions explicitly enabled by collections (see [Spec 07](./07-generation-engine.md) and [Spec 08](./08-editor-and-authoring.md)):

- "Turn these photos into a project" — creates a `projects` entry from selected assets, AI fills fields
- "Add a service" — creates a `services` entry from a one-line description
- "Generate location variants for {entry} in {cities}" — creates N draft entries in the same collection, each with full per-city copy; see [Programmatic SEO via Variant Entries](#programmatic-seo-via-variant-entries)
- "Generate FAQ from services" — creates entries in an `faqs` collection from the existing `services` entries
- "Rewrite this entry" — re-prompts a single entry's fields

## Editor

See [Spec 08](./08-editor-and-authoring.md) for the full builder model. The collections surface adds:

- a Collections tab in the site shell listing all collections
- a schema editor per collection (constrained to the field-type registry; reorder, add, remove, edit fields)
- an entries list per collection with reorder, duplicate, delete, status toggle
- a per-entry editor that renders inputs from the schema (rich text editor for `rich_text`, asset picker for `asset`, etc.)
- a page-template editor that shows block field-binding controls when the page type is `collection_detail`

Editing constraints:

- destructive schema changes (rename field, change type, delete required field) trigger a migration prompt the user must acknowledge
- entries cannot be saved if they fail schema validation
- a collection cannot be deleted while pages bind to it; the editor surfaces what would break

## API Surface

See [Spec 10](./10-api-surface.md) for the full route list. The collection routes are:

```http
GET    /api/sites/:siteId/collections
POST   /api/sites/:siteId/collections
POST   /api/sites/:siteId/collections/draft-from-prompt
GET    /api/sites/:siteId/collections/:collectionId
PATCH  /api/sites/:siteId/collections/:collectionId
DELETE /api/sites/:siteId/collections/:collectionId

GET    /api/sites/:siteId/collections/:collectionId/entries
POST   /api/sites/:siteId/collections/:collectionId/entries
GET    /api/sites/:siteId/collections/:collectionId/entries/:entryId
PATCH  /api/sites/:siteId/collections/:collectionId/entries/:entryId
DELETE /api/sites/:siteId/collections/:collectionId/entries/:entryId
POST   /api/sites/:siteId/collections/:collectionId/entries/reorder

POST   /api/sites/:siteId/collections/:collectionId/entries/draft-from-prompt
```

The shipped prompt routes generate and persist immediately. Collection-schema prompting creates the collection; entry prompting creates entries with `status=draft`. These operations must use shared prompt quotas, jobs, rate limits, audit records, and atomic persistence.

Required additions:

```http
POST   /api/sites/:siteId/collections/:collectionId/schema/migrate
POST   /api/sites/:siteId/collections/:collectionId/entries/:entryId/reprompt
```

All routes share the unified-session authorization model in [Spec 17](./17-guest-authoring-and-claim.md). Generation routes count against the trial 25-prompt budget.

## Database

See [Spec 06](./06-database-design.md) for the full schema. Collections add:

```sql
create table collections (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  slug text not null,
  singular_label text not null,
  plural_label text not null,
  schema jsonb not null,
  settings jsonb not null default '{}'::jsonb,
  sort_order int not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (site_id, slug)
);

create table collection_entries (
  id uuid primary key default gen_random_uuid(),
  collection_id uuid not null references collections(id) on delete cascade,
  site_id uuid not null references sites(id) on delete cascade,
  slug text not null,
  fields jsonb not null default '{}'::jsonb,
  seo jsonb not null default '{}'::jsonb,
  status text not null default 'draft',
  sort_order int not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (collection_id, slug)
);

create index collection_entries_site_idx on collection_entries(site_id);
create index collection_entries_status_idx on collection_entries(collection_id, status);
```

`pages` gains:

```sql
alter table pages add column type text not null default 'static';
alter table pages add column collection_id uuid references collections(id) on delete restrict;
```

Allowed `pages.type` values: `static`, `collection_index`, `collection_detail`.

`block_instances.bindings` is added as `jsonb not null default '{}'::jsonb` and validated against the block's prop schema plus the bound entry's field type.

## Public Runtime

See [Spec 16](./16-runtime-lifecycles-and-analytics.md). Collections and entries are included in the published snapshot. Public URL resolution adds:

- `collection_index` page slug → renders the index page from the snapshot
- `{collection.slug}/{entry.slug}` → resolves to the published entry and renders the collection_detail template

Only published entries are publicly routable. Draft entries are reachable only through preview tokens. Cache invalidation on publish covers the index page and every detail URL whose entry changed.

Sitemap generation includes one URL per published collection_detail entry.

## Validation Rules

At every save and at publish, the platform enforces:

- entry field values match the collection's current schema
- enum / enum_multi values are inside the declared options
- asset references resolve to assets owned by the same workspace
- reference fields resolve to existing entries in the named collection
- collection slug and entry slug are URL-safe and unique within their parent
- block bindings target only entry fields whose type matches the bound prop
- collection_detail pages reference a collection that exists and has at least one published entry before the page can publish

Publish must fail loudly on any binding whose target field was removed or retyped without migration.

## Out of Scope for MVP

- nested collections (a collection inside a collection) — references handle relations
- per-entry permissions
- arbitrary user-defined field types beyond the registry
- collection-level workflow states beyond `draft` / `published`
- per-language entry variants
- programmatic regeneration of all collection URLs on a schedule
- collection-bound forms beyond the existing `contact_form` block

These can be added later. The MVP target is the case-study wedge: services, projects, and programmatic local SEO via AI-generated variant entries.
