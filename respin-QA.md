# Re-spin QA — mysewerguys.squarespace.com (2026-07-12)

## Run under test

| | |
|---|---|
| Source site | https://mysewerguys.squarespace.com/ (plumber/drain company, Squarespace) |
| Import | `5f35b285-ca1c-4d67-a714-e71d306a6a00`, started 18:33:39Z, succeeded 18:35:43Z (~2m04s) |
| Result | `status=succeeded`, `degraded=false`, `fetchMode=plain`, share slug `ce641ec9bf6fa644` |
| Output | 3 pages (Home, Services, About), green palette, stock photos |
| Screenshots | `source-home.jpeg`, `source-services.jpeg` vs `respin-home.jpeg`, `respin-services2.jpeg`, `respin-about.jpeg` |

Headline: the import reports success, but the draft shipped with **zero contact
details for a 24/7 emergency plumber**, **literal placeholder testimonial copy**,
and **none of the six named services**. The decomposed pipeline crashed and the
draft actually came from the legacy mega-call fallback. "A bit bad" is accurate.

## What actually happened (from Railway API logs)

```
18:33:39  respin brand pull failed  error="respin: brand pull needs a workspace and site"
18:34:04  openai_usage call=site_outline    total=3745
18:34:12+ openai_usage call=page_layout ×4  total=13,943
18:34:45  openai_usage call=page_content    total=4711   (1 of 4 pages)
18:34:46  decomposed generation failed; falling back
          error="compose page /services: write page content:
                 page content block 2 type mismatch: got \"gallery\" want \"image_text\""
18:35:41  openai_usage call=site_generation_plan  total=9664   (fallback mega-call)
18:35:43  succeeded
```

Three consequences:

1. **The draft the user saw is the fallback single-call output**, not the
   decomposed pipeline's. Everything below about content quality is a review of
   the weakest path.
2. **~22.4k of ~32k tokens were spent and discarded** (outline + 4 layouts + 1
   content call thrown away wholesale).
3. This is systemic, not a fluke: the 06:56Z run on a different import died with
   the *identical* signature (`got "gallery" want "image_text"`, on `/about`),
   and **brand pull has failed with the same error on every import in the log
   window** (23:59Z, 06:56Z, 18:33Z).

## Before/after content comparison

| Source site has | Re-spin produced |
|---|---|
| Phone (262) 682-1580 — in announcement bar and footer, tel: link | **Nothing. No phone anywhere.** |
| Email thesewerguys1@gmail.com (footer, mailto:) | **Nothing** |
| Street address: 4623 75th St., STE 4-245, Kenosha, WI 53142 (footer) | Footer shows only "Kenosha / WI / United States" |
| 6 named services with descriptions (/services): Sewer Line Cleaning, Camera Inspections, Drain Cleaning, Emergency 24/7, Root Removal, Preventative Maintenance | **No services list at all.** Services page = hero + one FAQ item about 24/7 response + placeholder testimonial |
| "Our Work in Action" gallery (12 photos) | No gallery (ironically, the model *tried* to emit one — that's the block that killed the decomposed run) |
| Service-area list (Kenosha, Racine, Mt. Pleasant, …) | Absent |
| FAQ page with ~10 real questions (/faqs) | One invented FAQ item ("How do I request service?") |
| Contact form on every page | **No contact_form block anywhere.** Hero CTAs link to `/#contact`, which doesn't exist — broken anchor |
| Trust badges (BBB, Google, Trustpilot, Chamber), partner referrals | Absent (acceptable scope cut, but nothing replaced the trust story) |
| Logo + blue/white brand | No logo, generic green palette (brand pull failed) |
| Punchy hero: "Clogged Drain? We Fix Them 24/7 – Fast and Reliable!" | Flat: "Fast, reliable sewer service" / "Quick, 24/7 service" (the same phrase is reused as home subheadline, about subheadline, *and* footer tagline) |

**Shipped placeholder copy** (public demo, unedited): every page ends with a
testimonials block containing *"Add one concise testimonial that speaks to the
actual experience of working together." — Client name, Client role*.

Naming inconsistency: site is titled "The Sewer Guys, LLC" but the about body
starts "My Sewer Guys is a locally owned company…" (extraction echoed the
subdomain-flavored copy; nothing normalizes it).

What went right: locale correctly detected as `en` (Icelandic UI didn't leak),
about-page prose is faithful to the source, no facts were *fabricated* (no fake
phone/prices — the guardrail held), theme is tasteful, and the run finished in
~2 minutes with sane token spend.

## Root causes (file:line)

### 1. Brand pull fails on 100% of imports — wiring bug
`internal/respin/pipeline.go:156` calls `PullBrand` with `WorkspaceID`, `UserID`,
`ImportID` — but **never `SiteID`**, and `internal/respin/brand.go:160` hard-errors
without it. At that point in the pipeline no site exists yet (generation creates
it later), so the stage can never succeed as wired. Result: no logo, no source
colors, no hero photos, on every re-spin. The log has warned about this on every
run and it reads as noise because the pipeline soldiers on.

### 2. Contact details never reach the LLM — readability stripping
`internal/respin/extract.go:66-79` skips `<header>`, `<footer>`, `<nav>`, `<form>`
subtrees, and `findContentRoot` (extract.go:169) restricts text to `<main>` when
present. On this site — and on most small-business sites — **the phone, email,
and street address live only in the header announcement bar and the footer.**
Additionally `resolveLink` (extract.go:~269) discards `tel:` and `mailto:` hrefs,
which are the highest-precision contact signals a page has. So `ExtractedFields.Contact`
came back empty → `resolvePages` (compose.go:154) never added a contact page →
the brief had no Contact section → a 24/7 emergency-service business shipped
with no way to call them.

### 3. Decomposed pipeline: schema permits what the validator rejects, and one bad block nukes the whole run
- The `page_content` schema (`internal/generation/openai.go:389-401`) is
  `anyOf` over all layout block types — the model *can* legally return `gallery`
  where the layout said `image_text`. Then `validatePageContentMatchesLayout`
  (openai.go:451) hard-fails on exactly that. The schema should pin block *i* to
  type *layout[i]* (`prefixItems` / per-index `const`), making this failure
  structurally impossible.
- There is **no per-page retry**: the errgroup in
  `internal/generation/decomposed_orchestrator.go:66-82` aborts everything, and
  `service.go:272` falls back to the mega-call. Three pages had already
  composed or were composing fine. A single retry of the one failed page-content
  call (~4k tokens) would have saved the run; instead ~22k tokens were discarded
  and quality dropped to the weakest path.
- 2-of-2 recent failures are `gallery` vs `image_text`. The model keeps wanting
  galleries for image-led trades pages (matching the source's work-photos
  gallery — a good instinct!). Either let the layout pass pick `gallery` more
  readily, or soften the mismatch into "accept the returned type if it's in the
  allowed catalog and repairable".

### 4. Decomposed per-page calls never see the brief — facts can't survive
`PageLayoutRequest`/`PageContentRequest` (`internal/generation/decomposed.go:63-98`)
carry `SiteName`, `SiteGoal`, page goal, outline, interview answers — **but not
`input.Prompt`** (see the call at decomposed_orchestrator.go:71). The re-spin
brief is the prompt: verbatim services, prices, hours, contact, testimonials.
The content composer's own prompt says "invent only if the prompt + interview
answers give you enough" (openai.go:1551) — but it never receives the prompt, so
even a *successful* decomposed run would degrade re-spin facts into placeholders.
The whole point of `composeBrief` ("use facts verbatim") is silently defeated on
this path.

### 5. Fallback mega-call lacks the guardrails the decomposed prompts have
`pageLayoutSystemPrompt` (openai.go:1533) says: *"Use contact_form only when the
page should collect visitor input. Do not use testimonials, cta_band, or
text_section as a substitute for an actual form."* The fallback
`generationPlannerSystemPrompt` (openai.go:1388) has **no equivalent** — and the
fallback output used `testimonials` and `faq` blocks headed "Get in touch" /
"Request service" as fake contact sections. The repair heuristic that should
catch this (`generatedTextMentionsContactForm`, repair.go:355) only matches
"contact form"/"inquiry form"/etc.; the generated copy said *"Use this form to
request service"* and slipped through. Prompt parity between the two paths, plus
a broader intent matcher (e.g. "use this form", "request service", tel-intent),
would have fixed the worst page-level defect.

### 6. Repair pass injects placeholder testimonials instead of dropping the block
`repairTestimonialsProps` (`internal/generation/repair.go:580-586`) backfills an
empty testimonials block with hardcoded "Add one concise testimonial…" copy.
Reasonable inside the builder where the owner edits next; harmful in the public
before/after demo where it reads as the product's output. For re-spin
generations (and arguably all first-generation drafts), an empty repeater block
should be **dropped**, not stuffed with meta-instructions — especially
testimonials, which the brief explicitly says never to fabricate.

### 7. Page discovery: /faqs loses its fetch slot to /cart and /privacy-policy
`highValuePaths` (`internal/respin/discover.go:134`) scores about/services/
contact/pricing/menu, but not `faq`. With `defaultMaxPages = 5`, the remaining
slots after `/`, `/about`, `/services` go to zero-score links in document order —
on this site `/cart`, `/privacy-policy`, `/terms-of-service` beat `/faqs`. Ten
real Q&As were sitting there; the model invented one instead. Add
`faq`/`faqs`/`spurningar` to the list and consider down-scoring
cart/privacy/terms/login explicitly.

## Recommendations, prioritized

1. **Fix brand pull wiring** (bug, every run): either pull brand *after* the
   site exists, or restructure ingestion so assets can be ingested against the
   workspace/import and attached to the site later. This alone transforms the
   demo (real logo + real colors = instant "that's my business" recognition).
2. **Harvest contact data before boilerplate stripping** (bug-level content
   loss): scan the full document (header/footer included) for `tel:`/`mailto:`
   hrefs, address microdata/og tags, and phone-shaped regexes; feed them to the
   extraction stage as candidate facts. Footer/nav stripping should apply to
   prose, not to structured NAP signals.
3. **Pin per-index block types in the page_content schema** and add a single
   per-page retry before abandoning the decomposed run. Together these convert
   the current worst case (whole-run fallback) into a ~4k-token retry.
4. **Thread the brief into decomposed per-page requests** (add `Prompt` to
   `PageLayoutRequest`/`PageContentRequest`, or a distilled facts block per
   page). Without this, fixing #3 makes runs *succeed* while still losing the
   verbatim facts re-spin exists to preserve.
5. **Prompt parity for the fallback path**: port the contact_form rule (and the
   "be specific, not generic" copy guidance) into
   `generationPlannerSystemPrompt`; broaden `generatedTextMentionsContactForm`.
6. **Never ship placeholder repeater content on generation** — drop empty
   testimonials/faq/pricing blocks in `repairGenerationPlan` when the plan is a
   fresh generation (keep the placeholder behavior for in-builder edits if
   desired).
7. **Discovery scoring**: add `faq(s)`, penalize `cart|privacy|terms|login`.
8. Smaller polish: dedupe the tagline (subheadline reused 3×), keep the
   business-name usage consistent between site title and about copy, and make
   hero CTAs target real anchors (`/#contact` pointed nowhere).

## Open questions / follow-ups

- The stored extraction (`respin_imports.extracted_content`) isn't visible
  anywhere; a debug/admin view (or share-endpoint field behind a flag) would
  let QA distinguish "extraction missed it" from "generation dropped it" without
  DB access. For this run the evidence (no contact page in hints, footer-only
  facts) points to extraction-missed for contact, generation-dropped for
  services.
- Whether extraction captured all 6 services or only the homepage's 4 is
  unverifiable from outside for the same reason.
- The mega-call reported `cached=0` despite `cachedSiteContext()` — worth a
  glance at whether the fallback's system prefix actually hits the OpenAI prompt
  cache in production.
