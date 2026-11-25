-- +goose Up
-- Versions of a quote share the root's number; the version lives in its own column instead of
-- a "-vN" suffix on the number.
ALTER TABLE quotes DROP CONSTRAINT quotes_workspace_number_key;
UPDATE quotes q SET number = r.number
FROM quotes r
WHERE q.root_id = r.id AND q.number <> r.number;
ALTER TABLE quotes ADD CONSTRAINT quotes_workspace_number_version_key UNIQUE (workspace_id, number, version)
    DEFERRABLE INITIALLY DEFERRED; -- NewVersion inserts as v1, then sets the version in the same tx

-- +goose Down
ALTER TABLE quotes DROP CONSTRAINT IF EXISTS quotes_workspace_number_version_key;
UPDATE quotes SET number = number || '-v' || version WHERE root_id IS NOT NULL;
ALTER TABLE quotes ADD CONSTRAINT quotes_workspace_number_key UNIQUE (workspace_id, number);
