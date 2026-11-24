import type { ReactNode } from "react";

/**
 * Page margins for printed documents without using @page margins.
 *
 * A non-zero @page margin makes Chrome/Safari print their own header and footer there (page
 * title, URL, date), which is what ends up "stuck" to the PDF edge. With `@page { margin: 0 }`
 * the browser prints nothing in the margins; the empty header/footer rows of this table repeat
 * on every printed page and keep the content away from the paper edge instead.
 */
export function PrintPaged({ children, margin = "10mm" }: { children: ReactNode; margin?: string }) {
  return (
    <table className="w-full border-collapse">
      <thead className="hidden print:table-header-group">
        <tr><td className="p-0" style={{ height: margin }} /></tr>
      </thead>
      <tfoot className="hidden print:table-footer-group">
        <tr><td className="p-0" style={{ height: margin }} /></tr>
      </tfoot>
      <tbody>
        <tr style={{ breakInside: "auto" }}>
          <td className="p-0 align-top">{children}</td>
        </tr>
      </tbody>
    </table>
  );
}
