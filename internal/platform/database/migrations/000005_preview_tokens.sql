-- +goose Up
create table site_preview_tokens (
  id uuid primary key default gen_random_uuid(),
  site_id uuid not null references sites(id) on delete cascade,
  created_by uuid not null references users(id) on delete cascade,
  token_hash text not null unique,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  last_used_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index site_preview_tokens_site_active_idx
  on site_preview_tokens (site_id, expires_at)
  where revoked_at is null;

create trigger site_preview_tokens_set_updated_at before update on site_preview_tokens
  for each row execute function set_updated_at();

-- +goose Down
drop trigger if exists site_preview_tokens_set_updated_at on site_preview_tokens;
drop table if exists site_preview_tokens;
