"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { downloadFile } from "@/lib/download";
import { useToast } from "@/lib/toast/ToastProvider";
import { tr } from "@/lib/i18n";

/**
 * `@page` rule for printed documents.
 *
 * Margins come from `@page` itself so every printed page gets them, in Chrome and in Safari.
 * (Wrapping the document in a table with repeating header/footer rows also gives per-page
 * margins, but WebKit cannot split a table cell taller than a page and prints a blank sheet.)
 *
 * Declaring the page margin boxes, even empty, stops Chrome from printing its own header and
 * footer (page title, URL, date) in those margins. Safari prints none unless asked to.
 */
export function pageRule(margin = "0") {
  const boxes = ["top-left", "top-center", "top-right", "bottom-left", "bottom-center", "bottom-right"]
    .map((b) => `@${b} { content: ""; }`)
    .join(" ");
  return `@page { size: A4; margin: ${margin}; ${boxes} }`;
}

/**
 * Downloads the document built on the server (PDF, or Excel with `label`), with the print
 * options chosen on the page (layout, signing mode, place, signer…), so it looks the same
 * whatever the browser.
 */
export function PdfDownloadButton({ path, filename, label }: { path: string; filename: string; label?: string }) {
  const sp = useSearchParams();
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  const download = async () => {
    const q = new URLSearchParams();
    if (sp.get("format") === "admin") q.set("template", "admin");
    for (const k of ["sign", "place", "signer", "merge"]) {
      const v = sp.get(k);
      if (v) q.set(k, v);
    }
    setBusy(true);
    try {
      const qs = q.toString();
      await downloadFile(qs ? `${path}?${qs}` : path, filename);
    } catch (e) {
      toast.error(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <button
      type="button"
      onClick={download}
      disabled={busy}
      className="h-ctl rounded-md border border-(--doc-brand) px-4 text-[13px] font-medium text-(--doc-brand) disabled:opacity-60"
    >
      {busy ? "…" : (label ?? tr().adminDoc.downloadPdf)}
    </button>
  );
}
