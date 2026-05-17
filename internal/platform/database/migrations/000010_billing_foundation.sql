-- +goose Up
alter table workspaces
  add column if not exists plan text not null default 'trial',
  add column if not exists stripe_customer_id text unique;

create table billing_customers (
  workspace_id uuid primary key references workspaces(id) on delete cascade,
  stripe_customer_id text not null unique,
  email text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table billing_subscriptions (
  id uuid primary key default gen_random_uuid(),
  workspace_id uuid not null references workspaces(id) on delete cascade,
  stripe_customer_id text not null,
  stripe_subscription_id text not null unique,
  plan text not null,
  status text not null,
  price_id text,
  product_id text,
  current_period_start timestamptz,
  current_period_end timestamptz,
  cancel_at_period_end boolean not null default false,
  canceled_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index billing_subscriptions_workspace_idx
  on billing_subscriptions (workspace_id, updated_at desc);

create table billing_entitlements (
  workspace_id uuid primary key references workspaces(id) on delete cascade,
  plan text not null,
  status text not null,
  subscription_live boolean not null default false,
  custom_domains_enabled boolean not null default false,
  active_site_limit integer,
  monthly_prompt_limit integer,
  asset_storage_limit_bytes bigint,
  updated_at timestamptz not null default now()
);

create table billing_events (
  stripe_event_id text primary key,
  event_type text not null,
  payload jsonb not null,
  processed_at timestamptz not null default now()
);

create trigger billing_customers_set_updated_at before update on billing_customers
  for each row execute function set_updated_at();
create trigger billing_subscriptions_set_updated_at before update on billing_subscriptions
  for each row execute function set_updated_at();
create trigger billing_entitlements_set_updated_at before update on billing_entitlements
  for each row execute function set_updated_at();

-- +goose Down
drop trigger if exists billing_entitlements_set_updated_at on billing_entitlements;
drop trigger if exists billing_subscriptions_set_updated_at on billing_subscriptions;
drop trigger if exists billing_customers_set_updated_at on billing_customers;
drop table if exists billing_events;
drop table if exists billing_entitlements;
drop index if exists billing_subscriptions_workspace_idx;
drop table if exists billing_subscriptions;
drop table if exists billing_customers;
alter table workspaces
  drop column if exists stripe_customer_id,
  drop column if exists plan;
