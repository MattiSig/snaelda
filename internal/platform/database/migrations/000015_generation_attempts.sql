-- +goose Up
create table if not exists generation_attempts (
  id bigserial primary key,
  workspace_id uuid not null references workspaces(id) on delete cascade,
  user_id uuid,
  scope text not null,
  attempted_at timestamptz not null default now()
);

create index if not exists generation_attempts_workspace_window_idx
  on generation_attempts (workspace_id, scope, attempted_at desc);

create index if not exists generation_attempts_user_window_idx
  on generation_attempts (user_id, scope, attempted_at desc)
  where user_id is not null;

create index if not exists generation_attempts_cleanup_idx
  on generation_attempts (attempted_at);

-- +goose Down
drop index if exists generation_attempts_cleanup_idx;
drop index if exists generation_attempts_user_window_idx;
drop index if exists generation_attempts_workspace_window_idx;
drop table if exists generation_attempts;
