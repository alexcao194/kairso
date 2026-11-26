"use client";

import { useEffect, useMemo, type CSSProperties, type ReactNode } from "react";
import { useSearchParams } from "next/navigation";
import type { MeetingItem, TaskResponse } from "@/lib/api/schema";
import { SECTION_META, SECTION_ORDER, itemAssignment, meetingLead, sectionTitle, toDateInput } from "@/lib/domain/meeting";
import { fmtDate } from "@/lib/domain/format";
import { useLinkedTasks, useMeeting } from "@/lib/hooks/meetings";
import { useWorkspace } from "@/lib/workspace/WorkspaceContext";
import { docTokens } from "@/lib/domain/brand";
import { dateLocale, tr } from "@/lib/i18n";
import { PdfDownloadButton, pageRule } from "@/components/kairso/PrintPaged";
import { FormatSwitch, usePrintOptions } from "@/components/print/AdminDoc";
import { AdminMeetingPrint } from "./AdminMeetingPrint";
import { PrintSignatures } from "@/components/kairso/DocSignatures";

const pdfName = (meetingDate: string) => `Bien-ban-hop-${toDateInput(meetingDate).split("-").reverse().join("-")}`;

export function MeetingPrint({ id }: { id: string }) {
  const sp = useSearchParams();
  const opts = usePrintOptions();
  const q = useMeeting(id);
  const { workspaces } = useWorkspace();
  const m = q.data;
  const taskIds = useMemo(() => [...new Set((m?.items ?? []).flatMap((i) => (i.tasks ?? []).map((t) => t.id)))], [m]);
  const linked = useLinkedTasks(taskIds);
  const ws = workspaces.find((w) => w.id === m?.workspaceId);

  useEffect(() => {
    if (m) document.title = pdfName(m.meetingDate);
  }, [m, sp]);

  if (q.isLoading) return <Center>{tr().meetingPrint.loading}</Center>;
  if (!m) return <Center>{tr().meetingPrint.failed}</Center>;
  const pdf = { path: `/v1/meetings/${m.id}/pdf`, filename: `${pdfName(m.meetingDate)}.pdf` };
  if (opts.format === "admin") return <AdminMeetingPrint m={m} ws={ws} tasks={linked.map} pdf={pdf} />;

  const sections = SECTION_ORDER.map((s) => ({ s, rows: m.items.filter((i) => i.section === s), note: m.sectionNotes?.[s] ?? "" })).filter((x) => x.rows.length || x.note);
  const lead = meetingLead(m);

  return (
    <div className="min-h-screen bg-(--doc-backdrop) py-8 print:bg-(--doc-paper) print:py-0" style={docTokens(ws?.brandColor) as CSSProperties | undefined}>
      <style>{`
        ${pageRule("10mm 0")}
        @media print { html, body { background: var(--doc-paper) !important; } }
        .doc, .doc * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
        .doc tr, .doc .keep { break-inside: avoid; }
      `}</style>

      <div className="mx-auto mb-4 flex w-[210mm] max-w-full items-center justify-end gap-2 px-2 print:hidden">
        <FormatSwitch />
        <span className="mr-auto text-[12.5px] text-(--doc-muted)">{tr().meetingPrint.hint}</span>
        <PdfDownloadButton path={pdf.path} filename={pdf.filename} />
        <button type="button" onClick={() => window.print()} className="h-ctl rounded-md bg-(--doc-brand) px-4 text-[13px] font-medium text-white">
          {tr().meetingPrint.print}
        </button>
      </div>

      <article className="doc mx-auto w-[210mm] max-w-full bg-(--doc-paper) font-sans text-(--doc-ink) shadow-overlay print:shadow-none">
        <header className="flex items-start gap-4 px-[14mm] pb-[4mm] pt-[11mm] print:pt-[2mm]">
          <div className="min-w-0 flex-1">
            <p className="text-[10px] font-semibold uppercase tracking-[0.2em] text-(--doc-brand)">{tr().meetingPrint.title}</p>
            <h1 className="mt-1 text-[21px] font-bold leading-tight">{m.title}</h1>
          </div>
          <div className="grid size-[15mm] shrink-0 place-items-center overflow-hidden rounded-md border border-(--doc-line) bg-(--doc-paper)">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={ws?.avatar || "/icon-192.png"} alt={ws?.name ?? "Logo"} className="size-[12mm] object-contain" />
          </div>
        </header>
        <div className="mx-[14mm] h-[1.5px] bg-(--doc-brand)" />

        <div className="flex flex-col gap-[5mm] px-[14mm] pb-[12mm] pt-[5mm]">
          <div className="keep border-b border-(--doc-line) pb-[4mm]">
            <div className="grid grid-cols-[1fr_1.6fr_1.2fr] gap-5">
              <InfoCard label={tr().meetingPrint.time} value={fmtDate(m.meetingDate)} />
              <InfoCard label={tr().meetingPrint.location} value={m.location} />
              <InfoCard label={tr().meetingPrint.recorders} value={(m.recorders ?? []).map((p) => p.name).join(", ")} />
            </div>
            <div className="mt-2.5">
              <InfoCard label={tr().meetingPrint.attendees} value={(m.attendees ?? []).map((p) => p.name).join(", ")} />
            </div>
          </div>

          {(lead || m.summary) && (
            <Section n={1} title={tr().meetingPrint.general}>
              {lead && <p className="text-[12px] leading-relaxed text-(--doc-ink-2)">{lead}</p>}
              {m.summary && (
                <p className="mt-1.5 whitespace-pre-line text-[12px] leading-relaxed text-(--doc-ink-2)">
                  <b className="text-(--doc-ink)">{tr().meetingPrint.summary}</b> {m.summary}
                </p>
              )}
            </Section>
          )}

          {sections.map(({ s, rows, note }, i) => (
            <Section key={s} n={i + (lead || m.summary ? 2 : 1)} title={sectionTitle(s, rows)}>
              {rows.length > 0 && <ItemsTable section={s} rows={rows} tasks={linked.map} />}
              {note && (
                <p className="keep mt-2.5 whitespace-pre-line border-l-2 border-(--doc-brand) pl-3 text-[12px] text-(--doc-ink-2)">
                  <b className="text-(--doc-ink)">{tr().meetingPrint.note}</b> {note}
                </p>
              )}
            </Section>
          ))}

          <PrintSignatures
            formatDate={(iso) => new Date(iso).toLocaleDateString(dateLocale(), { day: "2-digit", month: "2-digit", year: "numeric" })}
            slots={(["preparer", "controller"] as const).map((role) => ({
              role,
              label: role === "preparer" ? tr().docSign.preparer : tr().docSign.controller,
              hint: tr().docSign.printHint,
              sig: (m.signatures ?? []).find((x) => x.role === role),
            }))}
          />
        </div>
      </article>
    </div>
  );
}

function Center({ children }: { children: ReactNode }) {
  return <div className="grid min-h-screen place-items-center text-[13px] text-ink-muted">{children}</div>;
}

function InfoCard({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[9px] font-semibold uppercase tracking-wider text-(--doc-brand)">{label}</p>
      <p className="mt-0.5 text-[12px] leading-snug text-(--doc-ink)">{value || "—"}</p>
    </div>
  );
}

function Section({ n, title, children }: { n: number; title: string; children: ReactNode }) {
  return (
    <section>
      <h2 className="keep mb-2 flex items-center gap-2.5 text-[14px] font-semibold text-(--doc-ink)" style={{ breakAfter: "avoid" }}>
        <span className="grid size-[6mm] place-items-center rounded-full bg-(--doc-brand) text-[11px] font-semibold text-white">{n}</span>
        {title}
      </h2>
      {children}
    </section>
  );
}

function ItemsTable({ section, rows, tasks }: { section: MeetingItem["section"]; rows: MeetingItem[]; tasks: Map<string, TaskResponse> }) {
  const meta = SECTION_META[section];
  const showStatus = !!meta.status && rows.some((r) => r.status === "done");
  const assign = new Map(rows.map((r) => [r.id, itemAssignment((r.tasks ?? []).map((t) => tasks.get(t.id)))]));
  const showDue = meta.tasks && [...assign.values()].some((a) => a.due);
  const showPriority = meta.tasks;

  return (
    <table className="w-full border-collapse text-[12px]">
      <thead>
        <tr className="bg-(--doc-brand) text-left text-[9.5px] font-semibold uppercase tracking-wide text-white">
          <th className="w-[13mm] px-3 py-1.5">STT</th>
          <th className="w-[34mm] px-3 py-1.5">{tr().meetingPrint.module}</th>
          <th className="px-3 py-1.5">{meta.contentLabel}</th>
          {showDue && <th className="w-[24mm] px-3 py-1.5">{tr().meetingPrint.due}</th>}
          {showStatus && <th className="w-[28mm] px-3 py-1.5">{tr().meetingPrint.status}</th>}
          {showPriority && <th className="w-[22mm] px-3 py-1.5">{tr().meetingPrint.priority}</th>}
        </tr>
      </thead>
      <tbody>
        {rows.map((r, i) => {
          const a = assign.get(r.id)!;
          const codes = (r.tasks ?? []).map((t) => tasks.get(t.id)?.code ?? t.code);
          return (
            <tr key={r.id}>
              <Td className="font-semibold text-(--doc-brand)">{i + 1}</Td>
              <Td className="font-semibold">{r.module}</Td>
              <Td>
                <span className="whitespace-pre-line">{r.content}</span>
                {(r.note || codes.length > 0) && (
                  <span className="mt-0.5 block text-[10.5px] text-(--doc-muted)">
                    {r.note}
                    {r.note && codes.length > 0 && " · "}
                    {codes.length > 0 && <span className="font-mono">{codes.join(", ")}</span>}
                  </span>
                )}
              </Td>
              {showDue && <Td className="whitespace-nowrap text-[12.5px]">{a.due ? fmtDate(a.due) : ""}</Td>}
              {showStatus && <Td className="text-[12.5px]">{meta.status![r.status === "done" ? 1 : 0]}</Td>}
              {showPriority && (
                <Td>
                  {r.priority && <span className="inline-block whitespace-nowrap rounded-full bg-(--doc-brand) px-2 py-0.5 text-[10px] font-semibold text-white">{tr().meetingPrint.priority}</span>}
                </Td>
              )}
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

const Td = ({ children, className = "" }: { children: ReactNode; className?: string }) => (
  <td className={`border-b border-(--doc-line) px-3 py-1.5 align-top leading-snug ${className}`}>{children}</td>
);
