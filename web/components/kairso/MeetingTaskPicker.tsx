"use client";

import { useDeferredValue, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getWorkspaceProjectTasks } from "@/lib/api/endpoints";
import type { MeetingTaskRef, TaskResponse } from "@/lib/api/schema";
import { useWorkspace } from "@/lib/workspace/WorkspaceContext";
import { Modal } from "@/components/ui/Modal";
import { SearchInput, Select } from "@/components/ui/form";
import { Button } from "@/components/ui/Button";
import { EmptyState, ErrorState, SkeletonRows } from "@/components/ui/feedback";
import { StatusChip } from "@/components/kairso/StatusChip";
import { IconCheck } from "@/components/icons";
import { tr } from "@/lib/i18n";

/** Search the project's existing tasks and link them to a meeting item (applied to the draft; saved with the minutes). */
export function MeetingTaskPicker({ open, ws, projectId, linked, onPick, onClose }: { open: boolean; ws: string; projectId?: string; linked: MeetingTaskRef[]; onPick: (t: TaskResponse) => void; onClose: () => void }) {
  const { projects } = useWorkspace();
  const [chosen, setChosen] = useState("");
  const [q, setQ] = useState("");
  const dq = useDeferredValue(q.trim());
  const pid = projectId || chosen;
  const list = useQuery({
    queryKey: ["tasks", ws, pid, { meetingPicker: dq }],
    queryFn: async () => (await getWorkspaceProjectTasks(ws, pid, { q: dq || undefined, limit: 30, page: 1 })).items ?? [],
    enabled: open && !!ws && !!pid,
  });
  const done = new Set(linked.map((t) => t.id));

  return (
    <Modal open={open} onClose={onClose} title={tr().meetingEd.pickTitle} description={tr().meetingEd.pickDesc} width={560} footer={<Button onClick={onClose}>{tr().meetingEd.pickDone}</Button>}>
      <div className="flex flex-col gap-3">
        {!projectId && (
          <Select aria-label={tr().meetingEd.pickProject} value={chosen} onChange={(e) => setChosen(e.target.value)}>
            <option value="">{tr().meetingEd.pickProject}</option>
            {projects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </Select>
        )}
        <SearchInput value={q} onChange={setQ} placeholder={tr().meetingEd.pickSearchPh} label={tr().meetingEd.pickSearch} className="w-full" />
        {!pid ? null : list.isLoading ? (
          <SkeletonRows rows={5} height={40} />
        ) : list.isError ? (
          <ErrorState error={list.error} onRetry={() => list.refetch()} />
        ) : (list.data ?? []).length === 0 ? (
          <EmptyState title={tr().meetingEd.pickNone} className="py-6" />
        ) : (
          <ul className="max-h-[360px] overflow-y-auto rounded-md border border-line">
            {(list.data ?? []).map((t) => {
              const added = done.has(t.id);
              return (
                <li key={t.id} className="border-b border-line-row last:border-b-0">
                  <button type="button" disabled={added} onClick={() => onPick(t)} className="flex min-h-[40px] w-full items-center gap-2 px-3 py-1.5 text-left hover:bg-row-hover disabled:cursor-default disabled:opacity-60">
                    <span className="w-[76px] shrink-0 font-mono text-[12px] text-ink-faint">{t.code}</span>
                    <span className="min-w-0 flex-1 truncate text-[13px] text-ink">{t.title}</span>
                    <StatusChip status={t.status} size="sm" />
                    {added && <span className="flex items-center gap-1 text-[11.5px] text-success"><IconCheck size={12} /> {tr().meetingEd.pickAdded}</span>}
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </Modal>
  );
}
