-- +goose Up
create extension if not exists pgcrypto;

create table users (
  id uuid primary key default gen_random_uuid(),
  email text unique not null,
  name text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table workspaces (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  created_by uuid references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table workspace_members (
  workspace_id uuid references workspaces(id) on delete cascade,
  user_id uuid references users(id) on delete cascade,
  role text not null default 'owner',
  created_at timestamptz not null default now(),
  primary key (workspace_id, user_id)
);

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

alter table sites
  add constraint sites_published_version_id_fkey
  foreign key (published_version_id)
  references site_versions(id)
  on delete set null
  deferrable initially deferred;

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

create table page_view_daily (
  site_id uuid not null references sites(id) on delete cascade,
  page_id uuid not null references pages(id) on delete cascade,
  view_date date not null,
  view_count bigint not null default 0,
  primary key (site_id, page_id, view_date)
);

create table audit_events (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid references workspaces(id) on delete cascade,
  site_id uuid references sites(id) on delete cascade,
  user_id uuid references users(id),
  action text not null,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index block_instances_page_sort_idx on block_instances (page_id, sort_order);
create index block_instances_site_idx on block_instances (site_id);
create index generation_jobs_workspace_created_idx on generation_jobs (workspace_id, created_at desc);
create index pages_site_sort_idx on pages (site_id, sort_order);
create index site_domains_site_idx on site_domains (site_id);
create index sites_workspace_idx on sites (workspace_id);

-- +goose StatementBegin
create function set_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;
-- +goose StatementEnd

create trigger users_set_updated_at before update on users
  for each row execute function set_updated_at();
create trigger workspaces_set_updated_at before update on workspaces
  for each row execute function set_updated_at();
create trigger sites_set_updated_at before update on sites
  for each row execute function set_updated_at();
create trigger site_domains_set_updated_at before update on site_domains
  for each row execute function set_updated_at();
create trigger themes_set_updated_at before update on themes
  for each row execute function set_updated_at();
create trigger pages_set_updated_at before update on pages
  for each row execute function set_updated_at();
create trigger block_instances_set_updated_at before update on block_instances
  for each row execute function set_updated_at();
create trigger generation_jobs_set_updated_at before update on generation_jobs
  for each row execute function set_updated_at();

-- +goose Down
drop trigger if exists generation_jobs_set_updated_at on generation_jobs;
drop trigger if exists block_instances_set_updated_at on block_instances;
drop trigger if exists pages_set_updated_at on pages;
drop trigger if exists themes_set_updated_at on themes;
drop trigger if exists site_domains_set_updated_at on site_domains;
drop trigger if exists sites_set_updated_at on sites;
drop trigger if exists workspaces_set_updated_at on workspaces;
drop trigger if exists users_set_updated_at on users;
drop function if exists set_updated_at();

drop table if exists audit_events;
drop table if exists page_view_daily;
drop table if exists form_submissions;
drop table if exists generation_jobs;
alter table if exists sites drop constraint if exists sites_published_version_id_fkey;
drop table if exists site_versions;
drop table if exists assets;
drop table if exists block_instances;
drop table if exists pages;
drop table if exists themes;
drop table if exists site_domains;
drop table if exists sites;
drop table if exists workspace_members;
drop table if exists workspaces;
drop table if exists users;
