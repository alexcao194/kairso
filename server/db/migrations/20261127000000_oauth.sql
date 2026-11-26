-- +goose Up
-- OAuth 2.1 for MCP connectors (Claude Desktop, claude.ai): clients register themselves
-- (RFC 7591), users approve them, and each approval becomes an api_keys row whose access
-- token is short-lived and refreshed with a rotating refresh token.
CREATE TABLE oauth_clients (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    redirect_uris text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT oauth_clients_pkey PRIMARY KEY (id)
);

CREATE TABLE oauth_codes (
    code_hash character varying(64) NOT NULL,
    client_id uuid NOT NULL,
    user_id uuid NOT NULL,
    redirect_uri text NOT NULL,
    code_challenge text NOT NULL,
    scope text DEFAULT '' NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT oauth_codes_pkey PRIMARY KEY (code_hash),
    CONSTRAINT oauth_codes_client_id_fkey FOREIGN KEY (client_id) REFERENCES oauth_clients(id) ON DELETE CASCADE,
    CONSTRAINT oauth_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

ALTER TABLE api_keys
    ADD COLUMN client_id uuid,
    ADD COLUMN refresh_hash character varying(64),
    ADD COLUMN refresh_expires_at timestamp with time zone,
    ADD CONSTRAINT api_keys_refresh_hash_key UNIQUE (refresh_hash),
    ADD CONSTRAINT api_keys_client_id_fkey FOREIGN KEY (client_id) REFERENCES oauth_clients(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE api_keys
    DROP CONSTRAINT api_keys_client_id_fkey,
    DROP CONSTRAINT api_keys_refresh_hash_key,
    DROP COLUMN refresh_expires_at,
    DROP COLUMN refresh_hash,
    DROP COLUMN client_id;
DROP TABLE oauth_codes;
DROP TABLE oauth_clients;
