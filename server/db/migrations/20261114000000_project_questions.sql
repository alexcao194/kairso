-- +goose Up
CREATE TABLE project_questions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    project_id uuid NOT NULL,
    question text NOT NULL,
    created_by_id uuid,
    answer text,
    answered_by_name character varying(150),
    answered_by_company character varying(150),
    answered_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT project_questions_pkey PRIMARY KEY (id),
    CONSTRAINT project_questions_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT project_questions_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_project_questions_project_created ON project_questions USING btree (project_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE TRIGGER update_project_questions_updated_at BEFORE UPDATE ON project_questions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TABLE IF EXISTS project_questions;
