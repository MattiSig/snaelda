# Block Registry

## MVP Registry Goal

Start with a block set that covers most simple websites without introducing layout complexity.

## Recommended MVP Block Types

### 1. Hero

Purpose: top section with headline, subheadline, CTA, and optional image.

Props:

- eyebrow
- headline
- subheadline
- primary CTA
- secondary CTA
- image asset
- layout variant

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

Purpose: site footer.

Props:

- logo or site name
- navigation links
- social links
- address/contact
- copyright

## Optional Early Additions

- logo cloud
- map/location
- stats/KPIs
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

When a block changes in a breaking way, create a new version such as:

- `hero@1.0.0`
- `hero@1.1.0`
- `hero@2.0.0`

Published snapshots should continue rendering against their stored block versions until explicitly migrated.
