-- +goose Up
-- Personal API keys: let scripts and AI assistants (MCP) act as the key's owner.
-- Only a SHA-256 hash of the key is stored; the plain key is shown once at creation.
CREATE TABLE api_keys (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    name text NOT NULL,
    prefix character varying(16) NOT NULL,
    key_hash character varying(64) NOT NULL,
    last_used_at timestamp with time zone,
    expires_at timestamp with time zone,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT api_keys_pkey PRIMARY KEY (id),
    CONSTRAINT api_keys_key_hash_key UNIQUE (key_hash),
    CONSTRAINT api_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_api_keys_user_id ON api_keys USING btree (user_id);

-- +goose Down
DROP TABLE api_keys;
