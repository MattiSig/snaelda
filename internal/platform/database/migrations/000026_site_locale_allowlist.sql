-- +goose Up
-- Constrain sites.default_locale to the Spec 22 content-locale allow-list
-- ('is', 'en'; 'sv' reserved for the Sweden phase). Coerce any legacy value
-- outside the allow-list to 'en' first so the constraint can be added safely.
update sites
set default_locale = 'en'
where default_locale not in ('is', 'en');

alter table sites
  add constraint sites_default_locale_check
  check (default_locale in ('is', 'en'));

-- +goose Down
alter table sites
  drop constraint if exists sites_default_locale_check;
