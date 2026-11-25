"use client";

import type { QuoteResponse, WorkspaceResponse } from "@/lib/api/schema";
import { formatMoney, formatNumber } from "@/lib/domain/money";
import { dict } from "@/lib/i18n/dict";
import { dateLocale, tr, type Lang } from "@/lib/i18n";
import { A, AdminHeader, AdminShell, AdminSignatures, Cell, Recipients, placeFromAddress, usePrintOptions } from "@/components/print/AdminDoc";

function docDate(d?: string) {
  if (!d) return new Date();
  const [y, m, day] = d.split("-").map(Number);
  return new Date(y, m - 1, day);
}

/** Quote laid out as a Vietnamese administrative document, for printing, signing and sealing. */
export function AdminQuotePrint({ quote, workspace }: { quote: QuoteResponse; workspace?: WorkspaceResponse | null }) {
  const { place, signer } = usePrintOptions();
  const a = A();
  const lang: Lang = quote.language === "en" ? "en" : "vi";
  const p = dict.vi.quotePrint;
  const money = (v: number) => formatMoney(v, quote.currency, quote.minorUnits, lang);
  const issuer = quote.issuer;
  const org = issuer.name || workspace?.name || "";
  const defaultPlace = placeFromAddress(issuer.address);
  const hasLineDiscount = quote.items.some((it) => it.discountPercent > 0);
  const cols = hasLineDiscount ? 7 : 6;
  const sig = quote.signatures.find((x) => x.role === "preparer");
  const fmt = (d?: string) => (d ? docDate(d).toLocaleDateString(dateLocale("vi"), { day: "2-digit", month: "2-digit", year: "numeric" }) : "");

  return (
    <AdminShell brandColor={workspace?.brandColor} defaultPlace={defaultPlace} unsigned={!sig} printLabel={tr().quotePrint.printBtn} signerDefault={a.director}>
      <AdminHeader org={org} number={quote.number} date={docDate(quote.issueDate)} place={place ?? defaultPlace} />

      <div className="keep mt-[8mm] text-center">
        <p className="text-[14pt] font-bold">{a.quoteTitle}</p>
        {quote.title && <p className="mt-0.5 font-bold">{a.about}: {quote.title}</p>}
      </div>

      <div className="mt-[6mm] text-justify">
        <p>
          {a.to}: <b>{quote.customer.name || "……………………"}</b>
        </p>
        {quote.customer.address && <p>{quote.customer.address}</p>}
        {(quote.customer.taxCode || quote.customer.phone || quote.customer.email) && (
          <p>{[quote.customer.taxCode && `${p.taxCode}: ${quote.customer.taxCode}`, quote.customer.phone && `${p.tel}: ${quote.customer.phone}`, quote.customer.email].filter(Boolean).join(" · ")}</p>
        )}
        <p className="mt-3 indent-[1cm]">{a.quoteIntro(org, quote.moduleName, quote.projectName)}</p>
      </div>

      <table className="mt-3 w-full border-collapse">
        <thead>
          <tr>
            <Cell head className="w-[10mm]">{p.no}</Cell>
            <Cell head>{p.item}</Cell>
            <Cell head className="w-[14mm]">{p.unit}</Cell>
            <Cell head className="w-[14mm]">{p.qty}</Cell>
            <Cell head className="w-[28mm]">{p.unitPrice}</Cell>
            {hasLineDiscount && <Cell head className="w-[12mm]">{p.discount}</Cell>}
            <Cell head className="w-[32mm]">{p.amount}</Cell>
          </tr>
        </thead>
        <tbody>
          {quote.items.map((it, i) => (
            <tr key={it.id}>
              <Cell className="text-center">{i + 1}</Cell>
              <Cell>
                {it.name}
                {it.description && <span className="block whitespace-pre-line text-[11pt] italic">{it.description}</span>}
              </Cell>
              <Cell className="text-center">{it.unit}</Cell>
              <Cell className="text-right">{formatNumber(it.quantity, 3, lang)}</Cell>
              <Cell className="text-right">{money(it.unitPrice)}</Cell>
              {hasLineDiscount && <Cell className="text-right">{it.discountPercent ? `${formatNumber(it.discountPercent, 2, lang)}%` : ""}</Cell>}
              <Cell className="text-right">{money(it.lineTotal)}</Cell>
            </tr>
          ))}
          <TotalRow cols={cols} label={p.subtotal} value={money(quote.subtotal)} />
          {quote.discountTotal > 0 && (
            <TotalRow cols={cols} label={quote.discountType === "percent" ? `${p.discountTotal} (${formatNumber(quote.discountPercent, 2, lang)}%)` : p.discountTotal} value={`− ${money(quote.discountTotal)}`} />
          )}
          <TotalRow cols={cols} label={p.vat(formatNumber(quote.vatRate, 2, lang))} value={money(quote.taxTotal)} />
          <TotalRow cols={cols} label={p.grand.toUpperCase()} value={money(quote.grandTotal)} bold />
        </tbody>
      </table>
      <p className="keep mt-2 italic">
        <b className="not-italic">{p.inWords}:</b> {quote.amountInWords}.
      </p>

      <div className="mt-3 flex flex-col gap-1.5 text-justify">
        {quote.validUntil && <p className="indent-[1cm]">{a.validity(fmt(quote.validUntil))}</p>}
        {quote.notes && <Para title={p.notes}>{quote.notes}</Para>}
        {quote.terms && <Para title={p.terms}>{quote.terms}</Para>}
        {issuer.bankInfo && <Para title={p.bank}>{issuer.bankInfo}</Para>}
        <p className="indent-[1cm]">{a.closing}</p>
      </div>

      {/* A quote is issued under the director's signature and seal; the preparer signs alongside. */}
      <AdminSignatures
        slots={[
          { title: a.quoteSigner, hint: a.hint, sig },
          { title: signer || a.director, hint: a.hintSeal, blank: true },
        ]}
      />
      <div className="keep mt-[4mm] text-[11pt]">
        <Recipients />
      </div>
    </AdminShell>
  );
}

function TotalRow({ cols, label, value, bold }: { cols: number; label: string; value: string; bold?: boolean }) {
  return (
    <tr className={bold ? "font-bold" : ""}>
      <td colSpan={cols - 1} className="border border-(--doc-ink) px-1.5 py-1 text-right text-[12pt]">{label}</td>
      <Cell className="text-right">{value}</Cell>
    </tr>
  );
}

function Para({ title, children }: { title: string; children: string }) {
  return (
    <div className="keep">
      <p className="font-bold">{title}:</p>
      <p className="whitespace-pre-line">{children}</p>
    </div>
  );
}
