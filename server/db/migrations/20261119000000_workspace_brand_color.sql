-- +goose Up
-- Workspace accent colour as #RRGGBB (NULL = default theme). The web derives the whole UI and
-- PDF palette from this single value.
ALTER TABLE workspaces ADD COLUMN brand_color character varying(7);
ALTER TABLE workspaces ADD CONSTRAINT workspaces_brand_color_check CHECK (brand_color IS NULL OR brand_color ~ '^#[0-9A-Fa-f]{6}$');

-- +goose Down
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_brand_color_check;
ALTER TABLE workspaces DROP COLUMN IF EXISTS brand_color;
