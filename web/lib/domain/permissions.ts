import type { WorkspaceRole, ProjectRole } from "../api/schema";
import { tr } from "../i18n/dict";
import { liveLabels } from "../i18n/live";

export const ROLE_LABEL: Record<WorkspaceRole | ProjectRole, string> = liveLabels(() => tr().roles);

const ROLE_RANK: Record<WorkspaceRole | ProjectRole, number> = { holder: 3, admin: 2, member: 1 };
export const roleRank = (role: WorkspaceRole | ProjectRole) => ROLE_RANK[role];

export const canManageRole = (
  myRole: WorkspaceRole | ProjectRole | null | undefined,
  targetRole: WorkspaceRole | ProjectRole,
) => !!myRole && roleRank(myRole) >= roleRank(targetRole);

/** Workspace permissions; the server (`internal/features/access`) is the source of truth. */
export type Permission =
  | "workspace.settings"
  | "members.manage"
  | "attendance.manage"
  | "attendance.view_all"
  | "quotes.manage"
  | "meetings.manage"
  | "files.manage"
  | "webhooks.manage"
  | "performance.manage"
  | "kpi.settings";

/** Permissions a role has when the workspace never changed them (used until the matrix loads). */
export function defaultPermissions(role: WorkspaceRole | null | undefined): Permission[] {
  const all: Permission[] = [
    "workspace.settings", "members.manage", "attendance.manage", "attendance.view_all", "quotes.manage",
    "meetings.manage", "files.manage", "webhooks.manage", "performance.manage", "kpi.settings",
  ];
  if (role === "holder") return all;
  if (role === "admin") return all.filter((p) => p !== "kpi.settings");
  return [];
}
