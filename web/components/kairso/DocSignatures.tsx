"use client";

import type { ReactNode } from "react";
import type { Signature } from "@/lib/api/schema";
import { fmtDate } from "@/lib/domain/format";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/Button";
import { IconCheck } from "@/components/icons";

export type SignRole = Signature["role"];

/** Name + date stamp as it prints on a document (uses the document colour tokens). */
export function DocStamp({ name, date }: { name: string; date: string }) {
  return (
    <div
      className="inline-flex -rotate-2 flex-col items-center rounded-md border-2 border-(--doc-brand) px-4 py-1.5 text-(--doc-brand)"
      style={{ boxShadow: "inset 0 0 0 1px var(--doc-paper), inset 0 0 0 2px var(--doc-brand)" }}
    >
      <span className="text-[13px] font-bold uppercase leading-tight tracking-wide">{name}</span>
      <span className="mt-0.5 font-mono text-[10.5px] leading-tight">{date}</span>
    </div>
  );
}

export interface PrintSlot {
  role: SignRole;
  label: string;
  hint: string;
  sig?: Signature;
  /** Extra content above the stamp (e.g. an uploaded signature image). */
  above?: ReactNode;
}

/** Signature row at the end of a printed document: one column per role, stamped when signed. */
export function PrintSignatures({ slots, formatDate }: { slots: PrintSlot[]; formatDate: (iso: string) => string }) {
  return (
    <div className="keep mt-[5mm] flex justify-end gap-10 text-center text-[12px]">
      {slots.map((s) => (
        <div key={s.role} className="w-[58mm]">
          <p className="font-semibold">{s.label}</p>
          <p className="text-[10.5px] italic text-(--doc-muted)">{s.hint}</p>
          <div className="grid h-[24mm] place-items-center">
            <div className="flex flex-col items-center gap-1">
              {s.above}
              {s.sig && <DocStamp name={s.sig.name} date={formatDate(s.sig.signedAt)} />}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export interface EditorSlot {
  role: SignRole;
  label: string;
  sig?: Signature;
  /** Why the caller cannot sign this slot (shown instead of the button); empty = they can. */
  blocked?: string;
  /** Caller may remove the existing stamp. */
  canRemove?: boolean;
}

/** In-app card to sign or remove the stamps on a document. */
export function SignaturesPanel({
  slots,
  hint,
  busy,
  onSign,
  onUnsign,
}: {
  slots: EditorSlot[];
  hint?: string;
  busy?: boolean;
  onSign: (role: SignRole) => void;
  onUnsign: (role: SignRole) => void;
}) {
  const t = useT();
  return (
    <section className="rounded-lg border border-line bg-surface">
      <div className="flex items-baseline gap-3 border-b border-line px-4 py-2.5">
        <h2 className="text-[13px] font-semibold">{t.docSign.title}</h2>
        {hint && <p className="text-[12px] text-ink-muted">{hint}</p>}
      </div>
      <div className={`grid gap-px bg-line ${slots.length > 1 ? "md:grid-cols-2" : ""}`}>
        {slots.map((s) => (
          <div key={s.role} className="flex min-h-[88px] flex-col gap-2 bg-surface p-4">
            <p className="text-[11px] font-semibold uppercase tracking-wide text-ink-muted">{s.label}</p>
            {s.sig ? (
              <div className="flex flex-wrap items-center gap-3">
                <span className="grid size-7 shrink-0 place-items-center rounded-full bg-success-bg text-success"><IconCheck size={14} /></span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[14px] font-semibold">{s.sig.name}</p>
                  <p className="text-[12px] text-ink-muted">{t.docSign.signedOn(fmtDate(s.sig.signedAt))}</p>
                </div>
                {s.canRemove && (
                  <Button size="sm" variant="ghost" disabled={busy} onClick={() => onUnsign(s.role)}>{t.docSign.unsign}</Button>
                )}
              </div>
            ) : (
              <div className="flex flex-wrap items-center gap-3">
                <p className="flex-1 text-[13px] text-ink-faint">{s.blocked || t.docSign.unsigned}</p>
                {!s.blocked && <Button size="sm" variant="primary" loading={busy} onClick={() => onSign(s.role)}>{t.docSign.sign}</Button>}
              </div>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}
