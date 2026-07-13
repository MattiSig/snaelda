# Block Registry

## MVP Registry Goal

Start with a block set that covers most simple websites without introducing layout complexity.

The registry is one flat, closed set. Curation of blocks, variants, and theme ranges into per-vertical sets (a salon set, a trades set, a café set) is a selection layer on top of this registry, owned by [Spec 23](./23-vertical-block-sets.md); it never adds block types or reopens the registry.

## Recommended MVP Block Types

### 1. Hero

Purpose: top section with headline, subheadline, CTA, and optional image.

Props:

- variant — `standard` (default), `full-page`, or `statement`
- eyebrow
- headline
- subheadline
- primary CTA
- secondary CTA
- image asset
- layout variant — applies to the `standard` variant only; values: `centered`, `split-left`, `split-right`

Variants:

- `standard`: the headline-led hero with optional supporting image. The `layout` prop controls whether the image is centered below the headline (`centered`) or sits beside it (`split-left`, `split-right`).
- `full-page`: an immersive hero that fills exactly the visitor's viewport on first load (100vh). The image becomes the background; the eyebrow, headline, subheadline, and CTAs sit over a darkening gradient at the bottom. When this hero is the first block on a page, it covers the page header so the viewport opens fully on the hero. An image is strongly recommended; the renderer falls back to a themed gradient if one is missing. The `layout` prop is ignored in this variant.
- `statement`: a deliberately image-free, type-led hero — oversized display headline on a solid brand-primary (or near-solid tinted) background, with the eyebrow, subheadline, and CTAs set against it in high contrast. Its job is drama through typography and color, not photography: it must not render as a modest centered `standard` hero with the image missing. The `image` prop is ignored; the `layout` prop is ignored. This is an **additive** variant — ships as `hero@1.1.0` with passthrough migration, no breaking change.

Generation should pick `full-page` for image-led brands (photographers, restaurants, hotels, florists, galleries, salons, ceramics studios, wedding planners, tattoo artists, cafes, food, travel, fashion) or when the prompt asks for a bold, atmospheric opener. It should pick `statement` for text-led trades and services (plumbers, electricians, lawyers, accountants, consultants) with no strong hero photography, and for re-spins whose extracted source hero is text-only (`sourceHero.textOnly`, Spec 21) — matching the source's energy with type and brand color instead of a stock photo. Headlines should stay short (3–7 words) and subheadlines to a single sentence so the overlay reads cleanly.

### 2. Text Section

Purpose: simple rich text or about section.

Props:

- heading
- body
- alignment
- width

### 3. Image + Text

Purpose: explain a service or product with a supporting image.

Props:

- heading
- body
- image
- image position
- optional CTA

### 4. Features Grid

Purpose: list features, services, or benefits.

Props:

- heading
- intro
- items array with icon, title, and body
- columns

### 5. Gallery

Purpose: portfolio or image showcase.

Props:

- heading
- images array
- layout

MVP layout options:

- grid
- masonry-like grid
- simple carousel-like presentation if implemented safely

### 6. Testimonials

Purpose: social proof.

Props:

- heading
- testimonials array with quote, name, role/company, and optional avatar

### 7. Pricing / Packages

Purpose: simple packages or service tiers.

Props:

- heading
- intro
- plans array with name, price, description, features, and CTA

### 8. CTA Band

Purpose: conversion-focused section.

Props:

- heading
- body
- CTA
- style variant

### 9. Contact Form

Purpose: lead capture.

Props:

- heading
- intro
- fields
- success message
- notification email

Allowed MVP form fields:

- name
- email
- phone optional
- message
- select optional

### 10. FAQ

Purpose: answer common questions.

Props:

- heading
- items array with question and answer

### 11. Team / Profile Cards

Purpose: team, personal page, or company profile section.

Props:

- heading
- people array with name, role, bio, photo, and links

### 12. Footer

Purpose: site footer with structured business contact information.

Props:

- navigation links
- social links
- copyright
- contact:
  - `address` — optional structured `{ street, city, postalCode, region, country }`
  - `phone` — optional E.164-style string
  - `email` — optional
  - `hours` — optional structured `{ day, opens, closes, closed? }[]` covering each day of the week
- showBrand — whether the footer renders `brand.businessName` and `brand.logo` (default true)
- showMadeWith — whether the footer renders a small "Made with Snælda" link (default true, toggleable by the site owner)

`businessName` and `logo` are not props on this block. They are resolved from site-level brand at render time (see [Spec 11](./11-theme-navigation-and-assets.md)), which keeps brand consistent across every page.

When `address` and/or `hours` are present, the renderer emits a LocalBusiness JSON-LD block on every page that includes this footer; see [Spec 09](./09-preview-publish-and-rendering.md). This is the SEO win that justifies the structured shape — generic free-text contact info doesn't qualify for LocalBusiness markup.

### 13. Stats / KPI

Purpose: a horizontal row of 3–5 large numbers paired with short labels, used near the top of a homepage or above a CTA to anchor trust with track-record proof.

Visual hierarchy is inverted from Features Grid: the number is the hero, the label is the explanation. Features Grid would render numbers as body text, which defeats the point.

Props:

- eyebrow
- heading
- intro
- items array, each with:
  - `value` — string (not number), so units and symbols are free: `"12"`, `"240"`, `"98%"`, `"4.9★"`, `"€2M+"`, `"24/7"`
  - `label` — short, 1–2 words
  - `subLabel` — optional qualifier (`"since 2008"`, `"(Google)"`)
- columns — 3, 4, or 5
- layout — `centered` or `left`
- style — `plain` or `card`

Generation picks Stats when the prompt implies a track-record proof point (years in business, projects completed, customers served, nights hosted, rating). Generation does not include Stats by default for new businesses with no track record — fake numbers are worse than no Stats block.

### 14. Collection List

Purpose: render entries from a site collection on a static page (e.g. "Featured projects" on the homepage, "All services" on a services index page).

Props:

- heading
- intro
- `collectionId`
- `limit` — max entries to render
- `sort` — collection field key, with direction
- `filter` — optional fixed filter `{ field, value }` for `enum` / `enum_multi` fields
- `layout` — `grid`, `list`, or `carousel`
- itemCardTemplate — which entry fields map to the card's title, image, summary, and link

See [Spec 19](./19-collections-and-content-types.md) for the collection model. This block is the surface that brings collection content into static pages; `collection_detail` templates render single entries directly via bindings rather than this block.

## Optional Early Additions

- logo cloud
- map/location
- blog/article teaser without full CMS
- simple embed block limited to safe providers

## Registry Lives in Code

The block registry should live in application code as versioned definitions:

```ts
export const blockRegistry = {
  hero: {
    versions: {
      "1.0.0": {
        schema: heroSchemaV1,
        defaultProps: heroDefaultsV1,
        editor: heroEditorConfigV1,
        render: HeroBlockV1,
        migrateFromPrevious: null
      }
    }
  }
};
```

## Block Authoring Contract

Each block definition should cover three responsibilities.

### 1. Generation Contract

This defines what AI is allowed to create:

- block type
- prop schema
- allowed variants
- defaults
- validation rules

### 2. Builder Contract

This defines how the block is edited:

- field definitions
- labels
- input types
- option lists
- media pickers
- builder preview behavior

### 3. Publish Contract

This defines how the block becomes live website output:

- render component/template
- prop-to-HTML mapping
- CSS hooks
- optional JS requirements

The same block prop contract should be shared across all three stages.

## Why Registry Data Is Not DB-First

The renderer depends on actual components, so block definitions cannot be treated as database-only records.

For MVP:

- components and schemas live in code
- block instances and props live in the database
- block metadata can optionally be mirrored to the database for admin/editor display

## Example Block Schema

```ts
const heroSchema = z.object({
  variant: z.enum(["standard", "full-page"]).default("standard"),
  eyebrow: z.string().max(80).optional(),
  headline: z.string().min(1).max(120),
  subheadline: z.string().max(280).optional(),
  primaryCta: z
    .object({
      label: z.string().max(40),
      href: z.string()
    })
    .optional(),
  secondaryCta: z
    .object({
      label: z.string().max(40),
      href: z.string()
    })
    .optional(),
  image: z
    .object({
      assetId: z.string(),
      alt: z.string().optional()
    })
    .optional(),
  layout: z.enum(["centered", "split-left", "split-right"]).default("centered")
});
```

## Versioning Rules

Every block instance stores:

- `type`
- `version`
- `props`
- optional `bindings` (see below)

When a block changes in a breaking way, create a new version such as:

- `hero@1.0.0`
- `hero@1.1.0`
- `hero@2.0.0`

Published snapshots should continue rendering against their stored block versions until explicitly migrated.

## Block Bindings (Collection Templates)

When a block is placed inside a `collection_detail` page template, individual props may bind to fields on the current collection entry. At render time the binding replaces the literal prop value with the entry field value.

Binding shape on a block instance:

```json
{
  "bindings": {
    "headline": { "source": "entry", "field": "title" },
    "image":    { "source": "entry", "field": "cover" }
  }
}
```

Rules:

- bindings are only valid in `collection_detail` templates; the validator rejects bindings on `static` and `collection_index` pages
- the bound field's type must match the prop's expected type (text/long_text/rich_text → string-shaped props; asset → image-shaped props; etc.) — enforced at save and at publish
- removing or retyping a bound field requires schema migration; publish fails loudly if a binding references a missing or mismatched field

See [Spec 19](./19-collections-and-content-types.md) for the field-type registry and the programmatic-SEO-via-variant-entries pattern.
