# Decision 0002: Frontend App Location

## Status

Accepted

## Decision

Keep the React frontend in this repository as a workspace package under `apps/web`.

## Context

The backend is a Go modular monolith and remains the owner of persistence,
validation, publishing, and public site serving. The frontend needs to share the
same product lifecycle while staying independently buildable as a TanStack Start
application.

Putting the app in `apps/web` gives the project one repository for the prototype
loop while leaving room for shared generated API types under `packages/*`.

## Consequences

- Root npm scripts delegate to the `@snaelda/web` workspace.
- Generated API types can be added later without a second repository.
- The Go backend and React builder can still deploy as separate services.
