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
  "pages": [
    {
      "id": "page_home",
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
    }
  ]
}
```

## Model Requirements

- The schema must be versioned, for example `site-config.v1`
- The structure must be fully renderable without re-reading draft tables
- Internal links should resolve cleanly from stored identifiers and slugs
- Block instances must include type, version, and props
- Theme tokens must use constrained keys

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
  theme: ThemeConfig;
  navigation: NavigationConfig;
  pages: PageDraft[];
};
```

### Page Draft

```ts
type PageDraft = {
  id: string;
  title: string;
  slug: string;
  seo: SeoConfig;
  blocks: BlockInstance[];
};
```

### Block Instance

```ts
type BlockInstance = {
  id: string;
  type: string;
  version: string;
  props: Record<string, unknown>;
  settings?: {
    hidden?: boolean;
    anchorId?: string;
  };
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
