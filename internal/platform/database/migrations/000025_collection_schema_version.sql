-- +goose Up
alter table collections
  add column if not exists schema_version int not null default 1;

update collections
set schema_version = 1
where schema_version < 1;

-- +goose Down
alter table collections
  drop column if exists schema_version;
