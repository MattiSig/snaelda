-- +goose Up
alter table workspaces
  add column if not exists once_over_status text not null default 'none'
    check (once_over_status in ('none', 'awaiting_intake', 'pending', 'delivered'));

create table if not exists once_over_requests (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references workspaces(id) on delete cascade,
  stripe_payment_id text not null unique,
  stripe_checkout_session_id text,
  paid_at timestamptz not null,
  intake_business text,
  intake_visitor text,
  intake_outcome text,
  intake_stuck_on text,
  intake_submitted_at timestamptz,
  video_url text,
  delivered_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists once_over_requests_pending_idx
  on once_over_requests (intake_submitted_at)
  where delivered_at is null and intake_submitted_at is not null;

create trigger once_over_requests_set_updated_at before update on once_over_requests
  for each row execute function set_updated_at();

-- +goose Down
drop trigger if exists once_over_requests_set_updated_at on once_over_requests;
drop index if exists once_over_requests_pending_idx;
drop table if exists once_over_requests;
alter table workspaces
  drop column if exists once_over_status;
