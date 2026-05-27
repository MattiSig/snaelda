-- +goose Up
-- +goose StatementBegin
do $$
declare
  reprompt_scope_constraint text;
  revisions_scope_constraint text;
  kind_constraint text;
begin
  select conname into reprompt_scope_constraint
  from pg_constraint
  where conrelid = 'reprompt_history'::regclass
    and contype = 'c'
    and pg_get_constraintdef(oid) ilike '%scope%';
  if reprompt_scope_constraint is not null then
    execute format('alter table reprompt_history drop constraint %I', reprompt_scope_constraint);
  end if;

  select conname into revisions_scope_constraint
  from pg_constraint
  where conrelid = 'draft_revisions'::regclass
    and contype = 'c'
    and pg_get_constraintdef(oid) ilike '%scope%';
  if revisions_scope_constraint is not null then
    execute format('alter table draft_revisions drop constraint %I', revisions_scope_constraint);
  end if;

  select conname into kind_constraint
  from pg_constraint
  where conrelid = 'generation_jobs'::regclass
    and contype = 'c'
    and pg_get_constraintdef(oid) ilike '%kind%';
  if kind_constraint is not null then
    execute format('alter table generation_jobs drop constraint %I', kind_constraint);
  end if;
end
$$;
-- +goose StatementEnd

alter table reprompt_history
  add constraint reprompt_history_scope_check
  check (scope in ('site', 'page', 'block'));

alter table draft_revisions
  add constraint draft_revisions_scope_check
  check (scope in ('site', 'page', 'block'));

alter table generation_jobs
  add constraint generation_jobs_kind_check
  check (kind in ('site', 'page_reprompt', 'site_reprompt', 'theme_regenerate', 'block_suggest'));

-- +goose Down
alter table generation_jobs
  drop constraint if exists generation_jobs_kind_check;

alter table generation_jobs
  add constraint generation_jobs_kind_check
  check (kind in ('site', 'page_reprompt', 'site_reprompt', 'theme_regenerate'));

alter table draft_revisions
  drop constraint if exists draft_revisions_scope_check;

alter table draft_revisions
  add constraint draft_revisions_scope_check
  check (scope in ('site', 'page'));

alter table reprompt_history
  drop constraint if exists reprompt_history_scope_check;

alter table reprompt_history
  add constraint reprompt_history_scope_check
  check (scope in ('site', 'page'));
