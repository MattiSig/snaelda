-- +goose Up
create table if not exists collections (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  slug text not null,
  singular_label text not null,
  plural_label text not null,
  schema jsonb not null default '[]'::jsonb,
  settings jsonb not null default '{}'::jsonb,
  sort_order int not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (site_id, slug)
);

create index if not exists collections_site_sort_idx
  on collections (site_id, sort_order, created_at);

create trigger collections_set_updated_at before update on collections
  for each row execute function set_updated_at();

create table if not exists collection_entries (
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

create index if not exists collection_entries_site_idx
  on collection_entries (site_id);

create index if not exists collection_entries_status_idx
  on collection_entries (collection_id, status);

create index if not exists collection_entries_sort_idx
  on collection_entries (collection_id, sort_order, created_at);

create trigger collection_entries_set_updated_at before update on collection_entries
  for each row execute function set_updated_at();

alter table pages
  add column if not exists type text not null default 'static'
    check (type in ('static', 'collection_index', 'collection_detail'));

alter table pages
  add column if not exists collection_id uuid references collections(id) on delete restrict;

create index if not exists pages_collection_idx
  on pages (collection_id)
  where collection_id is not null;

alter table block_instances
  add column if not exists bindings jsonb not null default '{}'::jsonb;

-- +goose Down
alter table block_instances drop column if exists bindings;

drop index if exists pages_collection_idx;
alter table pages drop column if exists collection_id;
alter table pages drop column if exists type;

drop trigger if exists collection_entries_set_updated_at on collection_entries;
drop index if exists collection_entries_sort_idx;
drop index if exists collection_entries_status_idx;
drop index if exists collection_entries_site_idx;
drop table if exists collection_entries;

drop trigger if exists collections_set_updated_at on collections;
drop index if exists collections_site_sort_idx;
drop table if exists collections;
