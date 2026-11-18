-- +goose Up
-- A quote revision chain: every version points at the first quote (root_id, NULL on the root
-- itself) and carries its own version number. Numbers stay unique via a "-vN" suffix.
ALTER TABLE quotes ADD COLUMN root_id uuid;
ALTER TABLE quotes ADD COLUMN version integer DEFAULT 1 NOT NULL;
ALTER TABLE quotes ADD CONSTRAINT quotes_root_id_fkey FOREIGN KEY (root_id) REFERENCES quotes(id) ON DELETE SET NULL;
CREATE INDEX idx_quotes_root_id ON quotes USING btree (root_id) WHERE root_id IS NOT NULL;
CREATE UNIQUE INDEX uq_quotes_root_version ON quotes USING btree (root_id, version) WHERE root_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uq_quotes_root_version;
DROP INDEX IF EXISTS idx_quotes_root_id;
ALTER TABLE quotes DROP CONSTRAINT IF EXISTS quotes_root_id_fkey;
ALTER TABLE quotes DROP COLUMN IF EXISTS version;
ALTER TABLE quotes DROP COLUMN IF EXISTS root_id;
