# Theme, Navigation, and Assets

## Brand

Brand identity is a site-level object that the theme system and a small set of blocks read from. It carries the minimum a real business needs to look like itself:

```json
{
  "businessName": "Example Business",
  "logo": {
    "assetId": "asset_logo_primary",
    "alt": "Example Business logo"
  },
  "primaryColor": "#1E5B4F"
}
```

Brand lives next to theme in the site config (see [Spec 05](./05-site-configuration-model.md)), not inside it. The split matters because brand is user-provided business identity, while theme is the platform's derived rendering layer.

### Theme Tokens Derive From Brand

`brand.primaryColor` is the source of the theme palette. The platform produces secondary, accent, surface, and contrast tokens deterministically from the primary color rather than letting AI invent each one. The model still picks typography pairing, spacing, shape, and mood; it does not invent colors.

When `brand.primaryColor` changes (via the brand editor), the theme palette regenerates without manual theme editing.

`brand.primaryColor` can arrive three ways, all treated identically by derivation: typed by the user in the brand editor, inferred from prompt style cues when absent, or extracted from a re-spun source site (Spec 21 brand pull — external-stylesheet CSS variables scored by button-background usage, `theme-color` meta, or dominant logo color).

### Blocks Resolve Brand From Site Context

Header and Footer blocks do not carry their own `logo` or `businessName` props. They read these from `brand` at render time. That keeps brand consistent across every page and removes the failure mode where re-uploading the logo in one place leaves stale copies elsewhere.

Other blocks that need a brand value (rare; mostly the Hero in some layouts) follow the same pattern: resolve from site context, do not duplicate as a prop.

## Theme System

### Theme Tokens

Example token shape:

```json
{
  "colors": {
    "background": "#ffffff",
    "surface": "#f7f7f7",
    "text": "#111111",
    "mutedText": "#666666",
    "primary": "#2f6fed",
    "primaryText": "#ffffff",
    "secondary": "#e8eefc",
    "border": "#e5e5e5"
  },
  "typography": {
    "headingFont": "Inter",
    "bodyFont": "Inter",
    "headingWeight": 700,
    "bodyWeight": 400
  },
  "layout": {
    "maxWidth": "1120px",
    "sectionPaddingY": "96px",
    "sectionPaddingX": "24px"
  },
  "shape": {
    "radius": "16px",
    "buttonRadius": "999px"
  }
}
```

### CSS Variable Output

```css
:root {
  --color-background: #ffffff;
  --color-text: #111111;
  --color-primary: #2f6fed;
  --font-heading: Inter;
  --font-body: Inter;
  --radius-card: 16px;
  --section-padding-y: 96px;
}
```

### Theme Generation Strategy

Prompt-derived themes should map to safe presets.

Examples:

- `minimal luxury` -> high contrast, neutral palette, serif heading option
- `playful startup` -> bright accent, rounded buttons, spacious layout
- `calm nordic` -> muted palette, large whitespace, soft corners

Do not let the model invent unrestricted theme keys.

## CSS and JS Strategy

Use a split model:

- shared CSS and JS owned by the product
- small per-site theme output generated at publish time

### Shared Assets

These should be maintained by the platform:

- block and layout CSS
- responsive rules
- shared interactive JS

### Per-Site Generated Assets

These should be created from the published snapshot:

- `theme.css`
- optional tiny JS only if a block truly needs it

Users should mainly control content and theme tokens, not arbitrary CSS or custom JavaScript.

## Frontend Styling Direction

Tailwind CSS is the styling baseline for the TanStack Start web app and the shared block system.

shadcn/ui is the default component source for the authenticated builder UI. Use it for common controls such as buttons, inputs, dialogs, menus, tabs, forms, loading states, and empty/error states so the app stays consistent and fast to build. Treat shadcn/ui as source-owned app components, not as a customer-facing component marketplace.

Recommended approach:

1. the TanStack Start app is configured with Tailwind CSS, shadcn/ui, the `@/*` import alias, and a shared `cn` utility
2. builder UI starts from shadcn/ui primitives before introducing bespoke components
3. blocks use Tailwind utilities in source code
4. the platform compiles a shared CSS bundle for maintained block and layout styles
5. each published site gets a small token-driven `theme.css`

This keeps public websites lightweight while still allowing prompt-driven visual direction.

## Navigation Model

Navigation should be generated from pages but stored explicitly so users can edit it.

Example:

```json
{
  "primary": [
    { "label": "Home", "pageId": "...", "href": "/" },
    { "label": "Services", "pageId": "...", "href": "/services" },
    { "label": "Contact", "pageId": "...", "href": "/contact" }
  ],
  "footer": [
    { "label": "Privacy", "href": "/privacy" }
  ]
}
```

Rules:

- internal links should prefer `pageId`
- renderer resolves `pageId` to slug
- external links must be validated
- broken page references must be caught before publish

## Asset Handling

Assets should be stored in object storage, not directly in Postgres.

Postgres stores:

- asset id
- workspace id
- site id
- storage key
- public URL or signed URL behavior
- alt text
- dimensions
- file type
- size
- upload metadata

### MVP Asset Flow

1. User uploads image
2. Backend creates a signed upload URL
3. User uploads to storage
4. Backend stores asset metadata
5. Blocks reference images by `assetId`
6. Renderer resolves asset ids to optimized URLs

For MVP, AI-generated imagery is optional. Prefer user-uploaded assets, gradients, placeholders, or stock-safe defaults if image generation is not part of the stack yet.

## Starter Image Integration

To improve first-draft conversion, the generation flow may attach starter imagery from Pexels (the backend integration in [Spec 07](./07-generation-engine.md)).

Recommended rules:

- image search happens through a backend integration, not directly from the model
- the model can request image candidates through a tool call
- selected images should store source metadata and attribution fields
- starter imagery should be clearly replaceable by the user

Recommended MVP scope:

- one hero image by default
- optional small gallery for visually driven businesses

Do not treat these images as final brand assets. They are a useful starting point, not a substitute for customer-owned imagery.
