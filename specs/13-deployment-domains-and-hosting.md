# Deployment, Domains, and Hosting

## MVP Deployment Shape

Recommended first deployment:

- one builder app for authenticated editing
- one backend API service
- one lightweight public site service
- one Postgres database
- one object storage bucket for uploads and published site artifacts
- optional Redis/cache later
- optional background worker later

Generation can initially be request/response while still recording status in `generation_jobs`.

## Suggested Domains

- `app.platform.com` for the builder
- `*.platform.com` for hosted customer sites
- custom domains later through `site_domains`

## MVP Subdomain Hosting

Each site gets:

```text
{site-slug}.platform.com
```

Request resolution flow:

1. Extract hostname
2. Lookup `site_domains.hostname`
3. Resolve site
4. Load published version
5. Load the published page artifact by path from object storage
6. Return response

## Later Custom Domains

Expected workflow:

1. User adds domain
2. Backend creates verification token
3. User adds DNS record
4. Backend verifies DNS
5. Domain status becomes active
6. Hosting layer provisions TLS/certificates

## TLD and Market Notes

The custom-domain lifecycle above is registrar-agnostic, but the go-to-market markets bring TLD-specific realities:

- **`.is` (Iceland, ISNIC).** Customers register `.is` domains directly at ISNIC; registration or resale by Snaelda is out of scope. ISNIC verifies nameserver delegation before activating a domain, which differs from the generic add-TXT-record flow — the verify step must accept delegation-based setups, and help copy should walk an ISNIC customer through it in Icelandic ([Spec 22](./22-localization.md)).
- **`.se` (Sweden)** follows in the Sweden phase; no work now.

The `platform.com` placeholders in this spec correspond to `snaelda.io` in production.

## EU Hosting

"EU-hosted" is a customer-facing trust claim in both markets and must be verifiable, not copy. Deployment keeps all customer data processing inside the EU/EEA:

- Postgres and object storage run in an EU/EEA region
- the LLM provider used for generation must process data under terms compatible with the claim (EU processing or an equivalent contractual basis) — this is the easiest component to miss

## Hosting Principle

Do not create separate deployed apps per customer site. The platform should run one maintained public site service that serves many published websites from stored output artifacts.
