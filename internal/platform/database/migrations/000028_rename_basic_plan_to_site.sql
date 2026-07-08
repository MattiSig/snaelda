-- +goose Up
-- Rename the legacy 'basic' subscription plan to 'site' (Spec 15 ISK-first
-- catalog). The plan string is stored free-text on three tables; coerce every
-- persisted 'basic' value so entitlement/limit lookups keyed on the catalog id
-- keep resolving after the rename. New rows already write 'site'.
update workspaces set plan = 'site' where plan = 'basic';
update billing_subscriptions set plan = 'site' where plan = 'basic';
update billing_entitlements set plan = 'site' where plan = 'basic';

-- +goose Down
update workspaces set plan = 'basic' where plan = 'site';
update billing_subscriptions set plan = 'basic' where plan = 'site';
update billing_entitlements set plan = 'basic' where plan = 'site';
