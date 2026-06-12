-- +goose Up
alter table generation_jobs
  drop constraint if exists generation_jobs_kind_check;

alter table generation_jobs
  add constraint generation_jobs_kind_check
  check (kind in (
    'site',
    'page_reprompt',
    'site_reprompt',
    'theme_regenerate',
    'block_suggest',
    'collection_draft',
    'entry_draft'
  ));

-- +goose Down
alter table generation_jobs
  drop constraint if exists generation_jobs_kind_check;

alter table generation_jobs
  add constraint generation_jobs_kind_check
  check (kind in ('site', 'page_reprompt', 'site_reprompt', 'theme_regenerate', 'block_suggest'));
