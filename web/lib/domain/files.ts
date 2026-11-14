// Pure helpers for the file manager: file-type classification, a folder index
// (for breadcrumbs and lookups), listing, search and sorting.
import type { FileResponse, TreeNode, TreeResponse } from "@/lib/api/schema";
import type { Hue } from "./hue";

export type FileKind = "image" | "video" | "audio" | "pdf" | "doc" | "sheet" | "slide" | "archive" | "code" | "other";

const EXT_KIND: Record<string, FileKind> = {
  jpg: "image", jpeg: "image", png: "image", gif: "image", webp: "image", svg: "image", bmp: "image", heic: "image", avif: "image",
  mp4: "video", mov: "video", webm: "video", mkv: "video", avi: "video",
  mp3: "audio", wav: "audio", m4a: "audio", ogg: "audio", flac: "audio",
  pdf: "pdf",
  doc: "doc", docx: "doc", odt: "doc", rtf: "doc", txt: "doc", md: "doc",
  xls: "sheet", xlsx: "sheet", ods: "sheet", csv: "sheet",
  ppt: "slide", pptx: "slide", odp: "slide", key: "slide",
  zip: "archive", rar: "archive", "7z": "archive", tar: "archive", gz: "archive",
  json: "code", xml: "code", yml: "code", yaml: "code", js: "code", ts: "code", html: "code", css: "code", sql: "code", go: "code", py: "code",
};

export const fileExt = (name?: string) => {
  const n = name ?? "";
  const i = n.lastIndexOf(".");
  return i > 0 ? n.slice(i + 1).toLowerCase() : "";
};

export function fileKind(f: Pick<FileResponse, "name" | "contentType">): FileKind {
  const ct = (f.contentType ?? "").toLowerCase();
  if (ct.startsWith("image/")) return "image";
  if (ct.startsWith("video/")) return "video";
  if (ct.startsWith("audio/")) return "audio";
  if (ct.includes("pdf")) return "pdf";
  return EXT_KIND[fileExt(f.name)] ?? "other";
}

export const KIND_HUE: Record<FileKind, Hue> = {
  image: "pink",
  video: "violet",
  audio: "cyan",
  pdf: "rose",
  doc: "blue",
  sheet: "green",
  slide: "orange",
  archive: "amber",
  code: "teal",
  other: "blue",
};

/** Kinds the "type" filter offers, in display order. */
export const FILTER_KINDS: FileKind[] = ["image", "pdf", "doc", "sheet", "slide", "video", "archive", "other"];

/** Whether the in-app preview can render the file. */
export const canPreview = (f: Pick<FileResponse, "name" | "contentType">) => {
  const k = fileKind(f);
  return k === "image" || k === "pdf" || k === "video" || k === "audio";
};

export type FolderInfo = {
  node: TreeNode;
  /** Ancestors from the top, ending with the folder itself. */
  path: TreeNode[];
  /** Whether the folder is effectively public (itself and every ancestor). */
  effPublic: boolean;
};

export type FileIndex = {
  folders: Map<string, FolderInfo>;
  /** Every file with the folder path it lives in. */
  files: Array<{ file: FileResponse; path: TreeNode[] }>;
};

export function buildIndex(tree?: TreeResponse): FileIndex {
  const folders = new Map<string, FolderInfo>();
  const files: FileIndex["files"] = [];
  const walk = (nodes: TreeNode[] | undefined, path: TreeNode[], parentPublic: boolean) => {
    for (const n of nodes ?? []) {
      const p = [...path, n];
      const eff = parentPublic && !!n.isPublic;
      if (n.id) folders.set(n.id, { node: n, path: p, effPublic: eff });
      for (const f of n.files ?? []) files.push({ file: f, path: p });
      walk(n.children, p, eff);
    }
  };
  walk(tree?.folders, [], true);
  for (const f of tree?.rootFiles ?? []) files.push({ file: f, path: [] });
  return { folders, files };
}

/** A file is visible publicly only when it and its whole folder chain are public. */
export function fileEffPublic(idx: FileIndex, f: FileResponse): boolean {
  if (!f.isPublic || !f.folderId) return false;
  return idx.folders.get(f.folderId)?.effPublic ?? false;
}

export type SortKey = "name" | "updated" | "size" | "type";
export type SortDir = "asc" | "desc";

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

export function sortFiles(files: FileResponse[], key: SortKey, dir: SortDir): FileResponse[] {
  const sign = dir === "asc" ? 1 : -1;
  return [...files].sort((a, b) => {
    let c = 0;
    if (key === "updated") c = Date.parse(a.updatedAt) - Date.parse(b.updatedAt);
    else if (key === "size") c = (a.sizeBytes ?? 0) - (b.sizeBytes ?? 0);
    else if (key === "type") c = collator.compare(fileKind(a), fileKind(b)) || collator.compare(fileExt(a.name), fileExt(b.name));
    return (c || collator.compare(a.name ?? "", b.name ?? "")) * sign;
  });
}

export function sortFolders(folders: TreeNode[], key: SortKey, dir: SortDir): TreeNode[] {
  const sign = key === "name" && dir === "desc" ? -1 : 1;
  // Project/module folders first, then custom ones, each alphabetically.
  const rank = (n: TreeNode) => (n.kind === "project" ? 0 : n.kind === "module" ? 1 : 2);
  return [...folders].sort((a, b) => rank(a) - rank(b) || collator.compare(a.name ?? "", b.name ?? "") * sign);
}

/** Count of files under a folder, recursively. */
export function countFiles(n: TreeNode): number {
  return (n.files?.length ?? 0) + (n.children ?? []).reduce((s, c) => s + countFiles(c), 0);
}

export const normalize = (s: string) =>
  s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").replace(/Đ/g, "D").toLowerCase();
