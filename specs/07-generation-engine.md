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
  "optionalHints": {
    "industry": null,
    "style": null,
    "pages": null
  }
}
```

## Target Output

The AI should output structured draft data:

```json
{
  "siteName": "Nordic Lens Studio",
  "siteSlug": "nordic-lens-studio",
  "siteGoal": "Generate photography leads",
  "theme": {},
  "pages": [],
  "navigation": {},
  "assetsNeeded": [],
  "assumptions": []
}
```

## High-Level Pipeline

1. Receive user prompt
2. Extract intent
3. Pick a site archetype
4. Create a page plan, capped at 10 pages
5. Choose allowed blocks per page
6. Generate copy and placeholder content
7. Generate theme tokens
8. Fetch starter image candidates when needed
9. Validate against schemas
10. Persist as draft site data
11. Return draft data for the builder preview

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

### Step 2: Site Plan

Generate:

- pages
- page goals
- navigation labels
- required blocks per page

### Step 3: Content Draft

Generate block props such as:

- headlines
- body copy
- FAQs
- feature items
- CTAs
- placeholder testimonials when source content is missing

### Step 4: Theme Draft

Generate:

- palette
- typography style
- spacing
- shape
- mood metadata

The prompt should influence both content structure and visual direction. The model should extract style intent and map it into safe theme tokens and supported block variants rather than raw CSS.

### Step 5: Starter Images

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

### Step 6: Validation and Repair

Validate against all relevant schemas. If invalid:

- ask the model to repair JSON
- or run deterministic repair logic where safe
- reject unsupported blocks
- trim plans to platform limits

### Step 7: Persist Draft

Create:

- site
- theme
- pages
- block instances
- navigation settings

## Generation Guardrails

Generation must obey:

- maximum 10 pages
- only known block types
- only supported block versions
- valid props per block schema
- safe links
- no scripts
- no unsupported embed code
- no unsanitized HTML
- contact forms with supported field types only
- valid theme token structure
- image suggestions should come from approved integrations only

## Tool Calling

Use model tool/function calling for external lookups needed during generation.

Recommended MVP tool:

- `search_unsplash_images(query, orientation, count)`

The model should never call third-party APIs directly. Your backend owns the tool implementation and returns constrained results back to the model.
