"use client";

import { useEffect, useState, type CSSProperties } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { patchWorkspace } from "@/lib/api/endpoints";
import { BRAND_HEX, BRAND_PRESETS, normalizeBrand, previewTokens } from "@/lib/domain/brand";
import { useWorkspace } from "@/lib/workspace/WorkspaceContext";
import { useToast } from "@/lib/toast/ToastProvider";
import { Button } from "@/components/ui/Button";
import { Field, Input } from "@/components/ui/form";
import { tr } from "@/lib/i18n";

const DEFAULT_PICK = "#4F46E5";

/** Workspace brand colour: any colour, previewed live, applied to the UI and to exported PDFs. */
export function BrandColorCard() {
  const { workspace, workspaceId, isManager } = useWorkspace();
  const toast = useToast();
  const qc = useQueryClient();
  const saved = normalizeBrand(workspace?.brandColor);
  const [text, setText] = useState(saved ?? "");
  useEffect(() => setText(saved ?? ""), [saved]);

  const typed = text.trim().toUpperCase();
  const valid = typed === "" || BRAND_HEX.test(typed);
  const draft = typed !== "" && valid ? typed : null;
  const changed = (draft ?? "") !== (saved ?? "");
  const save = useMutation({
    mutationFn: (brandColor: string) => patchWorkspace(workspaceId, { brandColor }),
    onSuccess: () => (qc.invalidateQueries({ queryKey: ["workspaces"] }), toast.success(tr().ops.saved)),
    onError: (e) => toast.error(e),
  });
  if (!workspace) return null;
  const vars = previewTokens(draft) as CSSProperties;

  return (
    <section aria-labelledby="brand-h" className="rounded-lg border border-line bg-surface p-4">
      <h2 id="brand-h" className="text-[14px] font-semibold">{tr().ops.brand}</h2>
      <p className="mb-3 mt-0.5 text-[12.5px] text-ink-muted">{tr().ops.brandDesc}</p>
      <div className="flex flex-wrap items-start gap-5">
        <div className="flex flex-col gap-3">
          <div className="flex items-end gap-3">
            <label className="flex flex-col gap-1">
              <span className="text-[12px] font-medium text-ink-2">{tr().ops.brandPick}</span>
              <input
                type="color"
                disabled={!isManager}
                value={draft ?? saved ?? DEFAULT_PICK}
                onChange={(e) => setText(e.target.value.toUpperCase())}
                className="h-ctl w-14 cursor-pointer rounded-md border border-line-strong bg-surface p-0.5 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </label>
            <Field label={tr().ops.brandHex} error={valid ? undefined : tr().ops.brandInvalid} className="w-[160px]">
              {(p) => <Input {...p} disabled={!isManager} value={text} maxLength={7} placeholder="#4F46E5" onChange={(e) => setText(e.target.value.startsWith("#") || !e.target.value ? e.target.value : `#${e.target.value}`)} />}
            </Field>
          </div>
          <div role="group" aria-label={tr().ops.brandPresets} className="flex flex-wrap gap-1.5">
            {BRAND_PRESETS.map((c) => (
              <button
                key={c}
                type="button"
                disabled={!isManager}
                aria-label={c}
                aria-pressed={draft === c}
                onClick={() => setText(c)}
                style={{ background: c }}
                className={`size-6 rounded-full border border-line-strong disabled:cursor-not-allowed disabled:opacity-50 ${draft === c ? "outline outline-2 outline-offset-2 outline-ink" : ""}`}
              />
            ))}
          </div>
          {isManager && (
            <div className="flex gap-2">
              <Button variant="primary" loading={save.isPending} disabled={!valid || !changed} onClick={() => save.mutate(draft ?? "")}>{tr().ops.save}</Button>
              <Button variant="ghost" disabled={!saved && !text} onClick={() => (setText(""), saved && save.mutate(""))}>{tr().ops.brandReset}</Button>
            </div>
          )}
        </div>

        <div style={vars} className="min-w-[260px] flex-1 rounded-lg border border-line bg-surface-sunken p-3">
          <p className="mb-2 text-[11.5px] font-medium text-ink-muted">{tr().ops.brandPreview}</p>
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex h-ctl items-center rounded-md bg-accent px-3 text-[13px] font-medium text-accent-on">{tr().ops.brandPreviewBtn}</span>
            <span className="inline-flex h-6 items-center rounded-full border border-accent-border bg-accent-soft px-2.5 text-[12px] font-medium text-accent-ink">{tr().ops.brandPreviewTag}</span>
          </div>
          <div className="mt-3 rounded-md px-3 py-2.5 text-[12.5px] font-semibold text-white" style={{ background: "linear-gradient(90deg, var(--doc-brand, #C8102E), var(--doc-brand-2, #E11D2E))" }}>
            {tr().ops.brandPreviewDoc}
          </div>
        </div>
      </div>
    </section>
  );
}
