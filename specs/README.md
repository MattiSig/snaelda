# Specs

This folder breaks the platform spec into focused documents you can review, implement, and update independently. Every spec owns one concern; cross-references are explicit so each doc reads cleanly on its own.

## North Star

> The website is data, not code.

- Drafts are stored as validated structured entities.
- Published versions are immutable snapshots.
- Rendering is done by maintained application components.
- AI is constrained to known blocks, schemas, and theme tokens.

Everything else is downstream of this rule.

## How the specs are organized

The spec set is grouped into five clusters of related work. Reading any cluster top-to-bottom should be enough to understand that part of the system without jumping between docs.

### 1. Product Foundation

What Snaelda is, who it's for, how the system is shaped, and the order capabilities ship in.

| #  | Spec | What it owns |
|----|------|--------------|
| 01 | [Product Summary and MVP Scope](./01-product-summary-and-scope.md) | The product brief: target users, the in-scope MVP capabilities, and the explicit non-goals. |
| 02 | [System Architecture](./02-system-architecture.md) | Component boundaries (Go API, TanStack web, Postgres, SeaweedFS) and how they fit together at runtime. |
| 14 | [Versioning, User Flow, and Delivery Plan](./14-versioning-user-flow-and-delivery-plan.md) | The release milestones and the order in which user-facing capabilities reach customers. |

### 2. Data Model

The website-as-data layer: entities, the block registry that generation and rendering both honor, the validated site config shape, and the Postgres schema underneath.

| #  | Spec | What it owns |
|----|------|--------------|
| 03 | [Domain Model](./03-domain-model.md) | Core entities — workspaces, sites, pages, blocks, collections, entries, versions, themes — and the relationships between them. |
| 04 | [Block Registry](./04-block-registry.md) | The closed, typed set of blocks the platform supports, with prop schemas and the contract shared by generation and the renderer. |
| 05 | [Site Configuration Model](./05-site-configuration-model.md) | The on-disk and in-memory shape of a site as structured data, never as generated code. |
| 06 | [Database Design](./06-database-design.md) | The Postgres schema for every table the platform owns and how it lines up with the domain model. |
| 19 | [Collections and Content Types](./19-collections-and-content-types.md) | Site-scoped typed collections, the field-type registry, collection-bound page templates, block bindings, and the AI maintenance actions that produce entries. |

### 3. Authoring Loop

Prompt → draft → edit → preview → publish. Everything the user touches inside the builder lives in this cluster.

| #  | Spec | What it owns |
|----|------|--------------|
| 07 | [Generation Engine](./07-generation-engine.md) | How a prompt becomes a validated site draft via constrained, structured AI output. |
| 08 | [Editor and Authoring Model](./08-editor-and-authoring.md) | The builder's rules for editing blocks, pages, themes, and navigation in place. |
| 09 | [Preview, Publish, and Rendering](./09-preview-publish-and-rendering.md) | Draft → preview-token → immutable published version, plus the renderer pipeline. |
| 11 | [Theme, Navigation, and Assets](./11-theme-navigation-and-assets.md) | Theme tokens, navigation as first-class data, and the asset library that feeds the renderer. |

### 4. Platform & Runtime

The surfaces the user does not see directly: API routes, security posture, hosting and domains, and how published sites behave at runtime.

| #  | Spec | What it owns |
|----|------|--------------|
| 10 | [API Surface](./10-api-surface.md) | The full HTTP route list across builder, public surface, and platform admin. |
| 12 | [Security, Validation, and Caching](./12-security-validation-and-caching.md) | Auth, CSRF, input validation, durable rate limiting, response headers, and cache semantics. |
| 13 | [Deployment, Domains, and Hosting](./13-deployment-domains-and-hosting.md) | Hosted subdomain delivery and the custom-domain attach/verify/TLS lifecycle. |
| 16 | [Runtime Lifecycles and Analytics](./16-runtime-lifecycles-and-analytics.md) | Public visibility rules, domain/runtime semantics, and the MVP analytics scope. |

### 5. Commerce, Identity, and Comms

Money in, people identified, mail out. This cluster is where the product becomes a business.

| #  | Spec | What it owns |
|----|------|--------------|
| 15 | [Billing and Stripe](./15-billing-and-stripe.md) | Subscription model, one-time add-ons, entitlement mapping, and Stripe webhook wiring. |
| 17 | [Trial Sessions, Recovery, and Subscription](./17-guest-authoring-and-claim.md) | Cookie-bound trials, recovery layers (workspace link or email), and the path from trial to paying account. |
| 18 | [Transactional Email](./18-transactional-email.md) | The `Mailer` interface, Resend in prod, Mailpit locally, the template set, and per-purpose rate limits. |

## Recommended Reading Order

If you are new to the project, start with the foundation and data clusters before anything else:

1. [Product Summary and MVP Scope](./01-product-summary-and-scope.md)
2. [System Architecture](./02-system-architecture.md)
3. [Domain Model](./03-domain-model.md)
4. [Site Configuration Model](./05-site-configuration-model.md)
5. [Collections and Content Types](./19-collections-and-content-types.md)
6. [Database Design](./06-database-design.md)
7. [Generation Engine](./07-generation-engine.md)
8. [Preview, Publish, and Rendering](./09-preview-publish-and-rendering.md)

The remaining specs cover editing, APIs, theming, security, deployment, runtime, billing, identity, and email.

## Operational Docs

Specs describe the product; the docs below describe how to operate it.

- [Once-over from the Maker](../docs/once-over-workflow.md) — the operator playbook for the `$99` async site review add-on.
- [Decisions](../docs/decisions/) — architecture decision records (ADRs).

## Spec Boundaries

These documents describe the MVP. They intentionally avoid broad website-builder features such as arbitrary code injection, freeform layout systems, custom CSS/JS, marketplace blocks, and per-site generated frontend apps. New scope earns a new spec; we do not retrofit existing specs to cover capabilities they weren't written for.
