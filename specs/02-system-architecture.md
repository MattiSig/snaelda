# System Architecture

## Core Architecture Principle

The website is data, not code.

Each website is represented by structured application data:

1. site metadata
2. theme tokens
3. pages (static and collection-bound templates)
4. navigation
5. block instances (with optional bindings to collection entry fields)
6. block content and config props
7. assets
8. collections and entries (typed lists the owner defines per site)
9. draft and published versions

The backend owns this model in Postgres. The editor works from draft data, and publishing turns that draft into lightweight public site output.

## Why This Architecture

This model gives the product:

- Safe generation because AI can only choose known blocks and valid props
- Easy editing because users edit fields, not code
- Stable rendering because every site uses maintained components
- Versioning because draft and published states are separated
- Scale because one renderer can serve many websites
- Maintainability because new block versions can be added without breaking older published sites

## High-Level System Components

### Backend API

A single backend service owns:

- authentication and session integration
- workspaces and site ownership
- website generation from prompt
- site config validation
- CRUD for pages, blocks, themes, navigation, and assets
- draft and publish workflow
- preview token generation
- public site lookup by subdomain or domain
- contact form submission handling
- versioning and rollback

Internal modules:

- `auth` — sessions, magic links, CSRF, user store.
- `sites` — the authoring root. Owns sites, pages, blocks, navigation, draft revisions, and preview tokens. Pages and blocks are nested resources under a site rather than standalone modules, because they have no lifecycle outside their parent site.
- `collections` — site-scoped typed collections, entries, schema validation, and the field-binding contract that lets page templates render from entries. See [Spec 19](./19-collections-and-content-types.md).
- `themes` — theme tokens and regeneration.
- `generation` — prompt-to-site planner, site/page reprompt, starter imagery, and AI generation of collections and entries.
- `publishing` — snapshot validation, artifact production, public render, rollback, versions.
- `analytics` — page-view aggregation and the site analytics endpoint.
- `domains` — hosted-domain state and custom-domain attachment.
- `assets` — uploads, library, public asset delivery.
- `forms` — public form submission, spam handling, builder-side submission management.
- `imagery` — Pexels client used by generation for starter imagery.
- `billing` — Stripe Checkout, Customer Portal, webhooks, workspace entitlements, and gating for paid actions (generation, publish, custom domains, asset uploads). Required for MVP; see [Spec 15](./15-billing-and-stripe.md).

Workspaces exist in the database as the tenancy boundary that every authoring resource is scoped to, but there is no `workspaces` module in MVP — a default workspace is provisioned during login and authorization derives the workspace from the session. A dedicated module is only introduced once multi-workspace UX (switching, invites) is actually shipped. Billing entitlements still attach to the workspace row directly.

This is a modular monolith.

### Postgres Database

Postgres is the canonical source of truth.

Use relational tables for stable core entities:

- users
- workspaces
- sites
- pages
- collections
- collection_entries
- assets
- domains
- site_versions
- form_submissions

Use `jsonb` for:

- flexible block props
- theme tokens
- generation metadata
- immutable published snapshots

### Builder / Editor

The builder lets users:

- create a site from a prompt
- inspect generated draft pages
- edit block content
- reorder blocks
- add or remove blocks
- edit theme values
- manage pages and navigation
- preview the draft
- publish the site (authenticated only)

The same builder serves both authenticated users and unauthenticated visitors using a browser-bound guest session, with publish and other paid actions gated to authenticated accounts. See [Spec 17](./17-guest-authoring-and-claim.md) for the guest identity, quota, and claim model.

The builder can use a CMS-style tool such as Puck, but Puck should be treated as the editing layer rather than the canonical website model.

### Public Renderer

The public website layer serves published websites and should:

- resolve hostnames to a site
- load the latest published version
- serve lightweight output for each page
- avoid client-side rendering
- cache aggressively

Recommended direction:

- keep the builder separate from the public website service
- use subdomain-based routing such as `{site-slug}.platform.com`
- publish should generate artifacts that the public service can return quickly

### Generation Engine

The generation engine converts prompt input into validated site configuration. It must not write arbitrary HTML. It should produce structured draft data that passes validation before it is persisted.
