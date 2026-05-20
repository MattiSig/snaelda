-- +goose Up
alter table sites
  add column if not exists brand jsonb not null default '{}'::jsonb;

-- +goose Down
alter table sites
  drop column if exists brand;
