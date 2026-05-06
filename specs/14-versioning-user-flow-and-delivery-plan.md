# Versioning, User Flow, and Delivery Plan

## Versioning and Rollback

Publishing creates immutable snapshots.

Rollback flow:

1. User selects an older version
2. Backend sets `sites.published_version_id` to that version
3. Cache invalidates
4. Public site serves the older snapshot

Possible later addition:

- restore a published version back into draft for further editing

For draft editing, maintain lightweight undo support for destructive regeneration actions such as site-level and page-level re-prompts.

## MVP User Flows

### Create Site

1. User clicks `Create website`
2. User enters a prompt
3. Backend creates a generation job
4. Generation engine creates a valid site draft
5. User lands in the editor with a preview

### Edit Site

1. User clicks a block
2. Side panel shows block-specific fields
3. User edits fields
4. Backend validates and saves props
5. Preview updates

### Re-Prompt Site Or Page

1. User enters a new prompt for the whole site or a single page
2. Backend captures the current draft as an undoable revision
3. Generation runs against the targeted scope
4. New draft content replaces the targeted scope
5. User can undo if the result is worse than before

### Publish Site

1. User clicks `Publish`
2. Backend validates the draft
3. Backend creates a version snapshot
4. Backend generates page artifacts for the live site
5. Site becomes live on its subdomain
6. User receives the public URL

## Key Engineering Decisions

### Decision 1: Config-Driven Publishing

Use one publishing system and many configs, not generated apps per site.

Reasons:

- safer
- easier to maintain
- cheaper to host
- easier to version
- faster to ship

### Decision 2: Block Registry in Code

Keep block schemas and renderers in code.

Reasons:

- rendering needs deployed components
- schema and component stay synchronized
- migration logic can be version-controlled

### Decision 3: Draft Normalized, Publish Snapshot + Artifacts

Edit normalized draft data and publish immutable snapshots plus lightweight page artifacts.

Reasons:

- editing is simpler
- public serving is more stable
- rollback is easier
- published output is isolated from later draft changes

### Decision 4: Theme Tokens, Not Free CSS

Use constrained structured theme data.

Reasons:

- safer AI output
- more consistent design quality
- easier editor controls
- more reliable block rendering

### Decision 5: SEO As Publish Contract

Treat SEO as part of generation, editing, validation, and publish output.

Reasons:

- public pages need strong defaults
- publish is the safest place to enforce metadata quality
- sitemap and robots generation belong with released artifacts

### Decision 6: Light Analytics Only

Start with simple server-side page view counts.

Reasons:

- enough value for MVP
- low runtime cost
- avoids heavy tracking code on public sites

### Decision 7: Single Backend, Modular Internals

Start as a modular monolith.

Reasons:

- simpler deployment
- faster development
- easier transaction boundaries
- enough for the MVP

## Minimum Build Plan

### Phase 1: Foundation

- auth and workspaces
- site CRUD
- page CRUD
- block instance CRUD
- theme table
- block registry with 4 to 5 blocks
- preview renderer

### Phase 2: Generation

- prompt intake
- structured generation output
- validation and repair
- save generated draft
- add remaining MVP blocks

### Phase 3: Builder

- page list
- block list
- Puck-based block editor
- reorder blocks
- theme editor
- navigation editor

### Phase 4: Publish and Hosting

- snapshot builder
- artifact generation
- site versions
- subdomain resolution
- public site service
- cache invalidation

### Phase 5: Forms and Assets

- asset upload
- asset picker
- contact form submissions
- email notifications
- spam and rate limiting

## Important Early Non-Goals

Avoid building these in the first version:

- arbitrary layout nesting
- freeform positioning
- custom CSS editor
- custom JS
- complex responsive controls
- reusable symbols/components
- CMS collections
- per-site frontend compilation

## Open Questions

1. Should the first release support only one-page sites before expanding to 10 pages?
2. Should AI generate images, or should MVP use uploaded and placeholder assets only?
3. Should contact form submissions be stored, emailed, or both?
4. Which Stripe-backed billing model is required on day one: free beta, paid subscriptions, metered usage, or manual invoicing?
5. Should the product support multiple languages later?
6. Should published pages be dynamically server-rendered or statically cached after publish?
7. Which industries should prompt presets target first?
8. Should there be a `regenerate this block` action?
9. Should generated copy be editable inline in preview or only in a side panel?
10. What is the minimum acceptable public URL/domain experience for launch?

## Architecture Summary

Build a modular monolith where AI generates a validated website configuration made of versioned block instances, Postgres stores draft entities and immutable published snapshots, a builder handles authoring, and publishing produces lightweight site artifacts that are served on subdomains.
