-- +goose Up
alter table sites
  add column if not exists draft_revision bigint not null default 1;

update sites
set draft_revision = 1
where draft_revision < 1;

alter table sites
  alter column draft_revision set default 1;

drop index if exists sites_workspace_draft_revision_idx;

create index if not exists sites_workspace_draft_revision_idx
  on sites (workspace_id, draft_revision);

-- +goose Down
drop index if exists sites_workspace_draft_revision_idx;

alter table sites
  drop column if exists draft_revision;
