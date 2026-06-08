# Editor and Authoring Model

## Builder Goal

The builder should feel like a compact CMS-style page editor. A tool like Puck is a good fit for the editing experience, but the system should still store its own canonical draft model.

## Core Builder Capabilities

Users should be able to:

- create a site from a prompt
- inspect generated draft pages
- re-prompt the site from inside the builder
- re-prompt a single page from inside the builder
- click blocks to edit their fields
- reorder blocks
- hide or show blocks
- duplicate blocks
- delete blocks
- add approved blocks
- edit theme settings
- manage pages and navigation
- manage collections, schemas, and entries
- preview drafts
- publish the site

## Data Model Rule

Do not make the editor's internal JSON shape the source of truth.

Use an adapter layer:

- canonical draft data from the backend
- mapped into builder data for editing
- mapped back into canonical draft data on save

The builder should not treat raw Puck state as the stored website model.

## Block Editing

For each block, the builder should render fields from the block editor schema.

Example hero editor fields:

- eyebrow
- headline
- subheadline
- primary CTA label
- primary CTA link
- image
- layout variant

## Reordering and Block Operations

Allow users to:

- reorder blocks within a page
- hide/show a block
- duplicate a block
- delete a block
- add a block from the approved registry

## Page Management

Allow users to:

- add a page unless the site already has 10 editor-visible pages (templates count; URLs produced by templates do not)
- choose the page type at creation time: `static`, `collection_index`, or `collection_detail`
- bind a collection-typed page to the right collection
- rename a page
- edit the slug (where the page type uses one)
- set SEO title and description (entry-level SEO lives on the entry, not on the template; see [Spec 19](./19-collections-and-content-types.md))
- include or exclude it from navigation
- reorder navigation
- delete the page

For `collection_detail` templates, the page editor exposes block field-binding controls per block prop: pick a source entry field (typed to match the prop) or leave the literal prop value in place.

Programmatic-SEO patterns (e.g. one URL per `service × city`) are not a page-type concern — they are handled by AI-generating one variant entry per combination into the same collection; the same `collection_detail` template renders each. See [Spec 19](./19-collections-and-content-types.md).

## Collection Management

Allow users to:

- create a new collection (slug, singular/plural labels, initial schema)
- edit the collection schema: add, remove, reorder, rename, and retype fields drawn from the closed field-type registry in [Spec 19](./19-collections-and-content-types.md)
- acknowledge a migration prompt when a schema change is destructive (rename, type change, remove required field) — entries are migrated forward; publish fails until the migration is run
- create, edit, duplicate, delete, and reorder entries within a collection
- set per-entry SEO and status (`draft`, `published`)
- generate entries with AI ("turn these photos into a project", "add a service", "generate location variants for {entry} in {cities}", "rewrite this entry") — see [Spec 07](./07-generation-engine.md)

Editing constraints:

- entries cannot be saved if they fail schema validation
- a collection cannot be deleted while any page binds to it; the editor surfaces the offending pages
- enum / enum_multi options are edited at the collection level, not per entry

## Theme Editing

Allow controlled editing of:

- color palette
- font style preset
- typography scale
- button style
- corner radius
- section spacing
- content width
- image style

Do not expose raw CSS in the MVP.

Theme controls update the builder preview immediately. Saving persists the
current selection to the canonical draft; resetting restores the last saved
theme. Presets should offer materially distinct directions, including square
corners, graduated rounding, mixed serif and sans pairings, full-serif
typography, and multiple sans voices.

## Prompt-Driven Iteration

Users should be able to throw a new prompt at an existing draft when they want directional changes.

Examples:

- make it warmer
- add pricing
- reduce the number of sections
- make it feel more premium

This should update canonical draft data through the backend rather than bypassing the draft model.

## Prompt Scope

For MVP, support three prompt scopes:

- site-level prompt
- page-level prompt
- entry-level prompt (re-prompts the fields of a single collection entry)

Rules:

- site-level prompt replaces the generated content for the site draft scope it targets
- page-level prompt replaces the generated content for the targeted page (and, for `collection_detail` templates, may rewrite template content but not entry content)
- entry-level prompt replaces the targeted entry's fields, scoped to that entry only
- the result should be undoable
- block-level prompting can wait until later

Users should understand that a re-prompt is a replacement action for that scope, not a vague merge.

## Editing Constraint

The builder should never require end users to understand renderer internals or manually author configuration schemas.
