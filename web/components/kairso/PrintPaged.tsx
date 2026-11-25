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
