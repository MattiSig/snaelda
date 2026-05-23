-- +goose Up
alter table generation_jobs
  add column if not exists kind text not null default 'site'
    check (kind in ('site', 'page_reprompt', 'site_reprompt', 'theme_regenerate')),
  add column if not exists state text not null default 'pending'
    check (state in ('pending', 'running', 'succeeded', 'failed', 'canceled')),
  add column if not exists current_step text,
  add column if not exists error_reason text,
  add column if not exists started_at timestamptz,
  add column if not exists completed_at timestamptz,
  add column if not exists payload jsonb not null default '{}'::jsonb;

update generation_jobs
set kind = case
      when coalesce(input_context ->> 'scope', '') = 'page' then 'page_reprompt'
      when coalesce(input_context ->> 'scope', '') = 'site' then 'site_reprompt'
      else 'site'
    end,
    state = case
      when status = 'completed' then 'succeeded'
      when status = 'failed' then 'failed'
      when status = 'running' then 'running'
      else 'pending'
    end,
    payload = case
      when payload = '{}'::jsonb then input_context
      else payload
    end,
    current_step = case
      when status = 'completed' then coalesce(current_step, 'persist')
      else current_step
    end,
    error_reason = case
      when error_reason is not null then error_reason
      when error ? 'reason' then error ->> 'reason'
      else null
    end,
    started_at = coalesce(started_at, created_at),
    completed_at = case
      when completed_at is not null then completed_at
      when status in ('completed', 'failed') then updated_at
      else null
    end;

create index if not exists generation_jobs_workspace_state_idx
  on generation_jobs (workspace_id, state);

create index if not exists generation_jobs_started_idx
  on generation_jobs (started_at);

-- +goose Down
drop index if exists generation_jobs_started_idx;
drop index if exists generation_jobs_workspace_state_idx;

alter table generation_jobs
  drop column if exists payload,
  drop column if exists completed_at,
  drop column if exists started_at,
  drop column if exists error_reason,
  drop column if exists current_step,
  drop column if exists state,
  drop column if exists kind;
