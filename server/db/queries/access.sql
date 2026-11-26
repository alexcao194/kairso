-- name: GetWorkspaceAccess :one
-- The caller's role in a workspace together with the workspace's permission matrix.
SELECT wu.role, w.role_permissions
FROM workspace_users wu
JOIN workspaces w ON w.id = wu.workspace_id
WHERE wu.workspace_id = $1 AND wu.user_id = $2 AND w.deleted_at IS NULL
LIMIT 1;

-- name: GetWorkspaceRolePermissions :one
SELECT role_permissions FROM workspaces WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateWorkspaceRolePermissions :one
UPDATE workspaces SET role_permissions = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING role_permissions;

-- name: ListWorkspaceMemberRoles :many
SELECT wu.user_id, wu.role FROM workspace_users wu WHERE wu.workspace_id = $1;
