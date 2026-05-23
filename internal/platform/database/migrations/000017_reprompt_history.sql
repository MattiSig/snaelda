-- +goose Up
create table reprompt_history (
  id uuid primary key,
  site_id uuid not null references sites(id) on delete cascade,
  workspace_id uuid not null references workspaces(id) on delete cascade,
  scope text not null check (scope in ('site', 'page')),
  target_id uuid,
  prompt text not null,
  previous_revision_id uuid not null references draft_revisions(id) on delete cascade,
  result_revision_id uuid not null references draft_revisions(id) on delete cascade,
  job_id uuid references generation_jobs(id) on delete set null,
  change_summary text,
  created_by uuid references users(id) on delete set null,
  created_at timestamptz not null default now(),
  undone_at timestamptz
);

create index reprompt_history_site_created_idx
  on reprompt_history (site_id, created_at desc);

create index reprompt_history_site_scope_target_idx
  on reprompt_history (site_id, scope, target_id, created_at desc);

-- +goose Down
drop index if exists reprompt_history_site_scope_target_idx;
drop index if exists reprompt_history_site_created_idx;
drop table if exists reprompt_history;
