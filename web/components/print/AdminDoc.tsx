"use client";

import { useCallback, type CSSProperties, type ReactNode } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import type { Signature } from "@/lib/api/schema";
import { docTokens } from "@/lib/domain/brand";
import { dict } from "@/lib/i18n/dict";
import { tr } from "@/lib/i18n";
import { SegmentedControl } from "@/components/ui/form";
import { DocStamp } from "@/components/kairso/DocSignatures";
import { PrintPaged } from "@/components/kairso/PrintPaged";

export type PrintFormat = "modern" | "admin";
export type SignMode = "digital" | "wet";

/** Print options live in the query string so a printed link reopens the same way. */
export function usePrintOptions() {
  const sp = useSearchParams();
  const router = useRouter();
  const path = usePathname();
  const format: PrintFormat = sp.get("format") === "admin" ? "admin" : "modern";
  const sign: SignMode = sp.get("sign") === "wet" ? "wet" : "digital";
  const place = sp.get("place");
  const signer = sp.get("signer");
  const merge = sp.get("merge");
  const set = useCallback(
    (k: string, v: string | null) => {
      const next = new URLSearchParams(sp.toString());
      next.delete("auto");
      if (v === null || v === "") next.delete(k);
      else next.set(k, v);
      router.replace(`${path}?${next.toString()}`, { scroll: false });
    },
    [sp, router, path],
  );
  return { format, sign, place, signer, merge, set };
}

export function FormatSwitch() {
  const { format, set } = usePrintOptions();
  const t = tr().adminDoc;
  return (
    <SegmentedControl
      size="sm"
      label={t.format}
      value={format}
      onChange={(v) => set("format", v === "admin" ? "admin" : null)}
      options={[{ value: "modern", label: t.modern }, { value: "admin", label: t.admin }]}
    />
  );
}

/** Document body strings: administrative documents are always Vietnamese. */
export const A = () => dict.vi.adminDoc;

/** Last part of an address, used as the default place in the date line ("…, TP. Hồ Chí Minh"). */
export function placeFromAddress(address?: string) {
  const parts = (address ?? "").split(",").map((s) => s.trim()).filter(Boolean);
  return parts[parts.length - 1] ?? "";
}

const pad = (n: number) => String(n).padStart(2, "0");

/**
 * A4 administrative page (Decree 30/2020): Times New Roman 13pt, margins top/bottom 20mm,
 * left 30mm, right 15mm, black text. The toolbar picks the template and the signing mode.
 */
export function AdminShell({
  brandColor,
  defaultPlace,
  unsigned,
  printLabel,
  signerDefault,
  extra,
  children,
}: {
  brandColor?: string | null;
  defaultPlace: string;
  unsigned: boolean;
  printLabel: string;
  /** When set, the toolbar lets the user change the approver's title (defaults to this). */
  signerDefault?: string;
  /** Extra toolbar controls specific to the document. */
  extra?: ReactNode;
  children: ReactNode;
}) {
  const { sign, place, signer, set } = usePrintOptions();
  const t = tr().adminDoc;
  return (
    <div className="min-h-screen bg-(--doc-backdrop) py-8 print:bg-(--doc-paper) print:py-0" style={docTokens(brandColor) as CSSProperties | undefined}>
      <style>{`
        @page { size: A4; margin: 0; }
        @media print { html, body { background: var(--doc-paper) !important; } }
        .adoc, .adoc * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
        .adoc tr, .adoc .keep { break-inside: avoid; }
      `}</style>
      <div className="mx-auto mb-4 flex w-[210mm] max-w-full flex-wrap items-center gap-3 px-2 print:hidden">
        <FormatSwitch />
        <SegmentedControl
          size="sm"
          label={t.signMode}
          value={sign}
          onChange={(v) => set("sign", v === "wet" ? "wet" : null)}
          options={[{ value: "digital", label: t.signDigital }, { value: "wet", label: t.signWet }]}
        />
        <label className="flex items-center gap-1.5 text-[12.5px] text-(--doc-ink-2)">
          {t.place}
          <input
            className="h-7 w-40 rounded-md border border-(--doc-line) bg-(--doc-paper) px-2 text-[12.5px] text-(--doc-ink)"
            placeholder={t.placePh}
            defaultValue={place ?? defaultPlace}
            onBlur={(e) => set("place", e.target.value.trim() === defaultPlace ? null : e.target.value.trim())}
          />
        </label>
        {signerDefault && (
          <label className="flex items-center gap-1.5 text-[12.5px] text-(--doc-ink-2)">
            {t.signerTitle}
            <input
              className="h-7 w-40 rounded-md border border-(--doc-line) bg-(--doc-paper) px-2 text-[12.5px] uppercase text-(--doc-ink)"
              defaultValue={signer ?? signerDefault}
              onBlur={(e) => {
                const v = e.target.value.trim().toUpperCase();
                set("signer", v === signerDefault || !v ? null : v);
              }}
            />
          </label>
        )}
        {extra}
        <span className="flex-1" />
        <button type="button" onClick={() => window.print()} className="h-ctl rounded-md bg-(--doc-brand) px-4 text-[13px] font-medium text-white">
          {printLabel}
        </button>
        {sign === "digital" && unsigned && <p className="w-full text-[12px] text-(--doc-muted)">{t.notSigned}</p>}
      </div>
      <PrintPaged margin="20mm">
        <article
          className="adoc mx-auto w-[210mm] max-w-full bg-(--doc-paper) pb-[20mm] pl-[30mm] pr-[15mm] pt-[20mm] text-[13pt] leading-[1.35] text-(--doc-ink) shadow-overlay print:py-0 print:shadow-none"
          style={{ fontFamily: '"Times New Roman", Times, serif' }}
        >
          {children}
        </article>
      </PrintPaged>
    </div>
  );
}

/** Header: issuing body + number on the left, national motto + place/date on the right. */
export function AdminHeader({ org, number, date, place }: { org: string; number?: string; date: Date; place: string }) {
  const a = A();
  return (
    <div className="keep grid grid-cols-[42%_58%] text-center">
      <div>
        <p className="text-[12pt] font-bold uppercase leading-tight">{org}</p>
        <div className="mx-auto mt-0.5 h-px w-1/3 bg-(--doc-ink)" />
        {number !== undefined && <p className="mt-2 text-[13pt]">{a.number}: {number || "……/……"}</p>}
      </div>
      <div>
        <p className="text-[12pt] font-bold leading-tight">{a.nation}</p>
        <p className="text-[13pt] font-bold leading-tight">{a.motto}</p>
        <div className="mx-auto mt-0.5 h-px w-[55%] bg-(--doc-ink)" />
        <p className="mt-2 text-[13pt] italic">{a.dateLine(place, pad(date.getDate()), pad(date.getMonth() + 1), String(date.getFullYear()))}</p>
      </div>
    </div>
  );
}

export interface AdminSlot {
  title: string;
  hint: string;
  sig?: Signature;
  /** Always left empty to sign and seal by hand (no in-app signature for this role). */
  blank?: boolean;
}

/** Signature columns: stamped in "digital" mode, left blank for a wet signature and seal otherwise. */
export function AdminSignatures({ left, slots }: { left?: ReactNode; slots: AdminSlot[] }) {
  const { sign } = usePrintOptions();
  const fmt = (iso: string) => {
    const d = new Date(iso);
    return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}`;
  };
  return (
    <div className="keep mt-[8mm] flex items-start justify-between gap-6">
      {left !== undefined && <div className="min-w-0 flex-1 text-[11pt]">{left}</div>}
      {slots.map((s) => (
        <div key={s.title} className="w-[68mm] shrink-0 text-center">
          <p className="text-[13pt] font-bold">{s.title}</p>
          <p className="text-[12pt] italic">{s.hint}</p>
          <div className="grid h-[30mm] place-items-center">{sign === "digital" && !s.blank && s.sig && <DocStamp name={s.sig.name} date={fmt(s.sig.signedAt)} />}</div>
          <p className="text-[13pt] font-bold">{s.blank ? "" : (s.sig?.name ?? "")}</p>
        </div>
      ))}
    </div>
  );
}

/** "Nơi nhận" block that sits left of the signatures. */
export function Recipients() {
  const a = A();
  return (
    <>
      <p className="font-bold italic">{a.recipients}</p>
      {a.recipientsList.map((l) => (
        <p key={l}>{l}</p>
      ))}
    </>
  );
}

/** Bordered table cell for administrative documents. */
export const Cell = ({ children, className = "", head }: { children?: ReactNode; className?: string; head?: boolean }) =>
  head ? (
    <th className={`border border-(--doc-ink) px-1.5 py-1 text-center text-[12pt] font-bold ${className}`}>{children}</th>
  ) : (
    <td className={`border border-(--doc-ink) px-1.5 py-1 align-top text-[12pt] ${className}`}>{children}</td>
  );
