"use client";

import { useState } from "react";
import type { ProjectQuestionResponse } from "@/lib/api/schema";
import { useProjectQuestions, useQuestionMutations } from "@/lib/hooks/questions";
import { useToast } from "@/lib/toast/ToastProvider";
import { fmtDate } from "@/lib/domain/format";
import { Button, IconButton } from "@/components/ui/Button";
import { Textarea } from "@/components/ui/form";
import { Badge, EmptyState, ErrorState, SkeletonRows } from "@/components/ui/feedback";
import { useConfirm } from "@/components/ui/Confirm";
import { IconChat, IconEdit, IconPlus, IconTrash } from "@/components/icons";
import { tr } from "@/lib/i18n";

/** Member-side Q&A: write questions that people with the public link answer. */
export function QuestionsPanel({ workspaceId, projectId }: { workspaceId: string; projectId: string }) {
  const toast = useToast();
  const confirm = useConfirm();
  const list = useProjectQuestions(workspaceId, projectId);
  const m = useQuestionMutations(workspaceId, projectId);
  const [draft, setDraft] = useState("");
  const [editing, setEditing] = useState<{ id: string; text: string } | null>(null);
  const items = list.data ?? [];
  const fail = { onError: (e: unknown) => toast.error(e) };

  const add = () => {
    const q = draft.trim();
    if (q) m.create.mutate(q, { ...fail, onSuccess: () => setDraft("") });
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 flex-col gap-2 border-b border-line bg-surface px-4 py-3">
        <Textarea aria-label={tr().fbPage.qaLabel} rows={2} value={draft} maxLength={2000} placeholder={tr().fbPage.qaPh} onChange={(e) => setDraft(e.target.value)} />
        <div className="flex items-center gap-3">
          <span className="min-w-0 flex-1 text-[12px] text-ink-muted">{tr().fbPage.qaPublicHint}</span>
          <span className="shrink-0 font-mono text-[12px] text-ink-muted">{tr().fbPage.qaCount(items.length)}</span>
          <Button size="sm" variant="primary" loading={m.create.isPending} disabled={!draft.trim()} onClick={add}><IconPlus size={13} /> {tr().fbPage.qaAdd}</Button>
        </div>
      </div>
      {list.isLoading ? (
        <SkeletonRows rows={5} height={64} />
      ) : list.isError ? (
        <ErrorState error={list.error} onRetry={() => list.refetch()} />
      ) : items.length === 0 ? (
        <EmptyState icon={<IconChat size={28} />} title={tr().fbPage.qaNone} description={tr().fbPage.qaNoneDesc} className="flex-1" />
      ) : (
        <ul className="min-h-0 flex-1 overflow-y-auto bg-surface">
          {items.map((q: ProjectQuestionResponse) => (
            <li key={q.id} className="border-b border-line-row px-4 py-3">
              {editing?.id === q.id ? (
                <div className="flex flex-col gap-2">
                  <Textarea aria-label={tr().fbPage.qaLabel} rows={2} autoFocus value={editing.text} maxLength={2000} onChange={(e) => setEditing({ id: q.id, text: e.target.value })} />
                  <div className="flex justify-end gap-2">
                    <Button size="sm" onClick={() => setEditing(null)}>{tr().fbPage.qaCancel}</Button>
                    <Button size="sm" variant="primary" loading={m.update.isPending} disabled={!editing.text.trim()} onClick={() => m.update.mutate({ id: q.id, question: editing.text.trim() }, { ...fail, onSuccess: () => setEditing(null) })}>{tr().fbPage.qaSave}</Button>
                  </div>
                </div>
              ) : (
                <div className="flex items-start gap-2">
                  <p className="min-w-0 flex-1 whitespace-pre-wrap break-words text-[13.5px] font-medium text-ink">{q.question}</p>
                  <IconButton label={tr().fbPage.qaEdit} onClick={() => setEditing({ id: q.id, text: q.question })}><IconEdit size={14} /></IconButton>
                  <IconButton
                    label={tr().fbPage.qaDelete}
                    onClick={async () => {
                      if (await confirm({ title: tr().fbPage.qaDeleteTitle, message: tr().fbPage.qaDeleteDesc, confirmLabel: tr().fbPage.qaDelete, danger: true })) m.remove.mutate(q.id, fail);
                    }}
                  ><IconTrash size={14} /></IconButton>
                </div>
              )}
              {q.answer ? (
                <div className="mt-2 rounded-md border-l-2 border-accent bg-accent-soft px-3 py-2 text-[13px]">
                  <p className="whitespace-pre-wrap break-words text-ink-2">{q.answer}</p>
                  <p className="mt-1 flex items-center gap-2 text-[11.5px] text-ink-muted">
                    <span className="min-w-0 flex-1">{tr().fbPage.qaAnsweredBy([q.answeredByName, q.answeredByCompany].filter(Boolean).join(" · "), fmtDate(q.answeredAt))}</span>
                    <Button size="sm" variant="ghost" onClick={async () => {
                      if (await confirm({ title: tr().fbPage.qaClearTitle, message: tr().fbPage.qaClearDesc, confirmLabel: tr().fbPage.qaClear, danger: true })) m.clearAnswer.mutate(q.id, fail);
                    }}>{tr().fbPage.qaClear}</Button>
                  </p>
                </div>
              ) : (
                <div className="mt-1.5"><Badge tone="neutral">{tr().fbPage.qaWaiting}</Badge></div>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
