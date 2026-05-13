-- +goose Up
create table draft_revisions (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  workspace_id uuid not null references workspaces(id) on delete cascade,
  scope text not null check (scope in ('site', 'page')),
  page_id uuid references pages(id) on delete set null,
  prompt text not null default '',
  draft jsonb not null,
  generation_prompt text not null default '',
  generation_summary jsonb not null default '{}'::jsonb,
  created_by uuid references users(id) on delete set null,
  created_at timestamptz not null default now()
);

create index draft_revisions_site_created_idx
  on draft_revisions (site_id, created_at desc);

create index draft_revisions_workspace_created_idx
  on draft_revisions (workspace_id, created_at desc);

-- +goose Down
drop index if exists draft_revisions_workspace_created_idx;
drop index if exists draft_revisions_site_created_idx;
drop table if exists draft_revisions;
