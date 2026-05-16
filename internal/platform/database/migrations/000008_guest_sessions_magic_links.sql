-- +goose Up
create table guest_sessions (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references workspaces(id) on delete cascade,
  cookie_token_hash text not null unique,
  recovery_key_hash text unique,
  prompts_used int not null default 0,
  trial_started_at timestamptz not null default now(),
  trial_expires_at timestamptz not null default (now() + interval '4 days'),
  claimed_by_user_id uuid references users(id) on delete set null,
  claimed_at timestamptz,
  created_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now()
);

create index guest_sessions_workspace_idx
  on guest_sessions (workspace_id);

create index guest_sessions_active_idx
  on guest_sessions (cookie_token_hash, last_seen_at desc);

create table magic_links (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references users(id) on delete cascade,
  token_hash text not null unique,
  purpose text not null check (purpose in ('login', 'verify_email')),
  expires_at timestamptz not null,
  consumed_at timestamptz,
  created_at timestamptz not null default now()
);

create index magic_links_user_idx
  on magic_links (user_id, created_at desc);

-- +goose Down
drop table if exists magic_links;
drop index if exists guest_sessions_active_idx;
drop index if exists guest_sessions_workspace_idx;
drop table if exists guest_sessions;
