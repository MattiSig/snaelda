-- +goose Up
alter table reprompt_history
  drop constraint if exists reprompt_history_scope_check;

alter table reprompt_history
  add constraint reprompt_history_scope_check
  check (scope in ('site', 'page', 'block', 'collection', 'entry'));

alter table draft_revisions
  drop constraint if exists draft_revisions_scope_check;

alter table draft_revisions
  add constraint draft_revisions_scope_check
  check (scope in ('site', 'page', 'block', 'collection', 'entry'));

-- +goose Down
alter table draft_revisions
  drop constraint if exists draft_revisions_scope_check;

alter table draft_revisions
  add constraint draft_revisions_scope_check
  check (scope in ('site', 'page', 'block'));

alter table reprompt_history
  drop constraint if exists reprompt_history_scope_check;

alter table reprompt_history
  add constraint reprompt_history_scope_check
  check (scope in ('site', 'page', 'block'));
