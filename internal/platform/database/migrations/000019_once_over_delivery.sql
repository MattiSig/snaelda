-- +goose Up
alter table once_over_requests
  add column if not exists delivery_next_steps jsonb not null default '[]'::jsonb;

-- +goose Down
alter table once_over_requests
  drop column if exists delivery_next_steps;
