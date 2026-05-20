# Generation Engine

## Responsibility

The generation engine converts prompt input into a validated site draft plan. It must generate structured data, not arbitrary HTML.

Recommended MVP model choice:

- use `gpt-5-mini` for the main generation flow
- use structured outputs against the canonical draft schema
- add retries or repair only after backend validation

## Minimum Input

```json
{
  "prompt": "Create a clean website for a Stockholm photography studio...",
  "workspaceId": "...",
  "siteName": null,
  "preferredLanguage": "en",
  "brand": {
    "businessName": null,
    "logo": null,
    "primaryColor": null
  },
  "optionalHints": {
    "industry": null,
    "style": null,
    "pages": null
  }
}
```

`brand` is a first-class input, not a hint. When any of `businessName`, `logo`, or `primaryColor` is provided up front, generation must use it verbatim and must not invent alternatives. When fields are absent, generation derives them from the prompt (`businessName` from extracted intent, `primaryColor` from style cues, `logo` left null pending user upload).

## Target Output

The AI should output structured draft data:

```json
{
  "siteName": "Nordic Lens Studio",
  "siteSlug": "nordic-lens-studio",
  "siteGoal": "Generate photography leads",
  "brand": {
    "businessName": "Nordic Lens Studio",
    "logo": null,
    "primaryColor": "#4B6F60"
  },
  "theme": {},
  "collections": [],
  "pages": [],
  "navigation": {},
  "assetsNeeded": [],
  "assumptions": []
}
```

`collections` is a list of `{ slug, singularLabel, pluralLabel, schema, entries }` objects matching the model in [Spec 19](./19-collections-and-content-types.md). The planner decides whether the prompted business needs any collections; trades and portfolio businesses typically get `services` and `projects`, hospitality typically gets `properties` (often single-entry), restaurants typically get `menu_items`. The model is not constrained to those names — it chooses from the prompt and must validate against the field-type registry.

## High-Level Pipeline

1. Receive user prompt
2. Extract intent
3. Decide which collections (if any) the business needs and design their schemas
4. Seed initial entries for each collection (including any AI-determined location variants as their own full entries — see [Spec 19](./19-collections-and-content-types.md))
5. Create a page plan, capped at 10 editor-visible pages, that includes static pages plus any collection-bound templates (`collection_index`, `collection_detail`)
6. Choose allowed blocks per page; for collection templates, set up bindings from block props to entry fields
7. Generate copy and placeholder content
8. Generate theme tokens
9. Fetch starter image candidates when needed (including for entry `asset` / `asset_list` fields)
10. Validate against schemas (including each entry against its collection schema)
11. Persist as draft site data
12. Return draft data for the builder preview

## Suggested AI Stages

### Step 1: Intent Extraction

Extract:

- site type
- business name
- target audience
- services or products
- geography
- desired tone
- desired visual style
- primary CTA
- required pages
- missing information

### Step 2: Collection Plan

Decide which collections (if any) the business needs, and design each schema from the closed field-type registry in [Spec 19](./19-collections-and-content-types.md). Keep schemas tight — three to seven fields per collection covers most real cases. Enum options must be enumerated up front.

Then seed initial entries for each collection. Entry count should be informed by the prompt: a scaffolding company probably needs 3–5 service entries, a portfolio shop 6–10 project entries, a single-cabin rental 1 property entry.

### Step 3: Site Plan

Generate:

- pages (static and collection-bound)
- page goals
- navigation labels
- required blocks per page
- for `collection_detail` templates: which block props bind to which entry fields

### Step 4: Content Draft

Generate block props such as:

- headlines
- body copy
- FAQs
- feature items
- CTAs
- placeholder testimonials when source content is missing

For collection-bound templates, content lives in two places: literal prop values (used when no entry binds the prop) and entry field values (used when a binding is present). Generate both: the template should still preview sensibly in the editor when no entry is selected.

### Step 5: Theme Draft

Generate:

- palette
- typography style
- spacing
- shape
- mood metadata

The palette is **derived from `brand.primaryColor`** rather than freely chosen — the primary token equals brand color, and secondary/accent/surface tokens are produced deterministically from it (see [Spec 11](./11-theme-navigation-and-assets.md)). The prompt influences typography, spacing, shape, and mood; it does not override the brand color.

### Step 6: Starter Images

For MVP, image generation is not required.

Instead, use a backend Unsplash integration for starter imagery:

- model determines whether a page/block needs an image
- model creates a narrow search query
- backend tool searches Unsplash
- model selects from returned candidates

Recommended default behavior:

- one hero image for most sites
- optional small gallery for image-heavy businesses
- all suggested images remain easy to replace or remove

### Step 7: Validation and Repair

Validate against all relevant schemas. If invalid:

- ask the model to repair JSON
- or run deterministic repair logic where safe
- reject unsupported blocks
- trim plans to platform limits

### Step 8: Persist Draft

Create:

- site
- theme
- collections and entries
- pages (with `type` and `collection_id` populated for collection-bound templates)
- block instances (with `bindings` populated where the page is a collection template)
- navigation settings

## Generation Guardrails

Generation must obey:

- maximum 10 editor-visible pages (templates plus static pages)
- only known block types
- only supported block versions
- valid props per block schema
- block bindings only on `collection_detail` pages, and only to field types that match the bound prop
- collection schemas use only field types from the registry in [Spec 19](./19-collections-and-content-types.md)
- enum / enum_multi options are enumerated up front, never invented at entry-write time
- every seeded entry validates against its collection's schema
- safe links
- no scripts
- no unsupported embed code
- no unsanitized HTML
- contact forms with supported field types only
- valid theme token structure
- image suggestions should come from approved integrations only

## AI Maintenance Actions

Generation runs at site creation. The same generation infrastructure powers a smaller set of in-builder maintenance actions that produce or rewrite collection entries and collection-bound pages:

- "Turn these photos into a project" — given a selected asset set and a one-line prompt, create a draft entry in a `projects`-shaped collection (or whichever collection the user picks); AI fills field values from the prompt and image context.
- "Add a service" — create a draft entry in the chosen collection from a short description; AI completes the remaining fields.
- "Generate location variants for {entry} in {cities}" — fan out into N draft entries in the same collection, each with full per-city copy (slug suffixed with the city). This is the programmatic-SEO pattern; see [Spec 19](./19-collections-and-content-types.md) for why variants are full entries rather than a structural axis on the template.
- "Generate FAQ from services" — create or extend an `faqs`-shaped collection by reading existing `services` entries.
- "Rewrite this entry" — re-prompt a single entry's fields, scoped to that entry only.

These actions count against the per-workspace prompt budget the same way site- and page-level re-prompts do (see [Spec 17](./17-guest-authoring-and-claim.md)). Failure modes use the same validation/repair pipeline as initial generation.

## Tool Calling

Use model tool/function calling for external lookups needed during generation.

Recommended MVP tool:

- `search_unsplash_images(query, orientation, count)`

The model should never call third-party APIs directly. Your backend owns the tool implementation and returns constrained results back to the model.
