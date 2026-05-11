# Repository Guidelines

## Project Structure & Module Organization

Snaelda is split between a Go backend and a TanStack Start frontend. Backend entrypoints live in `cmd/api` and `cmd/db`; reusable backend code is under `internal/`, grouped by domain modules such as `sites`, `pages`, `themes`, `publishing`, and platform packages like `internal/platform/config` and `internal/platform/database`. Database migrations are in `internal/platform/database/migrations`.

The web app lives in `apps/web`. Routes are file-based under `apps/web/src/routes`, shared UI is in `apps/web/src/components`, and utilities/API helpers are in `apps/web/src/lib`. Product and architecture specs are in `specs/`; ADRs are in `docs/decisions/`.

## Brand & Design Guidance

Before making frontend design decisions, read `BRANDING.md` and align layout, visual style, tone, colors, typography, and interaction feel with the brand guidance there. `snaelda` means spindle in Icelandic, and the core visual theme is a ball of yarn wrapped around a spindle. Use `logo.png` as the primary visual reference for that motif, including the warm, crafted, ribbon/thread-like feeling it establishes.

Dark mode is required for frontend work. Follow the `BRANDING.md` dark-mode direction: it should feel a bit meaner than light mode, with warmer near-black/plum backgrounds, stronger contrast, brighter ribbon colors, and a sharper, more dramatic palette while staying readable and calm.

For frontend design work, also read `PRODUCT.md` before editing UI. If a `DESIGN.md` file exists, treat it as the current design-system reference for colors, typography, components, spacing, and interaction patterns. If `DESIGN.md` is absent, derive decisions from `BRANDING.md`, `PRODUCT.md`, existing UI tokens, and `logo.png`.

When using the Impeccable design workflow, follow the local skill instructions in `.agents/skills/impeccable/SKILL.md` instead of copying the full skill documentation into this file. Keep `AGENTS.md` as the project-level overview and let the skill file remain the source of truth for Impeccable-specific preflight, command references, and design-law details.

## Build, Test, and Development Commands

- `make dev-up`: start local Postgres and SeaweedFS via Docker Compose.
- `make api`: run the Go API with local database and S3-compatible defaults.
- `make db-migrate`: apply database migrations with `go run ./cmd/db migrate up`.
- `make db-seed`: seed local database data.
- `make test`: run all Go tests with `go test ./...`.
- `npm run web:dev`: start the frontend dev server for `@snaelda/web`.
- `npm run web:lint`: run ESLint for the TypeScript client.
- `npm run web:build`: build the frontend and run TypeScript checking.
- `npm run web:start`: run the built TanStack Start server.

Copy `.env.example` to `.env` for local configuration, and avoid committing real secrets.

## Coding Style & Naming Conventions

Format Go code with `gofmt`; use standard Go package naming, table tests where they improve clarity, and exported names only when crossing package boundaries. Keep backend modules aligned with existing domain folders in `internal/`.

Frontend code uses TypeScript, React 19, TanStack Router, Tailwind CSS, ESLint, and shadcn-style components. Follow the existing style: 2-space indentation, single quotes, named components in `PascalCase`, helpers in `camelCase`, and route filenames matching TanStack file-route patterns such as `app.sites.$siteId.preview.tsx`.

## Testing Guidelines

Go tests use the standard `testing` package and live beside implementation files as `*_test.go`. Name tests by behavior, for example `TestReadyWithDatabaseError`. Prefer `httptest` for API handlers and `t.Setenv` for configuration cases. Run `make test` before backend changes and `npm run web:lint && npm run web:build` before frontend changes.

For web-facing changes, also test the running app with the Playwright MCP by clicking through the affected user flow in a browser. Verify the page renders, primary controls work, and no obvious console or navigation errors occur before handing off.

## Commit Guidelines

Recent commits use short imperative subjects such as `Scaffold Go API foundation` and `Add Postgres migration foundation`. Keep commits focused and describe the outcome, not the process.

When handing off changes, include a brief summary, test commands run, and screenshots for visible frontend changes. Call out migrations, configuration changes, and new environment variables explicitly.
