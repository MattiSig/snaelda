# Product Summary and MVP Scope

## Product Summary

The product is a website creation and hosting platform for simple web presences such as:

- landing pages
- name-card pages
- small company websites
- portfolios
- event pages
- product pages

Users start with a prompt, for example:

> Create a modern landing page for my local photography studio with a gallery, pricing, contact form, and booking CTA.

The system converts that prompt into a structured website configuration composed of reusable building blocks. Users can then preview the generated draft, edit block content and theme settings, manage pages and navigation, and publish the website.

## Architecture Constraint

The MVP does not generate arbitrary custom code per website. Instead, it generates validated configuration that references a known block registry. The renderer consumes that configuration and produces the final site.

## MVP Should Support

- Guest authoring from the homepage prompt without signup, with a small free-prompt budget and a claim-on-signup flow, per [Spec 17](./17-guest-authoring-and-claim.md)
- User account and workspace/team model
- Website creation from prompt
- Up to 10 pages per website
- Block-based page composition
- Approximately 12 core block types
- Site-wide theme system
- Draft editing
- Preview mode
- Publish mode
- Hosted subdomain such as `site-name.platform.com`
- Asset upload and image library
- Basic SEO fields
- Contact form block with stored submissions and/or email forwarding
- Stripe-backed billing for platform subscriptions, usage limits, and payment collection if paid access is required at launch

Basic custom domain mapping is planned, but it does not need to be fully built in the first MVP.

## MVP Should Not Support Yet

- Arbitrary user code injection
- Full drag-and-drop layout freedom
- Marketplace of third-party blocks
- E-commerce checkout inside generated customer websites
- Complex CMS collections
- Multi-language sites
- Advanced permissions
- Full visual design editor comparable to Webflow
- Per-customer generated frontend apps

## Product Goals

The MVP should optimize for:

- safe generation
- fast editing
- stable rendering
- maintainable components
- simple deployment
- easy publishing and rollback

## Non-Goal

This is not a general-purpose website builder in the first version. The product should remain narrow, fast, and difficult to break.
