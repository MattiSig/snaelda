-- +goose Up
-- Give each workspace a content/UI locale (Spec 15/18/22). Iceland-first, so the
-- default is 'is'; constrained to the same allow-list as sites.default_locale
-- ('sv' reserved for the Sweden phase). A fresh column with a valid default keeps
-- every existing row inside the constraint.
alter table workspaces
  add column if not exists locale text not null default 'is';

alter table workspaces
  add constraint workspaces_locale_check
  check (locale in ('is', 'en'));

-- +goose Down
alter table workspaces
  drop constraint if exists workspaces_locale_check;

alter table workspaces
  drop column if exists locale;
