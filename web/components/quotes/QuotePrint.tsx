"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { useSearchParams } from "next/navigation";
import type { PartyInfo } from "@/lib/api/schema";
import { useWorkspace } from "@/lib/workspace/WorkspaceContext";
import { useQuote } from "@/lib/hooks/quotes";
import { formatMoney, formatNumber } from "@/lib/domain/money";
import { dict } from "@/lib/i18n/dict";
import { docTokens } from "@/lib/domain/brand";
import { PdfDownloadButton, PrintStatus, pageRule } from "@/components/kairso/PrintPaged";
import { safeFilename } from "@/lib/download";
import { FormatSwitch, usePrintOptions } from "@/components/print/AdminDoc";
import { AdminQuotePrint } from "./AdminQuotePrint";
import { PrintSignatures } from "@/components/kairso/DocSignatures";
import { dateLocale, tr, type Lang } from "@/lib/i18n";

const MIN_SCALE = 0.55;

function fmtStamp(iso: string, lang: Lang) {
  return new Date(iso).toLocaleDateString(dateLocale(lang), { day: "2-digit", month: "2-digit", year: "numeric" });
}

function fmtDocDate(d: string | undefined, lang: Lang) {
  if (!d) return "—";
  const [y, m, day] = d.split("-").map(Number);
  return new Date(y, m - 1, day).toLocaleDateString(dateLocale(lang), { day: "2-digit", month: "2-digit", year: "numeric" });
}

export function QuotePrint({ id }: { id: string }) {
  const sp = useSearchParams();
  const opts = usePrintOptions();
  const { workspaceId, workspace } = useWorkspace();
  const q = useQuote(workspaceId, id);
  const quote = q.data;

  // The title names the saved PDF; a soft navigation (changing an option) resets it, so re-apply.
  useEffect(() => {
    if (quote) document.title = quote.number;
  }, [quote, sp]);

  // Fit the whole document on one A4 page by scaling it, so Chrome and Safari
  // paginate identically instead of each breaking the table in a different place.
  const [fit, setFit] = useState(sp.get("fit") !== "0");
  const [scale, setScale] = useState(1);
  const [boxH, setBoxH] = useState<number | undefined>(undefined);
  const inner = useRef<HTMLDivElement>(null);
  const measure = useCallback(() => {
    const el = inner.current;
    if (!el) return;
    if (!fit) {
      setScale(1);
      setBoxH(undefined);
      return;
    }
    const pageH = 297 * (96 / 25.4) - 4; // A4 height in CSS px, minus a rounding margin
    let lo = MIN_SCALE;
    let hi = 1;
    let best = MIN_SCALE;
    for (let i = 0; i < 9; i++) {
      const s = (lo + hi) / 2;
      el.style.width = `${210 / s}mm`;
      el.style.transform = `scale(${s})`;
      if (el.scrollHeight * s <= pageH) {
        best = s;
        lo = s;
      } else {
        hi = s;
      }
    }
    // The widest scale at which the content still fits (1 when it already does).
    el.style.width = `${210}mm`;
    el.style.transform = "";
    if (el.scrollHeight <= pageH) best = 1;
    else {
      el.style.width = `${210 / best}mm`;
      el.style.transform = `scale(${best})`;
    }
    setScale(best);
    setBoxH(el.scrollHeight * best);
  }, [fit]);

  useLayoutEffect(() => {
    if (!quote) return;
    measure();
    document.fonts?.ready.then(measure);
    window.addEventListener("beforeprint", measure);
    return () => window.removeEventListener("beforeprint", measure);
  }, [quote, measure]);

  if (q.isLoading || !workspaceId) return <PrintStatus>{tr().quotePrint.loading}</PrintStatus>;
  if (!quote) return <PrintStatus>{tr().quotePrint.failed}</PrintStatus>;
  const pdfPath = `/v1/workspaces/${workspaceId}/quotes/${quote.id}/pdf`;
  const pdfName = `${safeFilename(quote.number)}.pdf`;
  const excel = { path: `/v1/workspaces/${workspaceId}/quotes/${quote.id}/export`, filename: `${safeFilename(quote.number)}.xlsx` };
  if (opts.format === "admin") return <AdminQuotePrint quote={quote} workspace={workspace} pdf={{ path: pdfPath, filename: pdfName }} excel={excel} />;

  // The document follows the quote's language, not the viewer's UI language.
  const lang: Lang = quote.language === "en" ? "en" : "vi";
  const p = dict[lang].quotePrint;
  const money = (v: number) => formatMoney(v, quote.currency, quote.minorUnits, lang);
  const issuer = quote.issuer;
  const hasLineDiscount = quote.items.some((it) => it.discountPercent > 0);

  return (
    <div className="min-h-screen bg-(--doc-backdrop) py-8 print:bg-(--doc-paper) print:py-0" style={docTokens(workspace?.brandColor) as CSSProperties | undefined}>
      <style>{`
        ${pageRule(fit ? "0" : "10mm 0")}
        @media print { html, body { background: var(--doc-paper) !important; } }
        .doc, .doc * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
        .doc tr, .doc .keep { break-inside: avoid; }
      `}</style>

      <div className="mx-auto mb-4 flex w-[210mm] max-w-full items-center justify-end gap-2 px-2 print:hidden">
        <FormatSwitch />
        <span className="mr-auto text-[12.5px] text-(--doc-muted)">{tr().quotePrint.hint}</span>
        <label className="flex items-center gap-1.5 text-[12.5px] text-(--doc-ink-2)">
          <input type="checkbox" checked={fit} onChange={(e) => setFit(e.target.checked)} />
          {tr().quotePrint.fitOne}
        </label>
        <PdfDownloadButton path={excel.path} filename={excel.filename} label={tr().adminDoc.downloadExcel} />
        <PdfDownloadButton path={pdfPath} filename={pdfName} />
        <button type="button" onClick={() => window.print()} className="h-ctl rounded-md bg-(--doc-brand) px-4 text-[13px] font-medium text-white">
          {tr().quotePrint.printBtn}
        </button>
      </div>

      <div className={`mx-auto w-[210mm] max-w-full ${fit ? "overflow-hidden" : ""}`} style={fit ? { height: boxH } : undefined}>
      <div ref={inner} style={fit ? { width: `${210 / scale}mm`, transform: `scale(${scale})`, transformOrigin: "top left" } : undefined}>
      <article className="doc relative mx-auto w-full overflow-hidden bg-(--doc-paper) font-sans text-(--doc-ink) shadow-overlay print:shadow-none">
        {quote.status === "draft" && (
          <div aria-hidden="true" className="pointer-events-none absolute inset-0 grid place-items-center">
            <span className="-rotate-30 select-none text-[110px] font-bold tracking-[0.15em] text-(--doc-tint-line) opacity-60">{p.draftMark}</span>
          </div>
        )}

        <header className={`flex items-start gap-4 px-[14mm] pb-[4mm] pt-[11mm] ${fit ? "" : "print:pt-[2mm]"}`}>
          <div className="grid size-[15mm] shrink-0 place-items-center overflow-hidden rounded-md border border-(--doc-line) bg-(--doc-paper)">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={workspace?.avatar || "/icon-192.png"} alt={issuer.name || "Logo"} className="size-[12mm] object-contain" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-[14px] font-bold leading-tight">{issuer.name || workspace?.name}</p>
            <PartyLines party={issuer} p={p} />
          </div>
          <div className="shrink-0 text-right">
            <p className="text-[22px] font-bold leading-none tracking-[0.1em] text-(--doc-brand)">{p.docTitle}</p>
            <p className="mt-1.5 text-[11px] text-(--doc-ink-2)">{p.number}: <b className="font-mono text-(--doc-ink)">{quote.number}</b></p>
            <p className="text-[11px] text-(--doc-ink-2)">{p.date}: {fmtDocDate(quote.issueDate, lang)}</p>
            {quote.validUntil && <p className="text-[11px] text-(--doc-ink-2)">{p.validUntil}: {fmtDocDate(quote.validUntil, lang)}</p>}
          </div>
        </header>
        <div className="mx-[14mm] h-[1.5px] bg-(--doc-brand)" />

        <div className="relative flex flex-col gap-[4.5mm] px-[14mm] pb-[12mm] pt-[5mm]">
          <div className="keep grid grid-cols-[1.4fr_1fr] gap-6 border-b border-(--doc-line) pb-[4mm]">
            <div>
              <p className="text-[9px] font-semibold uppercase tracking-wider text-(--doc-brand)">{p.to}</p>
              <p className="mt-0.5 text-[13px] font-semibold">{quote.customer.name || "—"}</p>
              <PartyLines party={quote.customer} p={p} />
            </div>
            <div className="text-[11.5px]">
              <p className="text-[9px] font-semibold uppercase tracking-wider text-(--doc-brand)">{p.project}</p>
              <p className="mt-0.5 font-medium">{quote.projectName}</p>
              <p className="mt-1.5 text-[9px] font-semibold uppercase tracking-wider text-(--doc-brand)">{p.module}</p>
              <p className="mt-0.5 font-medium">{quote.moduleName}</p>
            </div>
          </div>

          {quote.title && <h1 className="text-[16px] font-bold leading-snug">{quote.title}</h1>}

          <table className="w-full border-collapse text-[11.5px]">
            <thead>
              <tr className="bg-(--doc-brand) text-left text-[9.5px] uppercase tracking-wide text-white">
                <th className="w-8 px-2 py-1.5 text-center">{p.no}</th>
                <th className="px-2 py-1.5">{p.item}</th>
                <th className="w-12 px-2 py-1.5">{p.unit}</th>
                <th className="w-14 px-2 py-1.5 text-right">{p.qty}</th>
                <th className="w-28 px-2 py-1.5 text-right">{p.unitPrice}</th>
                {hasLineDiscount && <th className="w-12 px-2 py-1.5 text-right">{p.discount}</th>}
                <th className="w-32 px-2 py-1.5 text-right">{p.amount}</th>
              </tr>
            </thead>
            <tbody>
              {quote.items.map((it, i) => (
                <tr key={it.id} className="border-b border-(--doc-line) align-top">
                  <td className="px-2 py-1.5 text-center text-(--doc-muted)">{i + 1}</td>
                  <td className="px-2 py-1.5">
                    <p className="font-medium">{it.name}</p>
                    {it.description && <p className="mt-0.5 whitespace-pre-line text-[10.5px] text-(--doc-ink-2)">{it.description}</p>}
                  </td>
                  <td className="px-2 py-1.5">{it.unit}</td>
                  <td className="px-2 py-1.5 text-right font-mono">{formatNumber(it.quantity, 3, lang)}</td>
                  <td className="px-2 py-1.5 text-right font-mono">{money(it.unitPrice)}</td>
                  {hasLineDiscount && <td className="px-2 py-1.5 text-right font-mono">{it.discountPercent ? `${formatNumber(it.discountPercent, 2, lang)}%` : ""}</td>}
                  <td className="px-2 py-1.5 text-right font-mono">{money(it.lineTotal)}</td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className="keep flex justify-end">
            <dl className="w-[80mm] text-[12px]">
              <TotalRow label={p.subtotal} value={money(quote.subtotal)} />
              {quote.discountTotal > 0 && (
                <TotalRow label={quote.discountType === "percent" ? `${p.discountTotal} (${formatNumber(quote.discountPercent, 2, lang)}%)` : p.discountTotal} value={`− ${money(quote.discountTotal)}`} />
              )}
              <TotalRow label={p.vat(formatNumber(quote.vatRate, 2, lang))} value={money(quote.taxTotal)} />
              <div className="mt-1 flex items-baseline justify-between bg-(--doc-brand) px-3 py-1.5 text-white">
                <dt className="text-[12px] font-semibold">{p.grand}</dt>
                <dd className="font-mono text-[14px] font-bold">{money(quote.grandTotal)}</dd>
              </div>
            </dl>
          </div>
          <p className="keep -mt-1.5 text-[11.5px] italic text-(--doc-ink-2)">
            <b className="not-italic text-(--doc-ink)">{p.inWords}:</b> {quote.amountInWords}.
          </p>

          {quote.notes && <Block title={p.notes}>{quote.notes}</Block>}
          {quote.terms && <Block title={p.terms}>{quote.terms}</Block>}
          {issuer.bankInfo && <Block title={p.bank}>{issuer.bankInfo}</Block>}

          <PrintSignatures
            formatDate={(iso) => fmtStamp(iso, lang)}
            slots={[
              {
                role: "preparer",
                label: p.preparer,
                hint: p.signHint,
                sig: quote.signatures.find((x) => x.role === "preparer"),
              },
            ]}
          />
        </div>
      </article>
      </div>
      </div>
    </div>
  );
}

type PrintMessages = (typeof dict)["vi"]["quotePrint"];

function PartyLines({ party, p }: { party: PartyInfo; p: PrintMessages }) {
  const cls = "text-[11px] leading-snug text-(--doc-ink-2)";
  return (
    <div className={`mt-0.5 flex flex-col ${cls}`}>
      {party.address && <span>{party.address}</span>}
      {party.taxCode && <span>{p.taxCode}: {party.taxCode}</span>}
      {(party.phone || party.email) && <span>{[party.phone && `${p.tel}: ${party.phone}`, party.email].filter(Boolean).join(" · ")}</span>}
      {party.contactName && <span>{p.contact}: {party.contactName}</span>}
      {party.website && <span>{party.website}</span>}
    </div>
  );
}

function TotalRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between border-b border-(--doc-line) px-3 py-1">
      <dt className="text-(--doc-ink-2)">{label}</dt>
      <dd className="font-mono">{value}</dd>
    </div>
  );
}

function Block({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="keep border-l-2 border-(--doc-brand) pl-3">
      <p className="text-[9.5px] font-semibold uppercase tracking-wider text-(--doc-brand)">{title}</p>
      <p className="mt-0.5 whitespace-pre-line text-[11.5px] leading-relaxed text-(--doc-ink-2)">{children}</p>
    </div>
  );
}

