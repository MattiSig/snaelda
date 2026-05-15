-- +goose Up
alter table form_submissions add column if not exists client_ip_hash text;
alter table form_submissions add column if not exists spam_signals jsonb;

create index if not exists form_submissions_site_created_idx
  on form_submissions (site_id, created_at desc);

create table if not exists form_submission_attempts (
  id bigserial primary key,
  site_id uuid not null references sites(id) on delete cascade,
  block_id uuid not null,
  client_ip_hash text not null,
  attempted_at timestamptz not null default now()
);

create index if not exists form_submission_attempts_window_idx
  on form_submission_attempts (site_id, block_id, client_ip_hash, attempted_at desc);

create index if not exists form_submission_attempts_cleanup_idx
  on form_submission_attempts (attempted_at);

-- +goose Down
drop index if exists form_submission_attempts_cleanup_idx;
drop index if exists form_submission_attempts_window_idx;
drop table if exists form_submission_attempts;
drop index if exists form_submissions_site_created_idx;
alter table form_submissions drop column if exists spam_signals;
alter table form_submissions drop column if exists client_ip_hash;
