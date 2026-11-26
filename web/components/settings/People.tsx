"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { deleteWorkspaceInvitation, deleteWorkspaceMember, getWorkspaceMembers, getWorkspacePermissions, patchWorkspaceMember, postWorkspaceInvitations, putWorkspacePermissions } from "@/lib/api/endpoints";
import { fetchAll } from "@/lib/query/paged";
import { qk } from "@/lib/query/keys";
import type { WorkspaceRole } from "@/lib/api/schema";
import { canManageRole, ROLE_LABEL, roleRank } from "@/lib/domain/permissions";
import { useWorkspace } from "@/lib/workspace/WorkspaceContext";
import { useAuth } from "@/lib/auth/AuthContext";
import { useToast } from "@/lib/toast/ToastProvider";
import { Avatar } from "@/components/ui/Avatar";
import { Badge, Alert } from "@/components/ui/feedback";
import { Button } from "@/components/ui/Button";
import { Input, Select } from "@/components/ui/form";
import { useConfirm } from "@/components/ui/Confirm";
import { IconTrash } from "@/components/icons";
import { tr } from "@/lib/i18n";

const ROLES: WorkspaceRole[] = ["holder", "admin", "member"];

export function Members() {
  const { workspaceId, role: myRole, can } = useWorkspace();
  const canManageMembers = can("members.manage");
  const all = useAllMembers();
  const { user } = useAuth();
  const toast = useToast();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const refresh = () => qc.invalidateQueries({ queryKey: ["members"] });
  const setRole = useMutation({ mutationFn: (v: { id: string; role: WorkspaceRole }) => patchWorkspaceMember(workspaceId, v.id, { role: v.role }), onSuccess: refresh, onError: (e) => toast.error(e) });
  const remove = useMutation({ mutationFn: (id: string) => deleteWorkspaceMember(workspaceId, id), onSuccess: refresh, onError: (e) => toast.error(e) });
  const members = all.filter((m) => m.status === "accepted");

  return (
    <div className="flex flex-col gap-4">
      <div className="overflow-hidden rounded-lg border border-line bg-surface">
        <ul>
          {members.map((m) => {
            const self = m.userId === user?.id;
            const canManage = canManageRole(myRole, m.role);
            return (
              <li key={m.userId} className="flex min-h-[48px] items-center gap-3 border-b border-line-row px-4 py-1.5 last:border-b-0">
                <Avatar person={{ id: m.userId, name: m.userName, avatar: m.userAvatar }} size="md" />
                <span className="min-w-0 flex-1"><span className="block truncate text-[13px] font-medium">{m.userName}{self && <span className="ml-1.5 text-[11.5px] font-normal text-ink-faint">{tr().people.you}</span>}</span><span className="block truncate text-[11.5px] text-ink-faint">@{m.userAlias}</span></span>
                <Select small aria-label={tr().people.roleOf(m.userName)} className="w-40" disabled={!canManageMembers || self || !canManage} value={m.role} onChange={(e) => setRole.mutate({ id: m.userId, role: e.target.value as WorkspaceRole })}>
                  {ROLES.filter((r) => !myRole || roleRank(myRole) >= roleRank(r)).map((r) => <option key={r} value={r}>{ROLE_LABEL[r]}</option>)}
                </Select>
                {canManageMembers && !self && canManage && (
                  <Button size="sm" variant="ghost" aria-label={tr().people.removeOf(m.userName)} onClick={async () => { if (await confirm({ title: tr().people.confirmRemove(m.userName), message: tr().people.removeDesc, danger: true })) remove.mutate(m.userId); }}><IconTrash size={13} /></Button>
                )}
              </li>
            );
          })}
        </ul>
      </div>
      {canManageMembers ? <InviteBar /> : <Alert>{tr().people.inviteAdminOnly}</Alert>}
    </div>
  );
}

function useAllMembers() {
  const { workspaceId } = useWorkspace();
  return useAllMembersRaw(workspaceId);
}
function useAllMembersRaw(ws: string) {
  const q = useQuery({ queryKey: [...qk.members(ws), "all"], queryFn: () => fetchAll((p) => getWorkspaceMembers(ws, p)), enabled: !!ws });
  return q.data ?? [];
}

function InviteBar() {
  const { workspaceId, role: myRole } = useWorkspace();
  const toast = useToast();
  const qc = useQueryClient();
  const [alias, setAlias] = useState("");
  const [role, setRole] = useState<WorkspaceRole>("member");
  const invite = useMutation({
    mutationFn: () => postWorkspaceInvitations(workspaceId, { userAlias: alias.trim().replace(/^@/, ""), role }),
    onSuccess: () => { toast.success(tr().people.invited); setAlias(""); qc.invalidateQueries({ queryKey: ["members"] }); },
    onError: (e) => toast.error(e),
  });
  return (
    <form onSubmit={(e) => (e.preventDefault(), alias.trim() && invite.mutate())} className="sticky bottom-0 flex flex-wrap items-center gap-2 rounded-lg border border-line bg-surface p-3">
      <label htmlFor="inv-alias" className="sr-only">{tr().people.inviteAlias}</label>
      <Input id="inv-alias" small className="min-w-[200px] flex-1" placeholder={tr().people.invitePh} value={alias} onChange={(e) => setAlias(e.target.value)} />
      <Select small aria-label={tr().people.role} className="w-40" value={role} onChange={(e) => setRole(e.target.value as WorkspaceRole)}>
        {ROLES.filter((r) => !myRole || roleRank(myRole) >= roleRank(r)).map((r) => <option key={r} value={r}>{ROLE_LABEL[r]}</option>)}
      </Select>
      <Button type="submit" variant="primary" loading={invite.isPending} disabled={!alias.trim()}>{tr().people.invite}</Button>
    </form>
  );
}

export function Invites() {
  const { workspaceId, can } = useWorkspace();
  const canManageMembers = can("members.manage");
  const all = useAllMembers();
  const toast = useToast();
  const qc = useQueryClient();
  const cancel = useMutation({ mutationFn: (id: string) => deleteWorkspaceInvitation(workspaceId, id), onSuccess: () => qc.invalidateQueries({ queryKey: ["members"] }), onError: (e) => toast.error(e) });
  const pending = all.filter((m) => m.status === "pending");
  return (
    <div className="flex flex-col gap-4">
      {pending.length === 0 ? <p className="rounded-lg border border-line bg-surface px-4 py-8 text-center text-[13px] text-ink-muted">{tr().people.noInvites}</p> : (
        <ul className="overflow-hidden rounded-lg border border-line bg-surface">
          {pending.map((m) => (
            <li key={m.userId} className="flex min-h-[48px] items-center gap-3 border-b border-line-row px-4 py-1.5 last:border-b-0">
              <Avatar person={{ id: m.userId, name: m.userName, avatar: m.userAvatar }} size="md" />
              <span className="min-w-0 flex-1 text-[13px]"><span className="font-medium">{m.userName}</span> <span className="text-ink-faint">@{m.userAlias}</span></span>
              <Badge tone="warning">{tr().people.pending}</Badge><Badge>{ROLE_LABEL[m.role]}</Badge>
              {canManageMembers && m.invitationId && <Button size="sm" variant="ghost" loading={cancel.isPending} onClick={() => cancel.mutate(m.invitationId!)}>{tr().people.cancelInvite}</Button>}
            </li>
          ))}
        </ul>
      )}
      {canManageMembers && <InviteBar />}
    </div>
  );
}

type EditableRole = "admin" | "member";
const EDITABLE: EditableRole[] = ["member", "admin"];

/** Permission matrix: the holder turns permissions on or off for admins and members. */
export function Roles() {
  const { workspaceId, isHolder } = useWorkspace();
  const qc = useQueryClient();
  const toast = useToast();
  const t = tr().people;
  const q = useQuery({ queryKey: qk.permissions(workspaceId), queryFn: () => getWorkspacePermissions(workspaceId), enabled: !!workspaceId });
  const [draft, setDraft] = useState<Record<EditableRole, string[]> | null>(null);
  const save = useMutation({
    mutationFn: (roles: Record<EditableRole, string[]>) => putWorkspacePermissions(workspaceId, { roles }),
    onSuccess: (data) => {
      qc.setQueryData(qk.permissions(workspaceId), data);
      setDraft(null);
      toast.success(t.permSaved);
    },
    onError: (e) => toast.error(e),
  });

  if (!q.data) return <p className="text-[13px] text-ink-muted">{t.permLoading}</p>;
  const data = q.data;
  const current: Record<EditableRole, string[]> = draft ?? { admin: data.roles.admin ?? [], member: data.roles.member ?? [] };
  const dirty = !!draft;
  const toggle = (role: EditableRole, perm: string, on: boolean) =>
    setDraft({ ...current, [role]: on ? [...current[role], perm] : current[role].filter((p) => p !== perm) });
  const isDefault = EDITABLE.every((r) => [...current[r]].sort().join() === [...(data.defaults[r] ?? [])].sort().join());

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-hidden rounded-lg border border-line bg-surface">
        <table className="w-full text-[13px]">
          <thead className="text-left text-[11px] font-medium uppercase tracking-wide text-ink-faint">
            <tr className="h-thead border-b border-line">
              <th className="px-4">{t.permission}</th>
              <th className="w-28 px-2 text-center">{t.member}</th>
              <th className="w-28 px-2 text-center">{t.admin}</th>
              <th className="w-28 px-2 text-center">{t.holder}</th>
            </tr>
          </thead>
          <tbody>
            {data.permissions.map((perm) => {
              const [label, desc] = t.perms[perm] ?? [perm, ""];
              return (
                <tr key={perm} className="border-b border-line-row last:border-b-0">
                  <td className="px-4 py-2">
                    <p className="font-medium text-ink">{label}</p>
                    {desc && <p className="text-[12px] text-ink-muted">{desc}</p>}
                  </td>
                  {EDITABLE.map((role) => (
                    <td key={role} className="text-center">
                      <input
                        type="checkbox"
                        aria-label={t.permToggle(label, role === "admin" ? t.admin : t.member)}
                        className="size-4 cursor-pointer accent-[var(--accent)] disabled:cursor-default"
                        checked={current[role].includes(perm)}
                        disabled={!isHolder || save.isPending}
                        onChange={(e) => toggle(role, perm, e.target.checked)}
                      />
                    </td>
                  ))}
                  <td className="text-center">
                    <input type="checkbox" aria-label={t.permToggle(label, t.holder)} className="size-4 accent-[var(--accent)]" checked disabled />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        <p className="border-t border-line px-4 py-2 text-[12px] text-ink-muted">{t.permNote}</p>
      </div>
      {isHolder && (
        <div className="flex flex-wrap items-center gap-2">
          <span className="mr-auto text-[12px] text-ink-muted">{t.permEditHint}</span>
          <Button size="sm" variant="ghost" disabled={isDefault || save.isPending} onClick={() => setDraft({ admin: data.defaults.admin ?? [], member: data.defaults.member ?? [] })}>
            {t.permReset}
          </Button>
          <Button size="sm" variant="primary" disabled={!dirty} loading={save.isPending} onClick={() => save.mutate(current)}>
            {t.permSave}
          </Button>
        </div>
      )}
    </div>
  );
}
