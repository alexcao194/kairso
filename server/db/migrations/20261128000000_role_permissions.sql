-- +goose Up
-- Per-workspace permission matrix for the built-in roles. Keys are "admin" and "member", each a
-- list of permission keys; a missing key means that role keeps the default permissions. The
-- holder always has every permission.
ALTER TABLE workspaces ADD COLUMN role_permissions jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN role_permissions;
