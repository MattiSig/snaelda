# Site Configuration Model

## Goal

The generated website config must be the canonical renderable structure understood by the renderer. Published rendering should require only this snapshot and resolved asset URLs.

There are two related shapes in the system:

- canonical draft data for generation and editing
- published snapshot data for live serving and artifact generation

## Example Shape

```json
{
  "schemaVersion": "site-config.v1",
  "site": {
    "id": "site_123",
    "name": "Nordic Lens Studio",
    "defaultLocale": "en",
    "seo": {
      "title": "Nordic Lens Studio",
      "description": "Photography for brands, families, and events."
    }
  },
  "brand": {
    "businessName": "Nordic Lens Studio",
    "logo": {
      "assetId": "asset_logo_primary",
      "alt": "Nordic Lens Studio logo"
    },
    "primaryColor": "#4B6F60"
  },
  "theme": {
    "version": "theme.v1",
    "tokens": {
      "colors": {
        "background": "#F8F7F3",
        "text": "#161616",
        "primary": "#4B6F60",
        "secondary": "#D8C3A5",
        "muted": "#737373"
      },
      "typography": {
        "headingFont": "Inter",
        "bodyFont": "Inter",
        "scale": "modern"
      },
      "shape": {
        "radius": "large",
        "shadow": "soft"
      },
      "layout": {
        "maxWidth": "1120px",
        "sectionSpacing": "comfortable"
      }
    }
  },
  "navigation": {
    "primary": [
      { "label": "Home", "pageId": "page_home" },
      { "label": "Gallery", "pageId": "page_gallery" },
      { "label": "Contact", "pageId": "page_contact" }
    ]
  },
  "collections": [
    {
      "id": "col_services",
      "slug": "services",
      "singularLabel": "Service",
      "pluralLabel": "Services",
      "schema": [
        { "key": "title", "label": "Title", "type": "text", "required": true },
        { "key": "summary", "label": "Summary", "type": "long_text" },
        { "key": "cover", "label": "Cover image", "type": "asset" }
      ],
      "entries": [
        {
          "id": "entry_portraits",
          "slug": "portraits",
          "fields": {
            "title": "Portraits",
            "summary": "Natural light portrait sessions.",
            "cover": { "assetId": "asset_portraits" }
          },
          "seo": { "title": "Portrait sessions | Nordic Lens", "description": "..." },
          "status": "published",
          "sortOrder": 0
        }
      ]
    }
  ],
  "pages": [
    {
      "id": "page_home",
      "type": "static",
      "title": "Home",
      "slug": "/",
      "seo": {
        "title": "Nordic Lens Studio | Photography",
        "description": "Warm, natural photography for families and brands."
      },
      "blocks": [
        {
          "id": "block_hero_1",
          "type": "hero",
          "version": "1.0.0",
          "props": {
            "eyebrow": "Stockholm Photography Studio",
            "headline": "Natural photography for meaningful moments",
            "subheadline": "Portraits, events, and brand photography with a calm Nordic style.",
            "primaryCta": {
              "label": "Book a session",
              "href": "/contact"
            }
          }
        }
      ]
    },
    {
      "id": "page_services_index",
      "type": "collection_index",
      "title": "Services",
      "slug": "/services",
      "collectionId": "col_services",
      "blocks": []
    },
    {
      "id": "page_service_detail",
      "type": "collection_detail",
      "title": "Service detail template",
      "collectionId": "col_services",
      "blocks": [
        {
          "id": "block_hero_service",
          "type": "hero",
          "version": "1.0.0",
          "props": { "headline": "Service" },
          "bindings": {
            "headline": { "source": "entry", "field": "title" },
            "image": { "source": "entry", "field": "cover" }
          }
        }
      ]
    }
  ]
}
```

See [Spec 19](./19-collections-and-content-types.md) for the full collection, entry, and page-type model.

## Model Requirements

- The schema must be versioned, for example `site-config.v1`
- The structure must be fully renderable without re-reading draft tables
- Internal links should resolve cleanly from stored identifiers and slugs
- Block instances must include type, version, and props; bindings are optional and only valid on collection_detail pages
- Theme tokens must use constrained keys
- The snapshot must include every collection and every published entry that any page template binds to; the renderer must not need to re-query draft entries to serve a collection_detail URL
- Brand is a top-level sibling to theme. Theme tokens are derived from `brand.primaryColor` (see [Spec 11](./11-theme-navigation-and-assets.md)); the Header and Footer blocks resolve `businessName` and `logo` from brand rather than carrying duplicate per-block props

## Canonical Draft Contract

The backend should also own a canonical draft contract that the builder edits through an adapter layer.

Example:

```json
{
  "site": {
    "id": "site_123",
    "name": "Nordic Lens Studio",
    "slug": "nordic-lens",
    "status": "draft"
  },
  "brand": {
    "businessName": "Nordic Lens Studio",
    "logo": { "assetId": "asset_logo_primary", "alt": "Nordic Lens Studio logo" },
    "primaryColor": "#4B6F60"
  },
  "theme": {
    "version": "theme.v1",
    "tokens": {
      "colors": {
        "background": "#F8F7F3",
        "text": "#161616",
        "primary": "#4B6F60"
      },
      "typography": {
        "headingFont": "Inter",
        "bodyFont": "Inter"
      }
    }
  },
  "navigation": {
    "primary": [
      { "label": "Home", "pageId": "page_home" }
    ]
  },
  "pages": [
    {
      "id": "page_home",
      "title": "Home",
      "slug": "/",
      "blocks": [
        {
          "id": "block_hero_1",
          "type": "hero",
          "version": "1.0.0",
          "props": {
            "headline": "Natural photography for meaningful moments"
          }
        }
      ]
    }
  ]
}
```

Rules:

- this is the source of truth for draft editing
- the builder should not store raw editor state as the canonical model
- publish converts this draft shape into an immutable published snapshot

## Recommended Internal Types

### Site Draft

```ts
type SiteDraft = {
  site: {
    id: string;
    name: string;
    slug: string;
    status: "draft";
  };
  brand: BrandConfig;
  theme: ThemeConfig;
  navigation: NavigationConfig;
  pages: PageDraft[];
  collections: Array<Collection & { entries: CollectionEntry[] }>;
};

type BrandConfig = {
  businessName: string;
  logo?: {
    assetId: string;
    alt?: string;
  };
  primaryColor: string; // hex; seeds theme token derivation
};
```

### Page Draft

```ts
type PageType =
  | "static"
  | "collection_index"
  | "collection_detail";

type PageDraft = {
  id: string;
  type: PageType;
  title: string;
  slug?: string; // optional for collection_detail (URL pattern comes from the collection)
  collectionId?: string; // required when type is collection-bound
  seo: SeoConfig;
  blocks: BlockInstance[];
  settings?: Record<string, unknown>;
};
```

### Block Instance

```ts
type BlockBinding = {
  source: "entry";
  field: string;
};

type BlockInstance = {
  id: string;
  type: string;
  version: string;
  props: Record<string, unknown>;
  bindings?: Record<string, BlockBinding>;
  settings?: {
    hidden?: boolean;
    anchorId?: string;
  };
};
```

### Collection

```ts
type CollectionFieldType =
  | "text" | "long_text" | "rich_text"
  | "number" | "boolean" | "date"
  | "url" | "email" | "phone" | "location"
  | "enum" | "enum_multi"
  | "asset" | "asset_list"
  | "reference";

type CollectionField = {
  key: string;
  label: string;
  type: CollectionFieldType;
  required?: boolean;
  description?: string;
  options?: string[];               // enum / enum_multi
  defaultValue?: unknown;
  validation?: Record<string, unknown>;
};

type Collection = {
  id: string;
  slug: string;
  singularLabel: string;
  pluralLabel: string;
  schema: CollectionField[];
  settings?: Record<string, unknown>;
  sortOrder: number;
};

type CollectionEntry = {
  id: string;
  collectionId: string;
  slug: string;
  fields: Record<string, unknown>;
  seo: SeoConfig;
  status: "draft" | "published";
  sortOrder: number;
};
```

### Theme Config

```ts
type ThemeConfig = {
  version: "theme.v1";
  tokens: {
    colors: Record<string, string>;
    typography: Record<string, string | number>;
    layout: Record<string, string | number>;
    shape: Record<string, string | number>;
  };
};
```
