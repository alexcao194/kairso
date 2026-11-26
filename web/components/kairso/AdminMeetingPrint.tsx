"use client";

import type { MeetingItem, MeetingMinutesResponse, TaskResponse, WorkspaceResponse } from "@/lib/api/schema";
import { SECTION_META, SECTION_ORDER, itemAssignment, meetingLead, sectionTitle } from "@/lib/domain/meeting";
import { fmtDate } from "@/lib/domain/format";
import { tr } from "@/lib/i18n";
import { A, AdminHeader, AdminShell, AdminSignatures, Cell, usePrintOptions, type PdfTarget } from "@/components/print/AdminDoc";

const ROMAN = ["I", "II", "III", "IV", "V", "VI", "VII", "VIII"];

function meetingDay(d: string) {
  const [y, m, day] = d.slice(0, 10).split("-").map(Number);
  return new Date(y, m - 1, day);
}

/** Meeting minutes laid out as a Vietnamese administrative "biên bản", for printing, signing and sealing. */
export function AdminMeetingPrint({ m, ws, tasks, pdf }: { m: MeetingMinutesResponse; ws?: WorkspaceResponse; tasks: Map<string, TaskResponse>; pdf: PdfTarget }) {
  const { place } = usePrintOptions();
  const a = A();
  const sigs = m.signatures ?? [];
  const sections = SECTION_ORDER.map((s) => ({ s, rows: m.items.filter((i) => i.section === s), note: m.sectionNotes?.[s] ?? "" })).filter((x) => x.rows.length || x.note);
  const lead = meetingLead(m);
  const names = (ps: MeetingMinutesResponse["attendees"]) => (ps ?? []).map((p) => p.name).join(", ");

  return (
    <AdminShell brandColor={ws?.brandColor} defaultPlace="" unsigned={sigs.length < 2} printLabel={tr().meetingPrint.print} pdf={pdf}>
      <AdminHeader org={ws?.name ?? ""} number="" date={meetingDay(m.meetingDate)} place={place ?? ""} />

      <div className="keep mt-[8mm] text-center">
        <p className="text-[14pt] font-bold">{a.minutesTitle}</p>
        <p className="mt-0.5 font-bold">{m.title}</p>
      </div>

      <div className="mt-[6mm] flex flex-col gap-1 text-justify">
        <Heading n={0}>{a.partTime}</Heading>
        <p>- {a.time}: {fmtDate(m.meetingDate)}</p>
        <p>- {a.location}: {m.location || "……………………"}</p>

        <Heading n={1}>{a.partPeople}</Heading>
        <p>- {names(m.attendees) || "……………………"}</p>
        <p>- {a.secretary}: {names(m.recorders) || "……………………"}</p>

        <Heading n={2}>{a.partContent}</Heading>
        {lead && <p className="indent-[1cm]">{lead}</p>}
        {m.summary && (
          <p className="whitespace-pre-line indent-[1cm]">
            <b>{a.summary}:</b> {m.summary}
          </p>
        )}
        {sections.map(({ s, rows, note }, i) => (
          <div key={s} className="mt-1">
            <p className="keep font-bold" style={{ breakAfter: "avoid" }}>
              {i + 1}. {sectionTitle(s, rows)}
            </p>
            {rows.length > 0 && <ItemsTable section={s} rows={rows} tasks={tasks} />}
            {note && (
              <p className="mt-1 whitespace-pre-line">
                <i>{a.note}:</i> {note}
              </p>
            )}
          </div>
        ))}

        <p className="keep mt-2 indent-[1cm]">{a.minutesClosing}</p>
      </div>

      <AdminSignatures
        slots={[
          { title: a.minutesPreparer, hint: a.hint, sig: sigs.find((x) => x.role === "preparer") },
          { title: a.minutesController, hint: a.hintSeal, sig: sigs.find((x) => x.role === "controller") },
        ]}
      />
    </AdminShell>
  );
}

function Heading({ n, children }: { n: number; children: string }) {
  return (
    <p className="keep mt-2 font-bold" style={{ breakAfter: "avoid" }}>
      {ROMAN[n]}. {children.toUpperCase()}
    </p>
  );
}

function ItemsTable({ section, rows, tasks }: { section: MeetingItem["section"]; rows: MeetingItem[]; tasks: Map<string, TaskResponse> }) {
  const a = A();
  const meta = SECTION_META[section];
  const showStatus = !!meta.status && rows.some((r) => r.status === "done");
  const assign = new Map(rows.map((r) => [r.id, itemAssignment((r.tasks ?? []).map((t) => tasks.get(t.id)))]));
  const showDue = meta.tasks && [...assign.values()].some((x) => x.due);
  const showPriority = meta.tasks && rows.some((r) => r.priority);
  return (
    <table className="mt-1 w-full border-collapse">
      <thead>
        <tr>
          <Cell head className="w-[10mm]">STT</Cell>
          <Cell head className="w-[32mm]">{a.module}</Cell>
          <Cell head>{meta.contentLabel}</Cell>
          {showDue && <Cell head className="w-[24mm]">{a.due}</Cell>}
          {showStatus && <Cell head className="w-[26mm]">{a.status}</Cell>}
          {showPriority && <Cell head className="w-[18mm]">{a.priority}</Cell>}
        </tr>
      </thead>
      <tbody>
        {rows.map((r, i) => {
          const x = assign.get(r.id)!;
          return (
            <tr key={r.id}>
              <Cell className="text-center">{i + 1}</Cell>
              <Cell>{r.module}</Cell>
              <Cell>
                <span className="whitespace-pre-line">{r.content}</span>
                {r.note && <span className="block text-[11pt] italic">{r.note}</span>}
              </Cell>
              {showDue && <Cell className="text-center">{x.due ? fmtDate(x.due) : ""}</Cell>}
              {showStatus && <Cell className="text-center">{meta.status![r.status === "done" ? 1 : 0]}</Cell>}
              {showPriority && <Cell className="text-center">{r.priority ? a.yes : ""}</Cell>}
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
