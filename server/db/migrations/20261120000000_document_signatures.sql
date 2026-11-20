-- +goose Up
-- Name-and-date stamps on quotes and meeting minutes. One stamp per (document, role); the name
-- is a snapshot so the document keeps reading the same if the user renames themselves later.
CREATE TABLE document_signatures (
    doc_type text NOT NULL,
    doc_id uuid NOT NULL,
    role text NOT NULL,
    user_id uuid,
    name text NOT NULL,
    signed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT document_signatures_pkey PRIMARY KEY (doc_type, doc_id, role),
    CONSTRAINT document_signatures_type_check CHECK (doc_type IN ('quote', 'meeting')),
    CONSTRAINT document_signatures_role_check CHECK (role IN ('preparer', 'controller')),
    CONSTRAINT document_signatures_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- +goose Down
DROP TABLE IF EXISTS document_signatures;
