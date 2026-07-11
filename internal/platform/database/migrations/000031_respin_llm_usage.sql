-- +goose Up
-- Durable per-UTC-day LLM spend ledger for the unauthenticated re-spin demo
-- tier (Spec 21 abuse/cost limits). A single rolled-up row per day is enough:
-- the daily budget is a fleet-wide ceiling, incremented atomically as public
-- demo stages spend tokens, and read to pause the public endpoint once the day's
-- cap is reached. Session-bound re-spins are quota-accounted elsewhere and are
-- not recorded here.
create table if not exists respin_llm_usage (
  usage_date date primary key,
  total_tokens bigint not null default 0,
  updated_at timestamptz not null default now()
);

-- +goose Down
drop table if exists respin_llm_usage;
