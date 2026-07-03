-- +goose Up
-- Generated pages used to land as status 'draft', and the publish snapshot
-- excludes draft pages — so any site whose pages are ALL drafts (the generated
-- default, never a deliberate per-page opt-out) could not publish at all.
-- Promote those pages so first publish works; sites with at least one
-- published page keep their mix untouched.
update pages
set status = 'published'
where site_id in (
  select site_id
  from pages
  group by site_id
  having bool_and(status = 'draft')
);

-- +goose Down
-- No-op: the pre-migration state (generated default vs deliberate draft)
-- cannot be distinguished after the fact.
select 1;
