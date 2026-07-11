-- +goose Up
create table if not exists respin_imports (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid references workspaces(id) on delete cascade,
  guest_session_id uuid references guest_sessions(id) on delete set null,
  source_url text not null,
  normalized_url text not null,
  fetch_mode text
    check (fetch_mode in ('plain', 'headless')),
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

create index if not exists respin_imports_normalized_url_idx
  on respin_imports (normalized_url);

create index if not exists respin_imports_workspace_idx
  on respin_imports (workspace_id)
  where workspace_id is not null;

create trigger respin_imports_set_updated_at before update on respin_imports
  for each row execute function set_updated_at();

alter table generation_jobs
  add column if not exists respin_import_id uuid references respin_imports(id) on delete set null;

create index if not exists generation_jobs_respin_import_idx
  on generation_jobs (respin_import_id)
  where respin_import_id is not null;

-- +goose Down
drop index if exists generation_jobs_respin_import_idx;
alter table generation_jobs drop column if exists respin_import_id;

drop trigger if exists respin_imports_set_updated_at on respin_imports;
drop index if exists respin_imports_workspace_idx;
drop index if exists respin_imports_normalized_url_idx;
drop table if exists respin_imports;
