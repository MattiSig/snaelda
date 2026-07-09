# Re-spin: URL Import and Public Before/After Demo

## Purpose

Re-spin is the signature acquisition feature from the Nordic GTM strategy (docs/nordic-gtm-strategy.md §4): paste the URL of an existing small-business website and get that business's content reborn as a Snaelda draft — in the site's own language, with its own brand assets — shown as a public before/after demo that requires no signup.

This spec owns the URL import pipeline, the `respin_imports` provenance entity, the server-side asset ingest path, the public before/after demo, and the security contract for fetching attacker-controlled URLs. It deliberately does not own generation itself: re-spin's entire job is to produce the exact input contract of the generation engine (Spec 07) from a URL instead of a typed prompt.

The customer-facing verb is **"re-spin"** — unspool the old site, spin it again. It fits the spindle brand and must be used consistently in UI copy. Internal identifiers use `respin`.

Launch posture is Iceland-first: the target is one Icelandic vertical producing a genuinely stunning before/after (the GTM "Maximum Launchable Product"), not broad coverage.

This spec extends Spec 07 (Generation Engine), Spec 11 (Assets), Spec 12 (Security), and Spec 17 (Trial Sessions). It cross-references Spec 22 (Localization) and Spec 23 (Vertical Block Sets), both being written in parallel.

## The Hard Contract

**Re-spin's output is exactly Spec 07's Minimum Input shape.** The pipeline ends by composing:

```json
{
  "prompt": "<composed business brief synthesized from extracted content>",
  "workspaceId": "...",
  "siteName": "Hárgreiðslustofan Klippt",
  "preferredLanguage": "is",
  "brand": {
    "businessName": "Hárgreiðslustofan Klippt",
    "logo": "asset_ingested_logo",
    "primaryColor": "#7A3E48"
  },
  "optionalHints": {
    "industry": "salon",
    "style": null,
    "pages": ["home", "services", "contact"]
  }
}
```

Consequences of this contract:

- Extracted brand assets land in Spec 07's existing first-class `brand` input and are therefore used **verbatim** — generation must not invent alternatives. No new generation-side brand plumbing is needed.
- Structured extraction results (services, hours, testimonials, about copy) are carried in the composed `prompt` and `input_context`, so the generation engine consumes them through its normal intent-extraction and content stages.
- Degrading to the plain prompt flow is **structural, not bolted on**: a partially failed import simply produces a thinner instance of the same input shape, pre-filled into the homepage prompt UI.

Re-spin never introduces a second draft format, a second validation path, or a second persistence path. Everything downstream of input composition is Spec 07.

## Pipeline

1. **URL intake.** Normalize the URL (scheme, host lowercase, strip fragments and tracking params). Check the result cache (see Security Contract) before doing any work.
2. **Server-side fetch — plain first.** A plain HTTP fetch through the SSRF-guarded client. If the response yields insufficient readable content (empty `<body>`, client-rendered shell, content below a readability threshold), fall back to:
3. **Headless fallback.** A sandboxed headless browser renders JS-driven sites. The fetch mode used is recorded on the import record. Up to a small budget of additional same-origin pages (e.g. `/about`, `/services`, `/contact`, discovered from the fetched page's navigation, max 5) may be fetched under the same guards. Never off-site.
4. **Readability-style extraction + LLM cleanup.** Boilerplate stripping (nav chrome, cookie banners, footers) via readability heuristics, then an LLM pass that cleans the remaining text into coherent source content and flags what is missing.
5. **Business classification.** LLM classifies the business (vertical, services offered, locale signals, tone).
6. **Vertical block-set selection.** The classification picks the vertical block set per Spec 23. If no vertical set matches, fall back to the generic block registry (Spec 04) — this is a soft degradation, not a failure.
7. **Structured field extraction.** Extract into a typed shape: business name, services, opening hours, contact details (phone, email, address), about copy, testimonials. Each field is nullable; missing fields are flagged, not fabricated.
8. **Copy rewrite.** Rewrite extracted copy into natural, native copy in the site's target language per the localization contract in Spec 22. Target language defaults to the detected source-site language; the demo UI lets the user override before claiming.
9. **Brand asset pull.** Extract logo (link rel icons, `og:image`, header `<img>` heuristics), photos, and primary color (from CSS custom properties, dominant logo color, or theme-color meta). Assets go through the server-side ingest path below. Brand results populate `brand` in the canonical input. Preserving real brand identity is a hard requirement — a rewrap that strips the business into a generic template converts "that's my business but beautiful" into "it's one of those" (GTM §4 guardrail).
10. **Compose and generate.** Assemble the canonical Spec 07 input, create a `generation_jobs` row linked to the import record, and run the standard generation pipeline. All Spec 07 guardrails, validation, repair, and persistence apply unchanged.

## Graceful Degradation

Re-spin must never dead-end and never show a broken first impression. Failure handling is tiered:

| Condition | Behavior |
|---|---|
| JS-walled site, plain fetch empty | Headless fallback automatically |
| Headless also blocked (bot walls, CAPTCHAs) | Degrade to prompt flow |
| Thin content (under extraction threshold) | Degrade to prompt flow, pre-filled with what was found |
| Fetch error, timeout, oversize | Degrade to prompt flow |
| Classification confidence low | Proceed with generic block set; mark `degraded` |
| Asset pull partial/failed | Proceed without the failed assets; brand fields stay null per Spec 07's derivation rules |

"Degrade to prompt flow" means: the user lands in the ordinary homepage prompt experience with a prompt pre-filled from whatever was salvaged (business name, detected vertical, any extracted copy fragments) and a short, honest message — "We couldn't read everything from your site, so here's a head start." The `respin_imports` row records the degradation flag and reason. Because degraded output is just a thinner canonical input, no special downstream handling exists.

## Source Import Entity

`generation_jobs.prompt` is `not null` today (Spec 06), and a generation job has no natural place to hold fetch provenance, extracted content, or pulled-asset lineage. Re-spin therefore gets its own provenance entity that feeds the generation job:

```sql
create table respin_imports (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid references workspaces(id) on delete cascade,
  guest_session_id uuid references guest_sessions(id) on delete set null,
  source_url text not null,
  normalized_url text not null,
  fetch_mode text check (fetch_mode in ('plain', 'headless')),
  fetch_status text not null default 'queued'
    check (fetch_status in ('queued', 'fetching', 'extracting', 'composing', 'succeeded', 'degraded', 'failed')),
  extracted_content jsonb,
  classification jsonb,
  pulled_asset_ids jsonb not null default '[]'::jsonb,
  degraded boolean not null default false,
  degradation_reason text,
  share_slug text unique,
  error jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index respin_imports_normalized_url_idx on respin_imports(normalized_url);

alter table generation_jobs add column respin_import_id uuid references respin_imports(id);
```

- `workspace_id` is **nullable**: a public-demo import exists before any workspace does and is bound to a workspace only on claim.
- `extracted_content` holds the structured field extraction (name, services, hours, contact, about, testimonials, plus raw cleaned text).
- The generation job's `prompt` column is satisfied with the composed business brief, so the existing not-null constraint stands; `respin_import_id` carries provenance.
- `share_slug` backs the shareable before/after URL (below).

## Asset Ingest

Spec 11's only ingestion path today is the signed-upload flow (user → signed URL → storage → metadata). Re-spin adds a **server-side ingest path**: the backend fetches an external image through the SSRF-guarded client, validates it (content type allowlist, size cap, image decode check), stores it in object storage, and creates an `assets` row that yields a normal `assetId`.

Ingested assets carry source metadata: `source: 'respin'`, the origin URL, and the `respin_imports.id`. From the block/renderer perspective an ingested asset is indistinguishable from an uploaded one — blocks reference `assetId` exactly as in Spec 11. Ingested assets for an unclaimed demo are owned by the import; on claim they transfer to the new workspace, and unclaimed ones are garbage-collected with the import cache.

## Public Before/After Demo

The demo is the top-of-funnel (GTM §4): paste a URL on the public site, watch progress, and see a **before/after view** — their live site on one side, the re-spun draft on the other — with **no signup of any kind**.

- **Before** is a screenshot of the source site captured during fetch (the headless engine captures it when used; plain-fetch imports capture via a one-off sandboxed render of the source URL).
- **After** is the generated draft rendered through the standard preview pipeline (Spec 09) under a demo-scoped preview token.
- The view offers exactly two actions: **"Re-spin another"** and **"Sign up to keep it."**

This before/after view is a distinct surface from Spec 20's revision diff. Spec 20 diffs two draft revisions inside the builder; this is a marketing artifact comparing an external site to a generated draft. They share no code contract.

### Claim Handoff

"Sign up to keep it" hands off into Spec 17's trial machinery: the backend creates (or reuses) an L0 cookie-bound trial session and workspace, binds `respin_imports.workspace_id` and the generated site to it, transfers ingested assets, and lands the user in the builder. The claim consumes **one prompt** from the 25-prompt budget (the generation already happened; the claim books it). From that point the ordinary Spec 17 ladder applies — L1 link, L2 email, subscription — with the publish-gating exception below.

Unclaimed demo drafts are ephemeral: retained for the cache TTL, then deleted along with their ingested assets.

### Shareable Artifact

Before/after screenshots are exactly what travels in Icelandic small-business Facebook groups, so the demo result must be shareable without the recipient re-running the pipeline:

- Each completed demo gets a public URL at `/{locale}/respin/{share_slug}` showing the frozen before/after.
- The share page serves an Open Graph image that is the composed before/after side-by-side, so pasting the link into Facebook renders the money shot.
- Share pages are static snapshots of the demo moment; they do not expose the editable draft and carry their own "Re-spin your site" CTA.

## Security Contract

This endpoint performs server-side fetching of attacker-controlled URLs from unauthenticated callers — the largest new attack surface in the platform. Spec 12's existing rate limits cover cheap public writes (form submissions, magic-link requests); re-spin introduces a new tier: **expensive unauthenticated compute**. Both the fetcher and the LLM spend need their own protections.

### SSRF Guards

The fetch client (used for page fetch, asset ingest, and headless egress alike) must:

- allow only `http` and `https`, on ports 80 and 443
- resolve DNS itself and reject any resolved address in private, loopback, link-local, or cloud-metadata ranges (RFC 1918, `127.0.0.0/8`, `169.254.0.0/16` including `169.254.169.254`, `::1`, IPv6 ULA/link-local, and IPv4-mapped IPv6 forms)
- **pin the vetted IP for the actual connection** so a DNS-rebinding flip between check and connect cannot redirect the request
- cap redirects (max 5) and re-run the full guard on every hop
- reject credentials-in-URL forms (`user:pass@host`)

### Resource Caps

- plain fetch timeout ~10s; headless render budget ~25s wall-clock
- response size caps: ~5 MB per HTML document, ~10 MB per ingested image, ~40 MB total per import
- same-origin page budget of 5; no off-site fetches ever

### Headless Sandboxing

The headless browser is the highest-risk component: it executes attacker-supplied JavaScript. It must run in an isolated sandbox (dedicated container/jail, no host filesystem access, non-root), with all network egress forced through the SSRF-guarded proxy so in-page JS cannot reach internal ranges, downloads disabled, and hard kill on the render budget.

### Abuse and Cost Limits

- per-IP rate limit on demo starts (e.g. 3 per hour, small daily cap), durable per Spec 12's rate-limit posture
- a global concurrency cap on in-flight imports (headless slots especially); over-capacity requests queue briefly, then return a friendly "busy — try again shortly" response rather than degrading the whole service
- a daily LLM cost budget for the unauthenticated demo; when exhausted, the public endpoint pauses (the claim-side and in-builder flows, which are session-bound and quota-accounted, continue)
- **result caching per normalized URL** (e.g. 24h TTL): repeated pastes of the same URL — the expected behavior when a link travels in a Facebook group — serve the cached import and demo instead of re-running fetch and generation

### Bot Posture

Fetches identify honestly with a distinct User-Agent (`SnaeldaRespin/1.0 (+https://snaelda.io/respin-bot)`). Re-spin acts as a one-shot agent fetching on the site owner's explicit request, not a crawler; it still respects `robots.txt` disallow rules for the small same-origin page discovery beyond the exact URL given, and it never schedules recurring fetches.

## Ownership and Publish Gating

Product framing is **"a first draft from your current site," immediately editable**. Extraction will sometimes misclassify or miss fields; the remedy is fast correction in the builder, not perfect extraction. The demo and builder copy must set this expectation.

Publish gating diverges from the ordinary trial rule, and this divergence is deliberate:

- Spec 17 allows any trial session (L0/L1/L2) to publish to the hosted subdomain.
- **A re-spin-originated draft requires a claimed identity — Spec 17 L2, verified email — before it can be published.**

Rationale: the draft contains content scraped from a third-party website. Publishing it anonymously would let anyone clone an arbitrary business's content onto a Snaelda subdomain with zero accountability. Requiring a verified email before publish keeps ownership clean (their content in, a reachable person accountable) while leaving generation, editing, and preview fully open. The publish route detects re-spin origin via the site's generation job → `respin_import_id` linkage and returns the structured `identity_required` error, which the builder maps to the "Add an email" flow. Editing, re-prompting, and preview remain governed only by the ordinary trial rules.

## QA Loop

Re-spin is operated as a failure machine (GTM §11): the pipeline generates its own QA set.

- Before launch and continuously after, run re-spin against **~50 real Icelandic sites** in and around the launch vertical.
- Grade each output: fetch success, extraction fidelity, classification correctness, copy quality in Icelandic, brand preservation, and overall "would this screenshot travel."
- Every non-excellent output is a work item against a specific stage: fix the vertical blocks (Spec 23), the extraction prompts, the localization tone (Spec 22), or the degradation thresholds — not one-off patches to individual results.
- Keep the graded set as a regression suite: pipeline changes re-run against it before shipping.

The launch bar is the GTM MLP bar: one Icelandic vertical where the before/after is genuinely stunning, not many verticals where it is passable.

## API Surface

Public (no session required; expensive-compute limits apply):

```http
POST /api/respin                       # start an import for a URL; returns import id
GET  /api/respin/:importId             # status + progress events for the demo UI
GET  /api/respin/:importId/preview     # before/after payload (source screenshot + draft preview token)
POST /api/respin/:importId/claim       # create/reuse L0 trial session, bind workspace, enter builder
GET  /api/respin/share/:shareSlug      # frozen before/after snapshot for the share page
```

Session-bound (trial or authenticated; standard Spec 17 accounting):

```http
POST /api/sites/respin                 # run re-spin into the caller's existing workspace as a new site
```

Session-bound re-spins count against the 25-prompt budget like any generation. All routes emit `audit_events` with the import id in metadata.

## Scope Boundaries

- **No layout cloning.** Re-spin takes the source site's *content*, never its *design*. The output layout comes exclusively from Snaelda's block registry and vertical block sets. This is what makes the feature solo-sized (GTM §4).
- **No crawling.** Fetching is limited to the given URL plus a small same-origin page budget. No following external links, no site-wide spidering, no recurring fetches.
- **No ops layer.** Leads, booking, and Fortnox handoff (GTM §9) are out of scope here; re-spin ends at a claimable draft.
- **No source-site monitoring.** Re-spin is a one-time import, not a sync; later changes to the source site are not tracked.

## Open Questions

- Which single Icelandic vertical anchors the launch demo (tracks the GTM open question)?
- Should the demo offer a language toggle (Icelandic/English) on the after view before claim, or fix language at intake?
- Exact cache TTL and unclaimed-import retention window — 24h is the working default pending real share-traffic data.
