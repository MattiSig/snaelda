# Localization

## Purpose

Native Icelandic is the primary wedge for the Iceland beachhead ([docs/nordic-gtm-strategy.md](../docs/nordic-gtm-strategy.md)): almost nobody localizes for this market, and the founder can personally QA Icelandic output. That wedge only works if the language story holds end to end — generated copy, the published site, the builder UI, emails, pricing, and the trust footer. This spec owns the locale model and the language contract every other surface honors. Iceland ships first (`is` + `en`); Swedish (`sv`) is reserved and reuses the same machinery in the Sweden phase.

## Current State

A codebase audit found the locale plumbing exists in the schema but nothing speaks it:

- `sites.default_locale` exists end to end (migration `000001_init.sql`, siteconfig validation, `internal/sites` reader/writer) but **nothing consumes it**; new sites hardcode `'en'` (`internal/sites/mutator.go`).
- Generation accepts `preferredLanguage` (`internal/generation/handler.go`) and stores it but never uses it — prompts and fallback copy are hardcoded English (`internal/generation/service.go` literally assumes "Default locale is English"; category headline/subheadline/about fallback copy is English-only). The builder UI never even sends `preferredLanguage` (`apps/web/src/lib/api.ts` accepts it; `app.index.tsx` doesn't pass it).
- The frontend has zero i18n infrastructure; all builder and marketing strings are hardcoded English; billing hardcodes `$` (`app.billing.index.tsx`); date formatting delegates to the browser locale via `Intl.*(undefined, ...)`.
- All email templates are English-only (`internal/email/templates/*.txt`); the send helpers take no locale.
- Published-site platform strings are hardcoded English in Go — e.g. the form success message "Thanks. Your message is on its way." in `internal/forms/service.go`.
- No user or workspace locale column exists anywhere; only sites carry one.

The point of this spec is to close those gaps.

## Scope

**In scope:** per-site, single-locale sites in `is` or `en` (`sv` reserved). Every site has exactly one content locale; everything on that site — generated copy, platform strings, dates, structured data — is in that locale. This is the market wedge and the MVP shape.

**Out of scope:** multi-locale sites — one site serving several languages with per-locale page trees, locale switchers, or `hreflang` alternates. This **clarifies, not reverses**, the "multi-language sites" non-goal in [Spec 01](./01-product-summary-and-scope.md): that non-goal excludes one site in many languages; it never excluded a site whose single language is Icelandic.

## Locale Model

### Supported locales

A single allow-list, owned by the backend and shared with the frontend:

```
supported_locales = ['is', 'en']   -- 'sv' reserved, added in the Sweden phase
```

`sites.default_locale` ([Spec 06](./06-database-design.md)) gains a check constraint against the allow-list. Site creation stops hardcoding `'en'` and takes the locale from the creating flow (prompt input, re-spin detection, or explicit picker).

### Two locales, two jobs

- **Site content locale** — `sites.default_locale`. Governs everything the site's *visitors* see: generated copy, rendered platform strings, date/number formatting, `<html lang>`.
- **Product locale** — a new `locale` column on `workspaces` (default `'is'` in the Iceland phase), constrained to the same allow-list. Governs everything the site's *owner* sees: builder UI language and transactional email language. Workspace-level rather than per-user for MVP; the [Spec 03](./03-domain-model.md) workspace is the identity boundary that emails already key off.

The two are independent: an Icelandic owner can run an English-language site for tourists, and the builder still speaks Icelandic to them.

### Resolution rules

| Surface | Locale source |
|---|---|
| Generated site content ([Spec 07](./07-generation-engine.md)) | site content locale |
| Published/preview rendering ([Spec 09](./09-preview-publish-and-rendering.md)) | site content locale |
| Builder UI | workspace locale |
| Transactional email ([Spec 18](./18-transactional-email.md)) | workspace locale |
| Marketing site + public demo | visitor choice, defaulting to `is` in the Iceland phase |
| Trial sessions ([Spec 17](./17-guest-authoring-and-claim.md)) before a workspace exists | visitor choice, persisted onto the workspace at claim |

## Generated-Content Language Contract

This is the heart of the spec. The generation engine ([Spec 07](./07-generation-engine.md)) must treat the site's locale as a hard output contract, not a hint.

### Every copy-producing stage writes in the site locale

`preferredLanguage` in the generation input maps to `sites.default_locale` and is threaded into **every** stage that produces human-readable text:

- site plan copy (site name tagline, page goals surfaced to users)
- content draft (headlines, body copy, FAQs, feature items, CTAs, placeholder testimonials)
- collection entry seeding ([Spec 19](./19-collections-and-content-types.md)) — entry field values, labels
- SEO titles and meta descriptions
- image alt text
- navigation labels
- AI maintenance/assist actions ([Spec 07](./07-generation-engine.md), [Spec 20](./20-ai-authoring-ux.md)) — "add a service", "rewrite this entry", location variants
- change summaries and reprompt diffs shown in the builder ([Spec 20](./20-ai-authoring-ux.md))

The hardcoded English fallback copy in `internal/generation/service.go` becomes locale-keyed: every deterministic fallback string ships in all supported locales, selected by the site locale. The builder passes the chosen locale on every generation call — the current silent drop in `app.index.tsx` is a bug this spec fixes.

### Slug transliteration

Slugs stay ASCII for URL safety, but Icelandic input must transliterate deterministically rather than strip. One shared slugifier (backend-owned, used for site, page, and entry slugs):

| Character | Slug form |
|---|---|
| þ / Þ | th |
| ð / Ð | d |
| æ / Æ | ae |
| ö / Ö | o |
| á é í ó ú ý (and uppercase) | a e i o u y |

Swedish later adds å→a, ä→a, ö→o under the same table. "Þjónusta" → `thjonusta`, "Verkefni í Reykjavík" → `verkefni-i-reykjavik`. Existing slugs are never rewritten retroactively.

### Language-conformance validation

English leaking into an Icelandic draft is the single most embarrassing failure mode for the "native Icelandic" wedge — it instantly reads as machine output. The validation/repair stage (Spec 07, Step 7) gains a language-conformance check:

- every generated user-visible string is checked against the site locale (cheap heuristic language detection is sufficient; per-string classification, not per-draft)
- proper nouns, brand names, and user-provided verbatim content are exempt
- violations route through the existing repair loop ("rewrite these fields in Icelandic"), same as schema violations
- repeated failure falls back to regenerating the offending stage, never to shipping mixed-language copy

### Model split for non-English locales

Copy quality in Icelandic varies sharply by model; structural planning does not. When the site locale is non-English, the copy-producing stages (content draft, entry seeding, SEO text, alt text) **may use a stronger model** than the planning stages (intent, collection plan, site plan, layout). The decomposed pipeline (Spec 07 Execution Shape) makes this split cheap — it's a per-stage model choice, not an architecture change. English sites keep the cheap model throughout.

### Tone guidance

- **Icelandic:** natural small-business register — direct, warm, unpretentious; the register a Reykjavík salon or contractor would actually use. No translated-English constructions. The founder personally QAs this; the prompt guidance is tuned against real feedback, not written once.
- **Swedish (reserved):** du-form throughout, understated, lagom, no hype or superlatives — recorded now so the Sweden phase adds a tone profile, not a new mechanism.

Tone profiles are per-locale prompt fragments owned by the generation engine, versioned alongside the stage prompts.

## Rendered-Site Output

Published and preview rendering ([Spec 09](./09-preview-publish-and-rendering.md)) keys off `sites.default_locale`:

- `<html lang="is">` on every rendered page
- `og:locale` (`is_IS`, `en_US`, later `sv_SE`)
- locale-aware date and number formatting in rendered blocks (hours, entry dates, prices) via `Intl` with the site locale passed explicitly — never `undefined`
- LocalBusiness JSON-LD ([Spec 04](./04-block-registry.md)) emits locale-appropriate values where the schema carries text

### Platform string table

Platform-owned strings that appear on *published sites* — form success/error/validation messages, "closed" in opening hours, required-field notices — move out of Go string literals into a per-locale string table keyed by `sites.default_locale`, owned by the backend and covering all supported locales. The form success message in `internal/forms/service.go` is the canonical example: an Icelandic site must say "Takk fyrir" territory, not "Thanks. Your message is on its way." Missing keys fail loudly in tests, not silently in English.

## Product UI Localization

The web app gets an i18n layer. Framework choice is left to implementation, but three properties are required:

- **string extraction** — no hardcoded user-visible literals in components; lint-enforceable
- **locale-keyed catalogs** — `is` and `en` catalogs first, `sv` slot reserved; catalog completeness checked in CI
- **app-controlled formatting** — all `Intl.DateTimeFormat` / `Intl.NumberFormat` calls receive the resolved locale explicitly; the browser locale is never the implicit source of truth

The builder resolves its locale from the workspace; the marketing/landing surface from visitor choice with `is` default. **The marketing surface ships Icelandic natively — written, not machine-translated.** Per the GTM strategy, an English-only landing page reads like every global builder; the Icelandic landing copy is Phase 0 work and the founder writes or approves it directly.

## Email Localization

This section moves template localization **out** of [Spec 18](./18-transactional-email.md)'s out-of-scope list (Spec 18 is amended separately; this spec is the source of truth for the requirement):

- every template in the Spec 18 set gains locale-keyed variants (`is`, `en` now; `sv` reserved), text and HTML both
- the `internal/email` helper layer (`SendMagicLink`, `SendBillingReceipt`, …) gains a locale parameter; the `Mailer` transport interface is unchanged
- source of truth is the **workspace locale**; pre-claim trial emails use the locale captured in the trial session
- missing variant falls back to `en` with a logged warning — never a hard failure on the send path
- `billing_receipt` must format ISK correctly (see Currency below): `2.900 kr.`, no decimals
- `form_submission_forwarded` is owner-facing and uses the workspace locale, even when the submitting site is in another language

## Currency and Pricing Display

Per the GTM guardrail: **always local currency, never USD customer-facing.** ISK first.

- all customer-facing prices — marketing pricing table, billing screen, checkout, receipts — display in the workspace's market currency; the `$` hardcoded in `app.billing.index.tsx` is removed
- the price/tier model and Stripe price objects live in [Spec 15](./15-billing-and-stripe.md); this spec owns only the display rule
- **ISK is a zero-decimal currency in Stripe**: amounts are whole krónur (`2900` = 2.900 kr.), and display formatting must never divide by 100 or render decimals
- formatting goes through the same app-controlled `Intl.NumberFormat` path as everything else (`is-IS` renders `2.900 kr.`)

## Trust Localization

Locale-market trust signals, cross-referenced rather than owned here:

- **kennitala in the footer** for Icelandic businesses — the Footer block ([Spec 04](./04-block-registry.md)) gains an optional business-registration field, rendered with the correct local label ("Kt." for Iceland; "Org.nr" reserved for Sweden). Spec 04 is amended separately; generation seeds the field when the prompt or re-spin source provides it.
- **.is domain guidance** — the custom-domain flow ([Spec 13](./13-deployment-domains-and-hosting.md)) surfaces ISNIC-aware guidance for `.is` domains for Icelandic-locale sites; no registrar integration, guidance only.

## Rollout

**Iceland phase (now):** ship `is` + `en` across every surface in this spec. **Sweden phase:** add `sv` to the allow-list, catalogs, string table, email variants, slugifier, and tone profiles — new locale data, zero new mechanism. That property is the acceptance test for the design: if Swedish requires code changes beyond registering a locale, the abstraction is wrong.

### "Icelandic-ready" checklist

The Iceland beachhead is language-ready when all of the following hold:

- [ ] `sites.default_locale` constrained to the allow-list; site creation sets it from user input, never hardcoded
- [ ] `workspaces.locale` exists and drives builder UI and email language
- [ ] every generation stage produces Icelandic for an `is` site, including fallback copy, alt text, SEO text, nav labels, and AI-assist actions
- [ ] language-conformance validation rejects English leakage into Icelandic drafts
- [ ] Icelandic slug transliteration applied to site, page, and entry slugs
- [ ] published `is` sites render `<html lang="is">`, `og:locale`, localized platform strings (forms, hours), and locale-correct dates/numbers
- [ ] builder UI fully rendered from the `is` catalog with no hardcoded-string escapes
- [ ] marketing/landing surface live in natively written Icelandic
- [ ] all transactional emails available in Icelandic, keyed off workspace locale
- [ ] all customer-facing prices in ISK, zero-decimal, no `$` anywhere
- [ ] kennitala renderable in the footer of Icelandic sites

## Out of Scope

- multi-locale sites (per-locale page trees, locale switchers, `hreflang`) — see the Spec 01 non-goal
- machine-translation workflows or translation-memory tooling
- per-user locale overrides within a workspace (workspace-level is enough for MVP)
- right-to-left script support
- locale-specific legal-content generation (privacy policies, terms) beyond the trust-footer fields above
