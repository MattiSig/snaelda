# System Architecture

## Core Architecture Principle

The website is data, not code.

Each website is represented by structured application data:

1. site metadata
2. theme tokens
3. pages
4. navigation
5. block instances
6. block content and config props
7. assets
8. draft and published versions

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

Recommended internal modules:

- `auth`
- `workspaces`
- `sites`
- `pages`
- `blocks`
- `themes`
- `assets`
- `generation`
- `publishing`
- `domains`
- `forms`
- `billing` with Stripe after the core create/edit/publish loop is stable

This should start as a modular monolith.

### Postgres Database

Postgres is the canonical source of truth.

Use relational tables for stable core entities:

- users
- workspaces
- sites
- pages
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

The authenticated builder lets users:

- create a site from a prompt
- inspect generated draft pages
- edit block content
- reorder blocks
- add or remove blocks
- edit theme values
- manage pages and navigation
- preview the draft
- publish the site

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
