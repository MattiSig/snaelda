# Structure Plan

## Direction

Keep the system simple:

- Go for the backend
- Puck for the builder/editor
- published websites served by Go
- no client-side rendered public sites

The main idea is:

- Puck is the editing experience
- your own site config is the source of truth
- publish turns that config into lightweight website output

## Core Flow

1. User prompts a website
2. Backend creates a canonical draft:
   - site
   - pages
   - blocks
   - theme
   - navigation
3. User edits the draft in a Puck-based builder
4. Frontend maps between canonical draft data and Puck editor data
5. User publishes
6. Backend validates the draft and creates an immutable published snapshot
7. A publish renderer turns that snapshot into page artifacts
8. Go serves those artifacts on `{slug}.snaelda.io`

## Important Rule

Do not make Puck data your database contract.

Keep three layers:

- canonical draft data
- Puck editor data
- published site artifacts

That keeps you free to change the editor later without rewriting the website platform.

## Canonical Draft Contract

The backend should own one canonical draft shape that both generation and the builder use.

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
      { "label": "Home", "pageId": "page_home" },
      { "label": "Contact", "pageId": "page_contact" }
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

This is the contract that:

- prompt generation creates
- the builder edits through an adapter
- publish freezes into a versioned snapshot

The builder should never save raw Puck state directly as the source of truth.

## Prompt Iteration

Prompting is not just the entry point. It is also part of editing.

Users should be able to:

- create the first draft from a prompt
- re-prompt an existing site from inside the builder
- re-prompt a single page from inside the builder
- use prompts for directional changes before fine-tuning manually

The backend should treat re-prompts as updates to canonical draft data, not as a separate throwaway path.

For MVP:

- site-level re-prompt replaces the targeted site draft content
- page-level re-prompt replaces the targeted page draft content
- changes should be undoable through draft revisions

## Suggested Stack

- Go `1.22+`
- `net/http` + `ServeMux`
- `pgx`
- `sqlc`
- `goose`
- Postgres
- Railway Bucket or other S3-compatible object storage for assets and published site artifacts

Keep the backend as a modular monolith at first.

## Model Choice

For MVP generation, use:

- `gpt-5-mini` for the main prompt-to-site generation flow
- structured outputs for the draft schema
- optional smaller models later for tiny helper tasks

Reason:

- good enough quality for a well-defined generation task
- much cheaper than using a flagship model for every free draft
- supports tool/function calling cleanly

## Services

### `api`

Owns:

- auth and authorization
- site/page/block/theme CRUD
- generation
- publishing
- forms
- domain lookup
- external tool integrations such as Unsplash

### `builder`

Owns:

- Puck editor UI
- draft editing experience
- preview inside the editor

This can be a separate frontend app.

### `site`

Owns:

- requests for `*.snaelda.io`
- host lookup
- serving published pages

This should be extremely lightweight.

## How Publishing Works

Publishing is the bridge between Puck and the live website.

At publish time:

1. Load canonical draft data
2. Validate it
3. Freeze it into a published snapshot
4. Render each page into output artifacts
5. Mark that version as live

Those artifacts can be:

- stored HTML per page
- shared site CSS plus per-site theme CSS
- tiny optional JS only where needed

## Publish Job

Treat publish as a real backend job, even if the first version runs inline with the API call.

### Input

- site id
- canonical draft data from Postgres
- theme data
- navigation
- asset references

### Responsibilities

1. load the full draft for the site
2. validate pages, blocks, links, theme, and assets
3. create an immutable `site_versions` snapshot
4. render each page into final output
5. write artifacts to blob storage
6. update `sites.published_version_id`
7. invalidate caches
8. record audit/event logs

### Output

- published site version row
- page artifacts in object storage
- metadata linking the live site to the new version

## Publish Manifest

Each published version should also write a small manifest that tells the `site` service what exists for that version.

Example shape:

```json
{
  "siteId": "site_123",
  "versionId": "ver_7",
  "pages": {
    "/": "sites/site_123/versions/ver_7/pages/index.html",
    "/about": "sites/site_123/versions/ver_7/pages/about/index.html"
  },
  "assets": {
    "themeCss": "sites/site_123/versions/ver_7/assets/theme.css",
    "siteJs": "shared/site.js"
  }
}
```

The `site` service flow is then:

1. resolve host to site
2. find live version
3. load manifest
4. map request path to object key
5. fetch and return the artifact

### Not Its Job

- editing
- Puck state storage
- draft autosave
- auth/session handling
- live request routing

## Artifact Storage

Published page output should live in object storage, not on the app filesystem.

Railway Buckets are a reasonable MVP fit because they are S3-compatible and live inside the Railway project. Keep in mind they are private, so the Go `site` service should fetch or proxy the artifacts rather than exposing the bucket directly.

Example artifact keys:

- `sites/{siteID}/versions/{versionID}/pages/index.html`
- `sites/{siteID}/versions/{versionID}/pages/about/index.html`
- `sites/{siteID}/versions/{versionID}/assets/theme.css`
- `shared/site-core.css`
- `shared/site.js`

## CSS and JS Model

Keep styling and behavior constrained.

### Shared assets owned by us

- block system CSS
- layout and responsive rules
- shared component styling
- tiny shared JS for interactive blocks

Users do not directly control these files.

### Per-site assets generated at publish time

- `theme.css`
- maybe a tiny config snippet if needed later

Users mainly control:

- content
- chosen blocks
- theme tokens

Not:

- arbitrary CSS
- arbitrary JS

## Tailwind Direction

Tailwind is a good fit for the block system, but not as a per-customer runtime compiler.

Recommended use:

1. build blocks with Tailwind utilities
2. compile one shared CSS bundle for the block library
3. generate per-site `theme.css` from prompt-derived theme tokens

That gives you:

- a maintainable block system
- very small published sites
- user control over theme direction without freeform CSS

## Prompt and Theme Generation

The prompt should influence both:

- content and page structure
- visual/theming direction

So generation should extract style intent such as:

- minimal
- editorial
- playful
- luxury
- warm
- nordic

Then map that intent into safe theme tokens and block variants, not raw CSS.

## SEO Across The Lifecycle

SEO should be a first-class requirement, not an afterthought.

### Generation

Generation should produce:

- page titles
- meta descriptions
- page slugs
- heading intent
- alt-text placeholders
- internal linking intent

### Builder

The builder should let users review and edit:

- SEO title
- meta description
- slug
- social/share image later

### Publish

Publish should validate and generate:

- complete page metadata
- canonical URLs
- sitemap data
- `robots.txt`
- `sitemap.xml`

### Live Site

The public site should return:

- full HTML
- proper `<title>`
- meta description
- canonical tags
- Open Graph basics
- fast responses

## Light Analytics

Yes, light analytics is reasonable for MVP.

Keep it simple:

- page views only
- server-side counting in the Go `site` service
- no heavy analytics client

Recommended MVP behavior:

1. site request is served
2. `site` service records a page view event or increments a daily counter
3. builder shows simple totals per site/page

Recommended scope:

- total page views
- views by page
- views by day

Not MVP:

- session replay
- funnels
- heatmaps
- heavy client tracking
- complex attribution

If needed, use very light bot filtering and aggregate counts daily rather than storing more data than necessary.

## Live Request Flow

For a request like `nordic-lens.snaelda.io/about`:

1. Go site server reads the host
2. Resolve the site by slug or domain record
3. Load the current published version
4. Fetch the artifact for `/about` from object storage
5. Return HTML

The live site should not depend on Puck, draft tables, or client-side rendering.

## Builder Choice

Puck is a good fit if:

- you want a solid CMS-style editing experience
- blocks are maintained as reusable UI components
- you accept a small adapter layer between your model and the editor

Puck should be the authoring layer, not the runtime contract for public sites.

## Puck Adapter Rule

The builder should translate between:

- canonical draft data from the API
- Puck editor data in the browser

On save:

- convert Puck changes back into canonical pages, blocks, and theme tokens
- validate on the backend
- persist only canonical draft data

## Block Contract

Each block should have one definition in code that covers three stages:

### 1. Generation

What AI is allowed to create:

- block type
- allowed props
- validation rules
- supported variants
- default values

### 2. Builder Editing

How the block appears in the builder:

- field definitions
- labels
- input types
- image pickers
- select options
- editor preview behavior

### 3. Publish Rendering

How the block turns into the live website:

- render component/template
- prop-to-HTML mapping
- CSS usage
- optional JS requirements

Example shape:

```ts
const heroBlock = {
  type: "hero",
  version: "1.0.0",
  schema: heroSchema,
  defaults: heroDefaults,
  editor: heroEditorConfig,
  render: renderHeroBlock
};
```

The important rule is consistency:

- generation uses the same prop contract
- the builder edits the same prop contract
- publish renders the same prop contract

## Unsplash Integration

Starter imagery is a good conversion aid, but it should be constrained.

Recommended direction:

- use Unsplash as a backend integration
- let the model request image candidates through a tool call
- keep image selection narrow and replaceable

The model should not directly browse for images. The Go backend should expose a tool such as:

- `search_unsplash_images(query, orientation, count)`

Recommended internal module:

- `internal/images/unsplash.go`

It should:

- call the Unsplash API
- return a small set of candidates
- preserve source metadata and attribution fields
- store selected image references in canonical draft data

## Generation Tool Flow

Recommended flow:

1. user prompt arrives
2. `gpt-5-mini` creates the site plan and block draft
3. model decides whether starter imagery is needed
4. model calls the Unsplash search tool
5. Go backend calls Unsplash and returns image candidates
6. model selects image candidates for the draft
7. backend validates and persists final draft data

Keep MVP image scope narrow:

- one hero image by default
- optional small gallery for clearly image-heavy sites
- user can replace or remove all suggested images easily

## Publishing Options

### Best fit

Pre-render page output on publish.

Why:

- fastest public sites
- best SEO
- simplest runtime
- easy rollback

### Alternative

Render from snapshot on request and cache heavily.

This is simpler to start, but less ideal if performance is the top priority.

## Railway Shape

Keep it compact:

- `api` service
- `builder` service
- `site` service
- Postgres

Domain model:

- `app.snaelda.io` -> builder
- `{slug}.snaelda.io` -> site service

Later, custom domains can point to the same `site` service.

## Recommendation

The current best direction is:

- Go backend
- Puck-powered builder
- canonical site config in Postgres
- publish-time rendering to lightweight artifacts
- Go server for all public websites

That gives you a strong editing experience without making the public websites heavy.
