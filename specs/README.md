# Prompt-to-Website Platform Specs

This folder breaks the initial product and architecture spec into focused documents that are easier to review, implement, and update independently.

## Overview

The platform creates and hosts simple websites from prompts. The core architectural constraint is that each website is represented as structured data, not arbitrary generated code. AI produces validated configuration that references a maintained registry of blocks, and the renderer turns that configuration into preview and published websites.

## Table of Contents

1. [Product Summary and MVP Scope](./01-product-summary-and-scope.md)
2. [System Architecture](./02-system-architecture.md)
3. [Domain Model](./03-domain-model.md)
4. [Block Registry](./04-block-registry.md)
5. [Site Configuration Model](./05-site-configuration-model.md)
6. [Database Design](./06-database-design.md)
7. [Generation Engine](./07-generation-engine.md)
8. [Editor and Authoring Model](./08-editor-and-authoring.md)
9. [Preview, Publish, and Rendering](./09-preview-publish-and-rendering.md)
10. [API Surface](./10-api-surface.md)
11. [Theme, Navigation, and Assets](./11-theme-navigation-and-assets.md)
12. [Security, Validation, and Caching](./12-security-validation-and-caching.md)
13. [Deployment, Domains, and Hosting](./13-deployment-domains-and-hosting.md)
14. [Versioning, User Flow, and Delivery Plan](./14-versioning-user-flow-and-delivery-plan.md)
15. [Billing and Stripe](./15-billing-and-stripe.md)

## Recommended Reading Order

If someone is new to the project, read the docs in this order:

1. Product Summary and MVP Scope
2. System Architecture
3. Domain Model
4. Site Configuration Model
5. Database Design
6. Generation Engine
7. Preview, Publish, and Rendering

The remaining docs then cover implementation details for editing, APIs, assets, security, deployment, and the delivery plan.

## Core Principle

The most important rule across all specs is:

> The website is data, not code.

That means:

- Drafts are stored as validated structured entities.
- Published versions are immutable snapshots.
- Rendering is done by maintained application components.
- AI is constrained to known blocks, schemas, and theme tokens.

## Spec Boundaries

These documents describe the first product spec for the MVP. They intentionally avoid broad website-builder features such as arbitrary code injection, freeform layout systems, custom CSS/JS, and per-site generated frontend apps.
