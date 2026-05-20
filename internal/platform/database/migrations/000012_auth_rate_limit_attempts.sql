-- +goose Up
create table if not exists auth_rate_limit_attempts (
  id bigserial primary key,
  purpose text not null,
  key_hash text not null,
  attempted_at timestamptz not null default now()
);

create index if not exists auth_rate_limit_attempts_window_idx
  on auth_rate_limit_attempts (purpose, key_hash, attempted_at desc);

create index if not exists auth_rate_limit_attempts_cleanup_idx
  on auth_rate_limit_attempts (attempted_at);

-- +goose Down
drop index if exists auth_rate_limit_attempts_cleanup_idx;
drop index if exists auth_rate_limit_attempts_window_idx;
drop table if exists auth_rate_limit_attempts;
