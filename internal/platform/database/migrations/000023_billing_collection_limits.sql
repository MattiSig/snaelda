-- +goose Up
alter table billing_entitlements
  add column if not exists collection_limit integer,
  add column if not exists collection_entry_limit integer;

-- +goose Down
alter table billing_entitlements
  drop column if exists collection_entry_limit,
  drop column if exists collection_limit;
