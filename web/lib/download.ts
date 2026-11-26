import { requestBlob } from "@/lib/api/http";

export function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

/** GETs a generated file (Excel export…) through the API proxy and saves it. */
export async function downloadFile(path: string, filename: string) {
  triggerDownload(await requestBlob("GET", path), filename);
}

/** File-name-safe version of a document number or title. */
export const safeFilename = (s: string) => s.trim().replace(/[\\/:*?"<>|\s]+/g, "_") || "document";
