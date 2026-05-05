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
- `assets`
- `site_versions`
- `generation_jobs`
- `form_submissions`
- `page_view_daily`
- `audit_events`

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

### `pages`

```sql
create table pages (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  title text not null,
  slug text not null,
  sort_order int not null default 0,
  status text not null default 'draft',
  seo jsonb not null default '{}'::jsonb,
  settings jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (site_id, slug)
);
```

Add application-level or DB-level validation to ensure a site cannot exceed 10 active pages.

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
  settings jsonb not null default '{}'::jsonb,
  is_hidden boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

`site_id` is duplicated intentionally for easier authorization and querying.

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
