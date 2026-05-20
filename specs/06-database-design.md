# Database Design

## Database Role

Postgres is the canonical system of record.

## Recommended MVP Tables

- `users`
- `workspaces`
- `workspace_members`
- `sites`
- `site_domains`
- `pages`
- `block_instances`
- `themes`
- `collections`
- `collection_entries`
- `assets`
- `site_versions`
- `generation_jobs`
- `form_submissions`
- `page_view_daily`
- `audit_events`
- `guest_sessions`
- `magic_links`

## Draft vs Published State

Recommended MVP approach:

- store draft state in normalized tables such as `pages`, `block_instances`, and `themes`
- store published state as immutable JSON in `site_versions.snapshot`
- point `sites.published_version_id` at the live published version

This supports easy editing, safe publishing, and straightforward rollback.

## Suggested Schema

### `users`

```sql
create table users (
  id uuid primary key default gen_random_uuid(),
  email text unique not null,
  name text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

### `workspaces`

```sql
create table workspaces (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  created_by uuid references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

### `workspace_members`

```sql
create table workspace_members (
  workspace_id uuid references workspaces(id) on delete cascade,
  user_id uuid references users(id) on delete cascade,
  role text not null default 'owner',
  created_at timestamptz not null default now(),
  primary key (workspace_id, user_id)
);
```

### `sites`

```sql
create table sites (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references workspaces(id) on delete cascade,
  name text not null,
  slug text not null,
  status text not null default 'draft',
  default_locale text not null default 'en',
  published_version_id uuid,
  generation_prompt text,
  generation_summary jsonb not null default '{}'::jsonb,
  settings jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (workspace_id, slug)
);
```

### `site_domains`

```sql
create table site_domains (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  hostname text unique not null,
  type text not null default 'subdomain',
  status text not null default 'active',
  verification_token text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

### `themes`

```sql
create table themes (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  name text not null default 'Default',
  version text not null default 'theme.v1',
  tokens jsonb not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (site_id, name)
);
```

### `collections`

```sql
create table collections (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  slug text not null,
  singular_label text not null,
  plural_label text not null,
  schema jsonb not null,
  settings jsonb not null default '{}'::jsonb,
  sort_order int not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (site_id, slug)
);
```

`schema` is an ordered list of field definitions whose `type` is drawn from the closed field-type registry in [Spec 19](./19-collections-and-content-types.md). Application-layer validation rejects unknown types, missing required keys, and enum option drift across entries.

### `collection_entries`

```sql
create table collection_entries (
  id uuid primary key default gen_random_uuid(),
  collection_id uuid not null references collections(id) on delete cascade,
  site_id uuid not null references sites(id) on delete cascade,
  slug text not null,
  fields jsonb not null default '{}'::jsonb,
  seo jsonb not null default '{}'::jsonb,
  status text not null default 'draft'
    check (status in ('draft', 'published')),
  sort_order int not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (collection_id, slug)
);

create index collection_entries_site_idx on collection_entries(site_id);
create index collection_entries_status_idx on collection_entries(collection_id, status);
```

`fields` is keyed by the parent collection's schema field `key`s. Entry validation runs on every write and again at publish time against the schema version captured in the snapshot.

`site_id` is duplicated for easier authorization and tenant-scoped indexing, mirroring `block_instances`.

### `pages`

```sql
create table pages (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  title text not null,
  slug text,
  type text not null default 'static'
    check (type in ('static', 'collection_index', 'collection_detail')),
  collection_id uuid references collections(id) on delete restrict,
  sort_order int not null default 0,
  status text not null default 'draft',
  seo jsonb not null default '{}'::jsonb,
  settings jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (site_id, slug)
);

create index pages_collection_idx on pages(collection_id) where collection_id is not null;
```

`slug` is nullable because `collection_detail` templates derive their URL pattern from the bound collection rather than from a static slug. Application-level validation enforces:

- `slug` is required for `static` and `collection_index` pages
- `collection_id` is required when `type` is anything other than `static`
- a site cannot exceed 10 editor-visible pages (this cap counts templates, not URLs produced by them)

See [Spec 19](./19-collections-and-content-types.md) for the page-type model and how each type serves URLs.

### `block_instances`

```sql
create table block_instances (
  id uuid primary key default gen_random_uuid(),
  page_id uuid not null references pages(id) on delete cascade,
  site_id uuid not null references sites(id) on delete cascade,
  type text not null,
  version text not null,
  sort_order int not null default 0,
  props jsonb not null default '{}'::jsonb,
  bindings jsonb not null default '{}'::jsonb,
  settings jsonb not null default '{}'::jsonb,
  is_hidden boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

`site_id` is duplicated intentionally for easier authorization and querying.

`bindings` maps individual prop keys to a collection entry field reference. It is only valid when the owning page's `type` is `collection_detail`; the application-layer validator rejects bindings on `static` and `collection_index` pages, and rejects bindings whose field type does not match the bound prop. See [Spec 19](./19-collections-and-content-types.md).

### `assets`

```sql
create table assets (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references workspaces(id) on delete cascade,
  site_id uuid references sites(id) on delete set null,
  kind text not null,
  storage_key text not null,
  public_url text,
  alt_text text,
  metadata jsonb not null default '{}'::jsonb,
  created_by uuid references users(id),
  created_at timestamptz not null default now()
);
```

### `site_versions`

```sql
create table site_versions (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  version_number int not null,
  snapshot jsonb not null,
  created_by uuid references users(id),
  created_at timestamptz not null default now(),
  publish_note text,
  unique (site_id, version_number)
);
```

### `generation_jobs`

```sql
create table generation_jobs (
  id uuid primary key default gen_random_uuid(),
  site_id uuid references sites(id) on delete cascade,
  workspace_id uuid not null references workspaces(id) on delete cascade,
  status text not null default 'queued',
  prompt text not null,
  input_context jsonb not null default '{}'::jsonb,
  output_plan jsonb,
  error jsonb,
  created_by uuid references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

### `form_submissions`

```sql
create table form_submissions (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  page_id uuid references pages(id) on delete set null,
  block_id uuid,
  payload jsonb not null,
  status text not null default 'new',
  spam_score numeric,
  created_at timestamptz not null default now()
);
```

### `page_view_daily`

For MVP analytics, prefer simple aggregated counts over heavy event analytics.

```sql
create table page_view_daily (
  site_id uuid not null references sites(id) on delete cascade,
  page_id uuid references pages(id) on delete cascade,
  view_date date not null,
  view_count bigint not null default 0,
  primary key (site_id, page_id, view_date)
);
```

This is enough for lightweight reporting such as:

- total site views
- views by page
- views by day

If later needed, raw event storage can be added separately.

### `audit_events`

```sql
create table audit_events (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid references workspaces(id) on delete cascade,
  site_id uuid references sites(id) on delete cascade,
  user_id uuid references users(id),
  action text not null,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);
```

Guest-initiated rows leave `user_id` NULL and place the originating session id in `metadata.guest_session_id`.

### `guest_sessions`

```sql
create table guest_sessions (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references workspaces(id) on delete cascade,
  cookie_token_hash text not null unique,
  recovery_key_hash text unique,
  prompts_used int not null default 0,
  trial_started_at timestamptz not null default now(),
  claimed_by_user_id uuid references users(id),
  claimed_at timestamptz,
  created_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now()
);

create index guest_sessions_workspace_idx on guest_sessions(workspace_id);
```

Binds a browser-held cookie token to a workspace so an unauthenticated visitor can create, edit, and publish a site before signup. See [Spec 17](./17-guest-authoring-and-claim.md). Key columns:

- `recovery_key_hash` — hash of the user-facing workspace recovery link (Spec 17 L1). NULL when the session is cookie-only or after the session has been promoted to L2.
- `trial_started_at` — start of the 4-day trial window. The trial enforcement layer reads `now() - trial_started_at` and compares to the 4-day cap.
- `prompts_used` — counter against the 25-prompt lifetime cap.
- `claimed_by_user_id` — set when the session is promoted to L2 (email attached) or when subscription Checkout creates the owning user. Both paths also add a `workspace_members` row.

`workspaces.created_by`, `assets.created_by`, `site_versions.created_by`, `generation_jobs.created_by`, and `audit_events.user_id` are all nullable to accommodate trial-authored rows. No further schema changes are required.

### `magic_links`

```sql
create table magic_links (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references users(id) on delete cascade,
  token_hash text not null unique,
  purpose text not null check (purpose in ('login', 'verify_email')),
  expires_at timestamptz not null,
  consumed_at timestamptz,
  created_at timestamptz not null default now()
);

create index magic_links_user_idx on magic_links(user_id);
```

One-time tokens for magic-link login and L2 email verification. Tokens are single-use, expire after 15 minutes, and store only a hash; the plaintext is delivered by email and never persisted.
