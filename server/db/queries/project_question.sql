-- name: CreateProjectQuestion :one
INSERT INTO project_questions (project_id, question, created_by_id)
VALUES (sqlc.arg(project_id), sqlc.arg(question), sqlc.arg(created_by_id))
RETURNING *;

-- name: ListProjectQuestions :many
SELECT *
FROM project_questions
WHERE project_id = sqlc.arg(project_id) AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetProjectQuestionByID :one
SELECT *
FROM project_questions
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: UpdateProjectQuestionText :one
UPDATE project_questions SET question = sqlc.arg(question)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: ClearProjectQuestionAnswer :one
UPDATE project_questions
SET answer = NULL, answered_by_name = NULL, answered_by_company = NULL, answered_at = NULL
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: DeleteProjectQuestion :exec
UPDATE project_questions SET deleted_at = NOW()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- Public answer: only succeeds while the question is still unanswered, so a
-- link holder cannot overwrite an earlier answer.
-- name: AnswerProjectQuestion :one
UPDATE project_questions
SET answer = sqlc.arg(answer), answered_by_name = sqlc.arg(answered_by_name),
    answered_by_company = sqlc.arg(answered_by_company), answered_at = NOW()
WHERE id = sqlc.arg(id) AND project_id = sqlc.arg(project_id) AND deleted_at IS NULL AND answer IS NULL
RETURNING *;
