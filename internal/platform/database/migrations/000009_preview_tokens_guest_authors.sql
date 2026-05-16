-- +goose Up
alter table site_preview_tokens
  alter column created_by drop not null;

-- +goose Down
alter table site_preview_tokens
  alter column created_by set not null;
