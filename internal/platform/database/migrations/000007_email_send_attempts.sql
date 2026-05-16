-- +goose Up
create table email_send_attempts (
  id uuid primary key default gen_random_uuid(),
  address_hash text not null,
  purpose text not null,
  occurred_at timestamptz not null default now()
);

create index email_send_attempts_addr_idx
  on email_send_attempts (address_hash, purpose, occurred_at desc);

-- +goose Down
drop table if exists email_send_attempts;
