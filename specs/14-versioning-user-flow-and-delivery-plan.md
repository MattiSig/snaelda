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

The default entry point is the homepage prompt. No signup is required. The session model is defined in [Spec 17](./17-guest-authoring-and-claim.md).

1. Visitor enters a prompt on the homepage
2. Backend resolves the caller into one of three cases:
   - **No cookie:** mint a trial session, create a new workspace, run generation, return the first site
   - **Existing trial cookie:** reuse the session and workspace, run generation, create a new `sites` row in that workspace
   - **Authenticated user:** use the user's workspace, run generation, create a new `sites` row in that workspace
3. Generation engine creates a valid site draft
4. User lands in the editor with a preview

Authenticated users who start from `Create website` inside the builder follow case three above.

### Log In

Login is for users who have already attached an email to their workspace (Spec 17, L2 or paid).

1. User clicks `Log in` on the landing page
2. User enters their email
3. Backend creates a magic-link token, hashes it, and emails the plaintext as a one-time URL
4. User opens the link, backend validates the token, sets a session cookie, and lands the user in their most recently active workspace

If the email is unknown, the response is identical to the success case but no mail is sent.

### Restore Workspace

For users on L1 who have a workspace recovery link.

1. User pastes the recovery URL on the landing page or opens it directly
2. Backend looks up the session by hashed recovery key
3. Backend sets a fresh cookie bound to that session and lands the user in the builder

### Save Your Workspace

Trial users can secure their workspace without paying. Optional at any time.

1. User clicks `Save your workspace` in the builder chrome
2. User picks `Copy workspace link`, `Add an email`, or both
3. If `Copy workspace link`: backend mints and displays a recovery URL once; the hash is stored on the session
4. If `Add an email`: backend creates a `users` row, attaches it as workspace owner, sends a verify magic link, and from then on the user can also log in via magic link from any browser. Adding an email invalidates any existing recovery link.

### Subscribe

Subscription lifts the 4-day trial and 25-prompt caps and unlocks custom domains.

1. User triggers a paid action (custom domain, prompt past 25, or any write after the 4-day trial expires)
2. Builder opens Stripe Checkout for the workspace
3. Stripe collects email if no email is yet attached to the workspace
4. Webhook fires; backend writes `billing_subscriptions`, refreshes `billing_entitlements`, and (if needed) creates the `users` row from the Checkout email and claims the workspace
5. User returns to the builder; previously blocked actions now succeed

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

- workspaces
- trial sessions, recovery links, email attach, and magic-link login per [Spec 17](./17-guest-authoring-and-claim.md)
- 4-day trial window and 25-prompt cap enforcement
- site CRUD
- page CRUD
- block instance CRUD
- theme table
- block registry with 4 to 5 blocks
- preview renderer

### Phase 2: Generation

- prompt intake from both trial and authenticated sessions
- trial budget and trial-window enforcement on prompt routes
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
