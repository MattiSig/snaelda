# GTM

> **Superseded:** the go-to-market source of truth is now [docs/nordic-gtm-strategy.md](docs/nordic-gtm-strategy.md) (adopted 2026-07-02). This file records what changed and the decisions the new strategy does not resolve.

## What the new strategy replaces

The earlier version of this document described a small, curated, English-language launch. The Nordic strategy supersedes it on these points:

- **Hook.** `Prompt in. Good first draft out.` → *"Paste your URL, watch your business reborn — in your language, in 30 seconds."* Re-spin ([specs/21](specs/21-respin-url-import.md)) with its public before/after demo is the top-of-funnel; the homepage prompt remains as the second entry point and the fallback.
- **Market.** Generic/global → Iceland beachhead (native Icelandic as the wedge, distribution via network and Facebook groups, no paid ads), then Sweden for volume.
- **Pricing.** `$20/month` single plan in USD → local currency only, never USD: Site 2.900 ISK/mo, Pro 6.900 ISK/mo (upsell, not launch headline), Creator once-over 13.900 ISK once. See [specs/15](specs/15-billing-and-stripe.md).
- **Audience framing.** Craft-hobbyist examples → Nordic service SMBs, one vertical per market at a time ([specs/23](specs/23-vertical-block-sets.md)).

## What still stands

- The trial mechanics (no-signup entry, 4 days, 25 prompts, publish to subdomain, claim ladder) are unchanged and owned by [specs/17](specs/17-guest-authoring-and-claim.md).
- Solo-dev discipline: small surface, personal support, honest scarcity, "keep your face on it."
- Do not make the free experience too limited before value is shown; do not fake scarcity; do not allow unlimited paid growth before the product is ready.

## Open decision — the Founding 50 cap

The old plan capped the first wave at 50 paying users with a visible countdown; the new strategy is silent on capacity and its Phase 1 exit target (~10 Icelandic reference sites, first paying conversions) is compatible with a cap. Options:

1. Keep a cap, scoped per market (e.g. Founding 50 for Iceland) — preserves the solo support load and the honest-scarcity messaging.
2. Drop the cap and rely on one-vertical-per-market focus to bound the load.

Unresolved; decide before Phase 1 marketing starts.
