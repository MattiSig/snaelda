-- +goose Up
alter table pages
  add column if not exists in_draft boolean not null default true;

alter table pages
  drop constraint if exists pages_site_id_slug_key;

drop index if exists pages_site_draft_slug_idx;

create unique index if not exists pages_site_draft_slug_idx
  on pages (site_id, slug)
  where in_draft;

-- +goose StatementBegin
create or replace function enforce_site_active_page_limit()
returns trigger as $$
declare
  active_page_count int;
begin
  if new.status = 'archived' or not coalesce(new.in_draft, true) then
    return new;
  end if;

  select count(*) into active_page_count
  from pages
  where site_id = new.site_id
    and coalesce(in_draft, true)
    and status <> 'archived';

  if active_page_count > 10 then
    raise exception 'site % cannot have more than 10 active pages', new.site_id
      using errcode = '23514',
            constraint = 'pages_site_active_page_limit';
  end if;

  return new;
end;
$$ language plpgsql;
-- +goose StatementEnd

drop trigger if exists pages_site_active_page_limit on pages;

create constraint trigger pages_site_active_page_limit
after insert or update of site_id, status, in_draft on pages
deferrable initially immediate
for each row execute function enforce_site_active_page_limit();

-- +goose Down
drop trigger if exists pages_site_active_page_limit on pages;

-- +goose StatementBegin
create or replace function enforce_site_active_page_limit()
returns trigger as $$
declare
  active_page_count int;
begin
  if new.status = 'archived' then
    return new;
  end if;

  select count(*) into active_page_count
  from pages
  where site_id = new.site_id
    and status <> 'archived';

  if active_page_count > 10 then
    raise exception 'site % cannot have more than 10 active pages', new.site_id
      using errcode = '23514',
            constraint = 'pages_site_active_page_limit';
  end if;

  return new;
end;
$$ language plpgsql;
-- +goose StatementEnd

create constraint trigger pages_site_active_page_limit
after insert or update of site_id, status on pages
deferrable initially immediate
for each row execute function enforce_site_active_page_limit();

drop index if exists pages_site_draft_slug_idx;

alter table pages
  add constraint pages_site_id_slug_key unique (site_id, slug);

alter table pages
  drop column if exists in_draft;
