# Vertical Block Sets and Parametric Range

## Responsibility

This spec defines vertical block sets: named, founder-curated compositions over the existing closed block registry (Spec 04) that give each business vertical its own selection of blocks, variants, theme ranges, starter collections, imagery direction, and generation guidance. It also defines the anti-sameness contract — the explicit parametric range that keeps two sites in the same vertical from looking like twins.

The strategy source is `docs/nordic-gtm-strategy.md` §3 and §10: **constrain the design, liberate the content.** The founder designs the blocks; AI assembles, fills, writes, and localizes within them. Vertical sets are how that constraint is scoped per market segment without reopening design freedom.

## Scope Boundaries

- This spec does **not** add new block types. The registry in Spec 04 is unchanged in substance.
- This spec does **not** reopen the closed registry. AI still generates only known blocks, versions, and validated props (Spec 07 guardrails apply unmodified).
- This spec does **not** cover per-vertical marketing or SEO content (landing pages, keyword targeting, demo copy). That is go-to-market work, not platform data.

## What a Vertical Set Is

A vertical set is a **selection and curation layer** on top of the flat, closed block registry. The registry stays flat; blocks keep their existing `category` field and per-block variants (e.g. hero `variant` / `layout` in Spec 04). A vertical set never defines a block — it composes existing ones.

Each set is a named, code-defined object with six parts:

### 1. Included Blocks

The subset of registry block types this vertical uses. Generation for a site seeded by this set only plans blocks from the subset. A café set might include hero, image+text, gallery, collection list, testimonials, CTA band, contact form, footer, and stats — and exclude pricing tables and team cards it has no use for.

### 2. Pinned and Preferred Variants

Per-block variant guidance drawn from the variants blocks already expose:

- **Pinned** — the only allowed value (e.g. a tourism set pins hero `variant: full-page`).
- **Preferred** — a weighted shortlist generation chooses from (e.g. gallery `layout` preferring `masonry` over `grid`).

Pinning and preference are curation over Spec 04's existing enums; no new variant values are introduced here.

### 3. Theme Preset Ranges

A curated range over the theme tokens in Spec 11 — the parametric raw material:

- allowed **palette families** (the derivation from `brand.primaryColor` still holds; the set constrains which surface/accent temperature families the derivation may land in)
- allowed **font pairings** (a shortlist of heading/body pairs, not free font choice)
- allowed **density and spacing** presets (section padding, max width)
- allowed **shape** presets (radius, button radius)

Generation picks within the range; the user's brand color drives the palette as in Spec 11. Nothing outside the range is reachable.

### 4. Starter Collection Schemas

Ready-made collection schemas (Spec 19) typical for the vertical: `services` for salons and trades, `menu_items` for cafés, `properties` or `tours` for tourism. These are seeds, not constraints — the Spec 07 collection planner may still design bespoke collections when the prompt warrants it, but a matching starter schema is preferred because it has been proven against the vertical's block bindings and card templates.

### 5. Imagery Direction

Subject and style guidance that shapes stock search queries (the Pexels tool in Spec 07) and starter-image selection: subjects to seek, subjects to avoid, orientation and mood preferences. Brand-asset pull (Spec 21 re-spin) always takes priority over stock — real imagery is the strongest anti-sameness axis and the set's direction only fills gaps.

### 6. Generation Guidance

Per-vertical planning hints consumed by the Spec 07 pipeline:

- **tone** — copywriting register for the vertical (warm and personal for salons; matter-of-fact and trust-led for trades)
- **section ordering** — typical block order per page goal (image-led verticals open on imagery; trades open on proof and services)
- **typical pages** — the default page plan (still capped at 10 editor-visible pages per Spec 07)

## The Anti-Sameness Contract (Parametric Range)

**Design goal: two sites generated in the same vertical must not look like twins.** The strategy names this the sameness failure mode — constrain too hard and day-one "wow" inverts into "oh, it's one of *those*."

Every vertical set must ship with genuine range on each of these variation axes:

| Axis | Source of variation |
|---|---|
| Type pairing | The set's font-pairing shortlist (Spec 11 typography tokens) |
| Palette family | Brand-color-derived palette landing in different allowed families |
| Density / spacing | The set's density and spacing presets |
| Per-block layout | Preferred-variant shortlists over Spec 04 block variants |
| Imagery | Real brand assets pulled during re-spin (Spec 21), stock search guided by imagery direction — never the same defaults twice |

Two rules govern the range:

1. **Variation comes from curated per-vertical range, never from freeform user design freedom.** There is no slider, no custom CSS, no free font picker. The user liberates *content* — words, colors (via brand), photos — not layout or aesthetics. This restates the Spec 04/11 posture; vertical sets widen the curated range, not the user's design surface.
2. **The floor is absolute: no combination reachable within a vertical set may produce an ugly or broken result.** Every point in the cross-product of the axes is founder-curated and visually verified. If a pairing looks bad, it is removed from the range rather than papered over with guidance. The moat is the floor, not the AI.

How much range is enough is an empirical question answered by the QA loop below, not decided in the abstract.

## Selecting a Vertical Set

A vertical set is chosen once, at generation time, by one of two paths:

- **Re-spin flow (Spec 21, being written in parallel):** the extraction pipeline classifies the business from its existing site content and picks the matching set.
- **Prompt flow (Spec 07):** intent extraction (Step 1) already extracts site type and industry; a classification step maps that intent onto an available set.

**A generic/default set always exists** and is the fallback whenever classification fails, is ambiguous, or no vertical set matches. Classification failure must never block or degrade generation — the default set is itself a fully curated set (roughly today's un-verticalized behavior, formalized), not an error state. This mirrors the re-spin guardrail: degrade gracefully, never a dead end.

The chosen set and its version are recorded on the site (see Data and Versioning below).

## Cardinality and Rollout

- **One vertical per market at a time** — a strategy guardrail, restated here as a product rule. Vertical sets are expensive to curate to the quality bar; parallel half-finished sets are worse than one excellent one.
- **The first set is one Icelandic vertical.** The MLP is the re-spin producing a genuinely stunning before/after for that single vertical. Which vertical (tourism-adjacent, salons, trades, or cafés) is an open product decision recorded below; the mechanism in this spec must not depend on the choice.
- **More verticals are the primary fast-follow lever against sameness.** Each additional set adds genuine range across the whole platform. The second market (Sweden) gets its own single vertical on entry.

At all times the deployed set list is therefore small: the default set, plus one live vertical per market, plus at most one in curation.

## Quality Bar

A vertical set **ships only when re-spin output for real businesses in that vertical is consistently excellent.** This ties directly to the 50-site QA loop in Spec 21: point re-spin at real sites in the vertical; wherever output is not excellent, fix the set — trim the range, adjust variant preferences, tighten imagery direction, improve generation guidance — and re-run. The AI is the failure machine that generates the QA set; the founder's curation is what it tests.

**WCAG 2.1 AA is baked in at block and variant design time.** Every block variant, theme preset, and palette family a vertical set exposes must already meet AA (contrast, focus, structure) before the set may include it. Vertical curation inherits accessibility from the blocks; it never introduces a combination that breaks it (e.g. a palette family whose derived text contrast fails). A dedicated accessibility spec covering the platform-wide requirements is future work; this spec only requires that vertical curation never ship a non-conforming combination.

## Data and Versioning

Vertical sets follow the block-registry precedent (Spec 14, Decision 2): **defined in code, versioned, deployed with the application.** They reference block types, variants, theme presets, and collection schemas — all of which live in code — so a DB-first representation would desynchronize immediately.

```ts
export const verticalSets = {
  default: { versions: { "1.0.0": defaultSetV1 } },
  // first Icelandic vertical, name TBD (see Open Questions):
  // e.g. "is-salons": { versions: { "1.0.0": salonSetV1 } }
};
```

Rules:

- Each site records the **set id and version** that seeded it (`verticalSet: { id, version }` in the site record, Spec 06), for QA attribution and support — the same reasoning as block instances recording `type` + `version` (Spec 04).
- Breaking changes to a set create a new version; existing sites keep their recorded version as provenance. Because a set only acts at generation time, no migration of existing sites is required when a set changes.
- **Sites are never locked to their set.** The set constrains what generation produces, not what the user may edit afterward. Post-generation editing follows the ordinary Spec 08 rules: the user can add any registry block, switch any allowed variant, and edit theme tokens exactly as any other site. Re-prompts and AI maintenance actions (Spec 07, Spec 20) should continue honoring the recorded set's guidance where it still applies, but the editor is not vertical-scoped.

## Open Questions

- **Which single Icelandic vertical launches the MLP?** Candidates: tourism-adjacent, salons, trades, cafés. This is the open product decision from the strategy doc (§13); everything in this spec is designed to work regardless of the answer.
- How much parametric range per axis is enough to beat sameness without ballooning the curation surface? The 50-site QA loop (Spec 21) answers this empirically per vertical.
- Whether re-prompt flows should offer an explicit "re-seed from a different vertical set" action, or whether switching verticals always means generating a fresh draft.
