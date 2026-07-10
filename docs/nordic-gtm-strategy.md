# Snælda — Nordic Go-To-Market Strategy & Playbook

> **Status:** Adopted 2026-07-02. This supersedes the earlier `GTM.md` positioning where they conflict.
> **Current focus:** Phase 0 / Iceland beachhead prep — Icelandic localization first. Sweden-specific items (Swish, Fortnox, WCAG enforcement pressure, SEK) are recorded here but deferred until the Iceland beachhead is live. Payments stay on plain Stripe checkout (cards) for now — no wallet/Swish/Aur work — and `.is`-specific (ISNIC) domain guidance is deferred (2026-07-10).

> **Product:** Snælda (snaelda.io) — "Spin up a website. A real one."
> Drop your URL (or describe your business) → your content reborn into a designed site → tweak in a light editor → publish → point your domain.
> For small operations that need a website, *not* a website project.
> **Markets:** Iceland (beachhead) → Sweden (volume).
> **Operator:** Solo founder, Icelandic, based in Sweden. 10 YOE web dev / SEO / performance.

---

## 1. The Whole Product on a Napkin (Proven · Better · New)

Mark Pincus's framework (from *Life at the Speed of Play*, 2026): **copy what's proven, make it better until 10/10 people say an emphatic yes, then add one new bet — in that order.** The new bet will probably fail, so it stays small and comes last.

- **PROVEN:** a constrained, template-based builder. Copy Squarespace, not Wix. URL-import is a proven, expected category.
- **BETTER:** *"Paste your URL, watch your business reborn — in your language, in 30 seconds."* One demonstrable moment that bundles the high design floor, the rewrap (it's recognizably *theirs*), and native Icelandic/Swedish. The before/after **shows** the better instead of claiming it.
- **NEW:** the operations spindle — leads → booking → Fortnox handoff. Added last, atomic, demand-pulled, expected to wobble.

**If a feature doesn't fit on this napkin, it's probably a B+ stealing the A's space.**

> **"A B+ is the enemy of an A."** Current B+ risks crowding the A: multiple-site support, the "larger LLM," and the CRM. None make a salon owner say "f*ck yes" at signup; all add solo-maintained surface. They sit *behind* the A, never beside it.

**Instinct vs. idea:** instincts are right ~95% of the time, specific ideas wrong ~75%. The winning *instincts* — constrain the design, rewrap their content, Nordic-native, founder-led — are the 95%. The *idea* most at risk of being the wrong 75% is "...and also multi-site + CRM + bigger model at launch." Trust the instincts; defer the ideas.

---

## 2. Core Thesis

**Enter through the website. Expand into the operations. Lock in with the handoff.**

1. **Beachhead — the website.** The one thing every small business knows it needs. Easy to sell, instant magic moment (the rewrap).
2. **Moat — total-basics operations.** Leads, booking, simple status. Nobody buys a "CRM" cold — but they'll let their contact form quietly become a lead list. Smuggled in through the front door.
3. **Lock — the accounting handoff (Fortnox in SE).** Once leads → bookings → invoices flow through Snælda, switching cost is baked in. A website alone churns; a website holding their operations does not.

**The real prize is retention, not upsell margin.** Build toward **day-365 retention** (e.g. % of sites still publishing leads/bookings at 12 months), not virality.

**The unfair advantage no competitor can copy:** a single, reachable, accountable founder, natively Icelandic, operating inside Sweden. Wix/Squarespace structurally cannot tell this story.

---

## 3. Design Philosophy — High Floor, Liberated Content

**People are bad at designing web pages. Given a blank canvas, a non-designer is given the freedom to fail.** So Snælda doesn't hand over a canvas — the founder (10 YOE) designs the building blocks; AI assembles, fills, writes, and localizes within them. This is the Squarespace thesis (constraint → can't-look-bad) sharpened with AI, and it is the product, not a compromise.

**Division of labor that creates the moat:**
- **Human taste** → the design of the blocks. Where AI is weak (judgment about what "good" looks like for a Nordic service business).
- **AI** → arrange, fill, write copy, localize, swap imagery. Where AI is strong.
- Generic builders ask AI to *design* (mediocre, samey). Snælda asks AI to *assemble pre-designed excellence*. **The moat is the floor, not the AI** — and taste baked into components can't be commoditized the way "we use AI" can.

**Operational leverage:** a bounded design space = consistent output, fewer broken states, fewer support tickets. Constraint is what makes this maintainable by one person.

**The failure mode to engineer against — sameness.** Constrain too hard and every Snælda site looks like a Snælda site; day-one "wow" inverts into "oh, it's one of *those*." The craft is leaving enough **parametric range** (type, color, density, layout variants, and especially real imagery via content/pixel search) that two salons never look like twins — while never letting the user reach anything ugly.

**The rule:** **constrain the design, liberate the content.** Lock the aesthetic floor/ceiling; let people pour in their own words, colors, photos, and booking flows freely. Build blocks **per vertical** (a salon set, a trades set, a café set) so variation comes from genuine local range, not from a slider that lets users wreck it.

---

## 4. Signature Feature — "Re-spin" (URL Rewrap)

**Drop in your current site URL → we rewrap its content with our approach.** This is Proven + Better (build early), and it solves three hard problems at once: the blank-page problem, the demo, and Iceland distribution.

**Why it's technically cheap (because of the design philosophy):** you are **not** cloning the source layout (a JS/edge-case nightmare). You only need their *content*, not their *design*. It's the "liberate the content" pipeline with a URL as the source instead of a form.

**The pipeline (solo-sized):**
1. Server-side fetch (headless for JS-rendered sites)
2. Readability-style extraction + LLM cleanup
3. LLM: classify the business → pick the vertical block set → extract structured fields (name, services, hours, contact, about, testimonials) → rewrite copy into natural Icelandic/Swedish in tone → flag what's missing
4. Asset pull: logo, photos, brand colors (from CSS/logo). Image gaps → content/pixel auto-search
5. Compose into vertical block draft → **before/after view** → claim + edit → gated publish

**Guardrails (real failure modes):**
- **Degrade gracefully.** JS-walled / bot-blocked / thin sites must fall back to the prompt flow with whatever was salvaged — never a dead end, never a broken first impression.
- **Preserve their identity.** Pull and keep real brand assets aggressively. If the rewrap strips them into a generic template, you trigger the sameness problem and "that's my business but beautiful" becomes "it's one of those."
- **Frame as "first draft from your current site,"** immediately editable. Extraction will misclassify; fix is fast correction, not perfect extraction.
- Ownership is clean (their content in) — keep publish gated behind them claiming it.

**This is your top-of-funnel.** Make a **public** version — paste URL, see the reborn site with no signup; signup to keep it. The before/after is your most shareable asset, and before/after screenshots are exactly what travels in Icelandic small-business Facebook groups. It is simultaneously lead magnet, viral demo, and the thing that makes Phase 1 reference builds near-instant. Customer-facing verb: **"re-spin"** (unspool the old site, respin it — fits the name).

---

## 5. Positioning by Market

| | **Iceland** | **Sweden** |
|---|---|---|
| **Role** | Beachhead. Win small, fast, by network. | Volume. Enter with proof from Iceland. |
| **Primary wedge** | Native Icelandic (almost nobody localizes here) | Accessibility-compliant by default (legal pressure live) |
| **Supporting wedges** | Founder-market-fit, network, ISK + .is | Swish + Fortnox, EU-hosted + one real human, native Swedish tone |
| **Distribution** | Facebook groups + personal network (NO paid ads) | Vertical SEO, bookkeeper/accountant referrals, LinkedIn + FB groups |

---

## 6. Pricing

Always price in **local currency. Never USD** — a dollar tag kills the local-native story.

| Tier | USD ref | SEK | ISK | What's in it |
|---|---|---|---|---|
| **Site** | $19/mo | 199/mo | 2.900/mo | Builder, re-spin, AI content/images, forms → CSV export, custom domain, hosting |
| **Pro** | $49/mo | 499/mo | 6.900/mo | + CRM-lite, prospect export, larger LLM, multiple sites *(all behind the A — upsell, not launch headline)* |
| **Creator once-over** | $99 once | 999 once | 13.900 once | Founder personally reviews & polishes. Human-touch differentiator + quality feedback loop. |

---

## 7. Table Stakes

### Sweden
- [ ] **Swish** (via Stripe) — default payment for service SMBs
- [ ] **Native Swedish content** — du-form, understated, lagom, no hype
- [ ] **WCAG 2.1 AA by default** — legally live, surveillance active, fines up to 10M SEK
- [ ] **GDPR + cookie consent** (IMY), EU hosting
- [ ] **.se domain**

### Iceland
- [ ] **Cards via Stripe** — card-first market (~72% credit usage); wallet enablement deferred
- [ ] **Native Icelandic content** — your superpower; quality you can personally judge
- [ ] **ISK pricing** (`.is`/ISNIC domain guidance deferred)
- [ ] Accessibility — same direction via EEA; WCAG-by-default covers it free

> **Stripe covers payments for BOTH markets in one integration.** Don't integrate rails individually.

**Differentiators (later, demand-pulled):** BankID (SE) / rafræn skilríki via Auðkenni (IS) — always via aggregator (Criipto/Signicat/ZignSec/Scrive), never direct · Klarna/faktura (SE), Netgíró (IS) · Fortnox integration (SE, the retention lock) · native booking + Swish deposit · Reco.se reviews embed (SE).

---

## 8. The Playbook (phased)

### Phase 0 — Make it Nordic-ready *(weeks 1–3, before marketing)*
Live page is English-only, no pricing, no trust layer — reads like every global builder. Fix first.
- [ ] Swedish + Icelandic landing pages (write IS natively; nail SE tone)
- [ ] Trust layer: EU-hosted, "built & personally checked by one developer you can reach," GDPR clarity, accessibility statement
- [ ] WCAG 2.1 AA baked into every component (at component-design time)
- [ ] Stripe live (cards; Swish and wallets deferred)
- [ ] Pricing in SEK + ISK
- [ ] **Re-spin v1 + the public before/after demo** (this is the centerpiece, not a nicety)
- [ ] Forms → lead capture → CSV export

### Phase 1 — Iceland beachhead *(weeks 3–8)*
- [ ] Re-spin 5–10 real micro-businesses from your network (café, salon, contractor, tour op) — free/near-free for testimonials + before/after
- [ ] Post the before/afters in Icelandic small-business Facebook groups (THE channel)
- [ ] Work personal network for intros
- [ ] ISK via Stripe cards (wallets, Aur, and `.is` guidance all deferred)
- [ ] 60-sec demo: URL → reborn Icelandic site in minutes
- **Exit target:** ~10 reference sites, 5+ testimonials, first paying conversions

### Phase 2 — Sweden entry *(weeks 8–16)*
Crowded (Wix, Squarespace, One.com, Hostinger) — lead with the 4-part wedge.
- [ ] Full Swedish localization (tone = translation)
- [ ] **Pick ONE vertical** — make blocks, copy, SEO, demo sing for it
- [ ] SEO: "tillgänglig hemsida", "AI hemsida [vertical]" — accessibility angle underserved
- [ ] Partner channel: **redovisningskonsulter** (bookkeepers on Fortnox) refer SMB clients
- [ ] LinkedIn + Swedish FB groups

### Phase 3 — Pro upsell engine *(ongoing)*
- [ ] Form-fills surface as visible leads (the upgrade trigger)
- [ ] "Export prospects / push to Fortnox" = Site → Pro
- [ ] Fortnox tie = SE retention moat

### Phase 4 — Compounding loops *(ongoing)*
- [ ] "Made with Snælda" footer on entry sites (toggleable, default on)
- [ ] Iceland testimonials → Swedish case studies → SEO → more signups
- [ ] Keep publishing the accessibility angle until you *are* the accessible-by-default builder in Nordic search

---

## 9. Ops Layer — Architecture *(the NEW bet — build last)*

**Don't build a CRM. Expose one you already have.** A "prospect" is a form submission with a few extra fields. Website and CRM are the same data spine.

- **Lead record:** name · email · phone · message · source page · referrer/UTM · timestamp · status · notes
- **Pipeline (brutally simple):** `New → Contacted → Customer`
- **Outputs:** one-click CSV/Excel export (v1, near-free) · Fortnox handoff (lead → Customer pushes customer + draft invoice) · larger LLM drafts follow-ups + powers multi-site content

### "Total basics" discipline
Test for every ops feature: **does it fall naturally out of website data, or is it a separate product?**

| ✅ In (thin layers on data you hold/hand off) | 🚫 Trap (separate products that eat a solo founder) |
|---|---|
| Leads from forms | Full multi-stage pipelines |
| New→Contacted→Customer status | Email marketing |
| Booking + Swish deposit | Inventory |
| Push-to-Fortnox | Invoicing logic you own |
| | Payroll |

The moment you're rebuilding Fortnox or a dedicated booking tool — **stop and integrate.**

---

## 10. Build Order

### Maximum Launchable Product (not MVP)
The MLP is concrete: **re-spin producing a genuinely stunning before/after for ONE Icelandic vertical.** That polished moment *is* the launch — not a broad half-builder.
- [ ] Constrained per-vertical block system (one vertical, excellent)
- [ ] **Re-spin pipeline + public before/after demo**
- [ ] AI content/images in IS / SE / EN
- [ ] Forms → lead capture → CSV export
- [ ] Stripe (cards)
- [ ] Custom domain (.se / .is), GDPR consent, EU hosting
- [ ] WCAG 2.1 AA components
- [ ] $99 / 999 SEK creator once-over (just your time)

### Fast-follow
- [ ] More verticals (block sets) — variation kills the sameness risk
- [ ] **Booking + Swish deposit** ← first ops feature (in presets; makes leads AND payments)
- [ ] Pipeline view + AI follow-up drafts
- [ ] Fortnox integration
- [ ] BankID / rafræn skilríki via aggregator

### Later (only if demand pulls)
- [ ] Klarna/faktura, Netgíró
- [ ] Iceland accounting integration (Payday/Dk/Regla; CSV covers you until then)
- [ ] Multiple-site support, larger-LLM tier *(B+ risks — defer behind the A)*
- [ ] Reco/reviews embed
- [ ] Full ecommerce + shipping (different product; only if you choose it)

**Ops sequence:** booking → CRM-lite → Fortnox handoff. None before the beachhead has paying customers pulling for it. **Let demand order the roadmap.**

---

## 11. Guardrails (tattoo on the wall)

1. **Protect the A.** The re-spin "reborn in your language in 30 seconds" moment gets the demo, the polish, and your scarce solo hours. B+ features (multi-site, bigger LLM, CRM) queue behind it.
2. **Constrain the design, liberate the content.** Lock the floor/ceiling; free the words, colors, photos. Variation comes from per-vertical block range, never user freedom.
3. **Iceland by network and groups, never paid ads.** Sweden can test small paid later — earn the proof first.
4. **Always price in local currency.**
5. **One vertical per market at a time.**
6. **Keep your face on it.** The solo, reachable founder is the one moat no competitor can copy. Never hide behind "we."
7. **CSV export is the universal escape hatch** — defers every integration without blocking a sale.
8. **"Total basics" must stay total-basics.** Out-building a dedicated app = you've lost that fight.
9. **Maximum launchable, not minimum viable.** Ship narrow-but-excellent.
10. **AI as a failure machine.** Point re-spin at 50 real Icelandic sites; where output isn't 10/10, fix the blocks. It generates its own QA set.

---

## 12. Immediate Next Actions

- [ ] **Phase 0** — Swedish + Icelandic landing pages + SEK/ISK pricing + **re-spin v1 with public before/after**. Nothing else starts until the site stops reading as generic-global.
- [ ] Draft IS + SE landing copy (hero, "how it spins", pricing)
- [ ] Build the first vertical block set (pick the vertical)
- [ ] Sketch the booking flow (first ops feature)
- [ ] Line up 5–10 Icelandic micro-businesses to re-spin for Phase 1

---

## 13. Open Questions

- Which **single Icelandic vertical** to launch the MLP on? (tourism-adjacent · salons · trades · cafés)
- Which **single Swedish vertical** for Phase 2? (salons/barbers · hantverkare · consultants)
- Free vs paid for the first Iceland re-spins — how many free before switching to paid?
- Does Pro's "larger LLM" matter to buyers, or is IS/SE *fluency + tone* the real lever? (Lean: the latter.)
- How much parametric range is enough to beat sameness without reintroducing the freedom-to-fail problem?
- When does the Iceland accounting integration beat CSV-only?
