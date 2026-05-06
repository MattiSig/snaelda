-- +goose Up
-- +goose StatementBegin
create function enforce_site_active_page_limit()
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

-- +goose Down
drop trigger if exists pages_site_active_page_limit on pages;
drop function if exists enforce_site_active_page_limit();
