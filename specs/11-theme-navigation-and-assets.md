# Theme, Navigation, and Assets

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

## Tailwind Direction

Tailwind is a good fit for building the shared block system.

Recommended approach:

1. blocks use Tailwind in the source code
2. the platform compiles a shared CSS bundle
3. each published site gets a small token-driven `theme.css`

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

To improve first-draft conversion, the generation flow may attach starter imagery from Unsplash.

Recommended rules:

- image search happens through a backend integration, not directly from the model
- the model can request image candidates through a tool call
- selected images should store source metadata and attribution fields
- starter imagery should be clearly replaceable by the user

Recommended MVP scope:

- one hero image by default
- optional small gallery for visually driven businesses

Do not treat these images as final brand assets. They are a useful starting point, not a substitute for customer-owned imagery.
