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

## Hosting Principle

Do not create separate deployed apps per customer site. The platform should run one maintained public site service that serves many published websites from stored output artifacts.
