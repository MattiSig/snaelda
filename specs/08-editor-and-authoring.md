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

- add a page unless the site already has 10 pages
- rename a page
- edit the slug
- set SEO title and description
- include or exclude it from navigation
- reorder navigation
- delete the page

## Theme Editing

Allow controlled editing of:

- color palette
- font style preset
- button style
- corner radius
- section spacing
- image style

Do not expose raw CSS in the MVP.

## Prompt-Driven Iteration

Users should be able to throw a new prompt at an existing draft when they want directional changes.

Examples:

- make it warmer
- add pricing
- reduce the number of sections
- make it feel more premium

This should update canonical draft data through the backend rather than bypassing the draft model.

## Prompt Scope

For MVP, support two prompt scopes:

- site-level prompt
- page-level prompt

Rules:

- site-level prompt replaces the generated content for the site draft scope it targets
- page-level prompt replaces the generated content for the targeted page
- the result should be undoable
- block-level prompting can wait until later

Users should understand that a re-prompt is a replacement action for that scope, not a vague merge.

## Editing Constraint

The builder should never require end users to understand renderer internals or manually author configuration schemas.
