# AI Authoring UX

## Purpose

[Spec 07](./07-generation-engine.md) defines the generation pipeline as a backend contract: prompt → structured plan → repair → draft. [Spec 08](./08-editor-and-authoring.md) defines the manual edit surface. This spec is the seam between them — the user-facing experience that makes Snaelda *feel* like an AI-first CMS rather than "a CMS that happens to have a generate button."

Three concerns own this seam:

1. Progress during long-running generation calls.
2. Iterative re-prompting that feels like a conversation rather than a destructive overwrite.
3. AI-suggest affordances inside the editor itself, beyond initial site generation.

If the user types a prompt and sees nothing for 60 seconds, they have already left. If they re-prompt a page and cannot tell what changed, they will lose trust within two tries. These are MVP problems, not polish.

## Out of Scope

- The structured-output planner pipeline itself (owned by [Spec 07](./07-generation-engine.md)).
- The block, page, theme, and navigation editors (owned by [Spec 08](./08-editor-and-authoring.md)).
- Generation-driven collection maintenance actions (owned by [Spec 19](./19-collections-and-content-types.md)) — except their UX surface in the builder, which is covered here.
- Conversational free-text chat with the model. Snaelda is a structured editor with AI affordances, not a chat product.

## Generation Progress

Every generation request that takes longer than ~1 second has a visible, structured progress UI. The user always knows what step the system is on.

### Surfacing Steps

The generation pipeline in [Spec 07](./07-generation-engine.md) has named phases. The backend emits a progress event for each phase transition. The frontend renders these as an ordered checklist with current-step animation.

Required phase set (subset of [Spec 07 §Pipeline](./07-generation-engine.md)):

1. `prompt.normalize` — "Reading your prompt"
2. `plan.pages` — "Planning pages and structure"
3. `plan.theme` — "Picking colors and typography"
4. `plan.blocks` — "Choosing blocks for each page"
5. `assets.fetch` — "Finding starter imagery"
6. `copy.write` — "Writing copy"
7. `validate.repair` — "Checking and repairing"
8. `persist` — "Saving your draft"

Skip steps that do not apply (e.g. `assets.fetch` when no image-bearing blocks were planned). Emit a `complete` event with the resulting `siteId` and a `failed` event with a user-facing reason on terminal failure.

### Transport

Server-sent events (SSE) over a single long-lived response from `POST /api/sites/generate`. The endpoint upgrades to `Content-Type: text/event-stream` when the client sends `Accept: text/event-stream`; otherwise it behaves as today (single JSON response on completion). This keeps the legacy contract for tests and CLI usage.

Event shape:

```
event: progress
data: {"step": "plan.pages", "label": "Planning pages and structure", "index": 2, "total": 8}

event: complete
data: {"siteId": "...", "draftId": "..."}

event: failed
data: {"reason": "model_timeout", "message": "We could not finish — please try again."}
```

Page reprompt, site reprompt, and theme regeneration use the same event shape over their respective endpoints.

### Frontend Behavior

- The "Weaving your draft" copy is replaced with a vertical step list. The active step animates (subtle pulse on the row). Completed steps show a check; pending steps show a dim dot.
- The list is rendered immediately on submit, with all steps initially pending. The first SSE event flips step 1 to in-progress.
- A skeleton preview of plausible block shapes is rendered to the right of the step list once `plan.blocks` fires, so the user sees the page taking shape before copy lands.
- If the connection drops, the frontend falls back to polling `GET /api/sites/generate/:jobId` every 2 seconds until the job resolves. Generation must therefore be tracked server-side as a job with `id`, `state`, `currentStep`, `error`.
- Generation jobs older than 1 hour are pruned.

### Backend Job Model

A new `generation_jobs` table:

| column | type | notes |
|---|---|---|
| `id` | uuid pk | |
| `workspace_id` | uuid not null | |
| `site_id` | uuid nullable | populated once `persist` succeeds |
| `kind` | text not null check (kind in ('site','page_reprompt','site_reprompt','theme_regenerate','entry_generate','entry_reprompt','variant_fanout')) | |
| `state` | text not null default 'pending' check (state in ('pending','running','succeeded','failed','canceled')) | |
| `current_step` | text nullable | |
| `error_reason` | text nullable | |
| `started_at` | timestamptz | |
| `completed_at` | timestamptz nullable | |
| `payload` | jsonb not null default '{}'::jsonb | original request inputs needed for retry |

Indexes on `(workspace_id, state)` and `(started_at)` for pruning.

This table also serves as the durable backing for retry, cancel, and the "Show me what changed" history described below.

## Iterative Re-prompting

A re-prompt today replaces the draft and offers a single undo button. That is a destructive overwrite, not a conversation. Spec 20 raises the bar:

### Reprompt History

Every reprompt (site-scoped, page-scoped, entry-scoped) writes a `reprompt_history` row recording: scope, prompt text, resulting `draft_revision_id`, originating `generation_jobs.id`, created_at, and a short model-authored summary of the change ("Tightened the hero copy and replaced the gallery with a stats band.").

| column | type | notes |
|---|---|---|
| `id` | uuid pk | |
| `site_id` | uuid not null | |
| `scope` | text not null check (scope in ('site','page','entry','theme')) | |
| `target_id` | uuid nullable | page id, entry id, or null for site/theme |
| `prompt` | text not null | |
| `previous_revision_id` | uuid not null references draft_revisions(id) | |
| `result_revision_id` | uuid not null references draft_revisions(id) | |
| `job_id` | uuid references generation_jobs(id) | |
| `change_summary` | text nullable | model-authored, 1-2 sentences |
| `created_at` | timestamptz not null default now() | |
| `undone_at` | timestamptz nullable | when the user reverted this reprompt |

`draft_revisions` already exists. The pair of `previous_revision_id` and `result_revision_id` makes any reprompt reversible to either side.

### History Panel

A "History" pane in the builder right-rail (collapsible, not a separate route) lists reprompts for the current scope, newest first. Each row shows:

- the prompt text (truncated, expand on click)
- the change summary
- timestamp
- two actions: **Revert** (re-apply `previous_revision_id`) and **Show diff**

The current single-slot `undoSiteReprompt` UI is replaced by this panel. Multi-level revert works because revisions form a chain.

### Diff View

"Show diff" opens a side-by-side block diff in a modal:

- left column: rendered blocks from `previous_revision_id`
- right column: rendered blocks from `result_revision_id`
- block-level additions, removals, and modifications are highlighted (green / red / amber gutters)
- prop-level differences inside a kept block show as inline highlights on the affected text

The diff is computed in the frontend from the two snapshots returned by `GET /api/sites/:siteId/revisions/:revisionId`. No new backend diff endpoint is required.

### Pre-reprompt Confirmation for Risky Scopes

A site-level reprompt that would discard >50% of current blocks shows an inline warning above the submit button: "This will rewrite most of your site. We'll save a snapshot you can revert to." Page-level reprompts skip the warning unless they would discard the only page.

## AI-Assist Inside the Editor

Beyond initial generation and reprompt, the editor surfaces AI affordances on specific elements:

### Per-block Suggest Actions

A small "Improve with AI" affordance appears on hover for text-bearing blocks (hero headline, feature copy, CTA text, FAQ answers, testimonial quote, footer tagline). Clicking opens a small dropdown:

- **Tighten** — rewrite shorter
- **Expand** — rewrite longer with more detail
- **Change tone** → submenu of `friendlier`, `more professional`, `more playful`, `more direct`
- **Rewrite from prompt…** — opens a single-line prompt input scoped to this block

These are powered by the same generation backend, with a new scope `block_suggest` on the job table. Block suggest writes through the normal reprompt history as `scope = 'block'` so the user can revert any AI-driven block edit individually.

Block suggest is *not* free-form structural change — the resulting block always uses the same `type` and `version` as the source. The model is constrained to rewriting the existing props, not adding or removing the block. Structural change still flows through page-level reprompt.

### Per-image Suggest Actions

Image-bearing blocks gain a "Find a better image" action next to each `assetId` field. This opens the asset picker pre-filtered by a model-suggested query derived from the surrounding page context (page name + nearby headline + block intent). The user picks from the resulting starter set or uploads.

This reuses the existing `internal/imagery` Pexels pipeline. No new external integration is required for MVP.

### Page Suggestions

The empty state of a new page (just `Add Block` placeholder) gains a "Suggest blocks for this page" affordance. The user types one sentence ("a contact page for our two offices"); the model returns 3 candidate block arrangements as click-to-apply cards.

### Site Suggestions

The site-level dashboard surfaces ambient suggestions ("Your About page has no team block — generate one?", "Your hero text is shorter than 4 words on mobile — rewrite?") in a single dismissible card slot. Suggestions are generated by a backend `internal/generation/suggestions` job that runs on draft save with a 10-second debounce per site. At most 1 suggestion per site at a time.

Ambient suggestions cost a prompt budget unit when *accepted*, not when shown.

## Loading and Error UX

- Every long-running AI affordance uses the same step-list component as initial generation, scaled to its scope (one step for block suggest, three for page suggest).
- Every failure surface shows a concrete reason (`model_timeout`, `model_invalid_output`, `quota_exceeded`, `network`) and an explicit retry button. No raw stack traces, no "an error occurred."
- A persistent toast queue in the builder collects completed-in-background events ("Reprompt complete — Review changes") so the user can continue editing during long calls.

## Empty States and Onboarding

- First-time builder load shows a 3-step overlay tour pinned to: prompt input on dashboard, History panel, Publish button. Tour state is workspace-scoped; dismissing on one site dismisses globally.
- Empty pages, empty navigation, empty submissions, and empty analytics each get one short coaching line plus a primary action.

## Accessibility

- The generation step list announces step transitions via `aria-live="polite"`.
- Diff view is keyboard-navigable: arrow keys move between changed blocks, `Enter` toggles "show before only" / "show after only" / "show both."
- AI-suggest dropdowns close on `Esc` and trap focus while open.

## API Surface Additions

(See [Spec 10](./10-api-surface.md) for the canonical list — this section enumerates only the additions required by Spec 20.)

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/sites/generate` | Existing endpoint, gains SSE behavior when `Accept: text/event-stream` |
| `GET` | `/api/generation/jobs/:jobId` | Poll fallback when SSE is unavailable |
| `POST` | `/api/generation/jobs/:jobId/cancel` | Cancel a running job |
| `GET` | `/api/sites/:siteId/reprompts` | List reprompt history for the site |
| `POST` | `/api/sites/:siteId/reprompts/:id/revert` | Re-apply the previous revision |
| `GET` | `/api/sites/:siteId/revisions/:revisionId` | Fetch a draft revision snapshot for diffing |
| `POST` | `/api/sites/:siteId/blocks/:blockId/suggest` | Block-suggest action (returns new block props) |
| `POST` | `/api/sites/:siteId/pages/:pageId/suggest-blocks` | Page-suggest action (returns candidate block arrangements) |
| `GET` | `/api/sites/:siteId/suggestions` | Ambient suggestion (at most one outstanding) |
| `POST` | `/api/sites/:siteId/suggestions/:id/accept` | Apply a suggestion |
| `POST` | `/api/sites/:siteId/suggestions/:id/dismiss` | Dismiss a suggestion |

All routes are authenticated and trial-scoped per [Spec 17](./17-guest-authoring-and-claim.md). Block-suggest, page-suggest, and ambient-suggestion *acceptance* count against the prompt budget; preview suggestion *fetches* do not.

## Open Questions

1. Should diff view show theme/token changes alongside block changes, or live in a separate "Theme history" tab? MVP picks the latter to keep the block diff focused.
2. Should ambient suggestions be opt-in per site or opt-out? MVP picks opt-out with a "Pause suggestions" toggle in site settings.
3. Should block-suggest dropdowns be available to trial users? Yes — but blocked-and-claim-prompted once the prompt budget runs out, same flow as page reprompt.
