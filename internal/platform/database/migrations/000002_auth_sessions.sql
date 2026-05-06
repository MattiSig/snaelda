-- +goose Up
create table auth_sessions (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references users(id) on delete cascade,
  refresh_token_hash text unique not null,
  user_agent text not null default '',
  expires_at timestamptz not null,
  revoked_at timestamptz,
  last_used_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index auth_sessions_user_active_idx
  on auth_sessions (user_id, expires_at)
  where revoked_at is null;

create trigger auth_sessions_set_updated_at before update on auth_sessions
  for each row execute function set_updated_at();

-- +goose Down
drop trigger if exists auth_sessions_set_updated_at on auth_sessions;
drop table if exists auth_sessions;
