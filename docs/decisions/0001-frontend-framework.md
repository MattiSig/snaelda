# Decision 0001: Frontend Framework

## Status

Accepted

## Decision

Use TanStack Start for the authenticated React builder and preview experience.
Use Tailwind CSS as the styling baseline and shadcn/ui as the default source-owned component system for builder UI primitives.

## Context

The product needs a React application for prompt entry, authenticated builder workflows, draft preview, and public rendering experiments. The core platform contract is still owned by the Go backend: drafts are structured data, block and theme validation happen before persistence or publish, and published websites are served from immutable snapshots or generated artifacts.

TanStack Start is the preferred React candidate in the implementation plan. The current specs do not require Next.js-specific hosting, middleware, image optimization, or app-router conventions. Choosing TanStack Start keeps the builder close to TanStack Router and TanStack Query conventions while leaving public site serving to the Go backend and publish pipeline.

The web app also needs a consistent UI implementation path. Tailwind CSS keeps styling close to the React components and matches the planned block-system CSS approach. shadcn/ui provides practical, source-owned primitives for forms, buttons, dialogs, menus, tabs, loading states, and empty/error states without making a third-party component library the canonical product model.

The root architecture spec in `structure.md` also points toward a Puck-powered builder, canonical draft data owned by the backend, publish-time artifact generation, and Go serving public websites. TanStack Start fits that boundary because it can host the builder and Puck adapter without making Puck state or a frontend framework the public runtime contract.

## Consequences

- The frontend app should be scaffolded as a React/TanStack Start package when the frontend foundation item is started.
- The TanStack Start app should be configured with Tailwind CSS, shadcn/ui, the `@/*` import alias, and a shared `cn` utility.
- Builder UI should prefer shadcn/ui primitives before bespoke components.
- The builder should call the Go API through typed client code rather than owning canonical draft state.
- Public site serving remains a Go/backend concern until a later publish-rendering decision changes that.
- Next.js remains a fallback only if deployment or ecosystem constraints become more important than the current architecture suggests.
