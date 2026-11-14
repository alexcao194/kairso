"use client";

import type { ComponentProps, DragEvent, KeyboardEvent, ReactNode } from "react";
import type { FileResponse, TreeNode } from "@/lib/api/schema";
import { countFiles, fileEffPublic, fileExt, fileKind, KIND_HUE, type FileIndex, type SortDir, type SortKey } from "@/lib/domain/files";
import { fmtRelative } from "@/lib/domain/format";
import { fmtBytes } from "@/lib/upload";
import { useT } from "@/lib/i18n";
import { IconTile } from "@/components/ui/IconTile";
import { Menu } from "@/components/ui/Popover";
import { IconChevronDown, IconChevronUp, IconFile, IconFolder, IconGlobe, IconImage, IconMore, IconPlay } from "@/components/icons";

export type MenuItems = ComponentProps<typeof Menu>["items"];

export type ViewProps = {
  folders: TreeNode[];
  files: FileResponse[];
  idx: FileIndex;
  selectedId: string | null;
  /** Search results span folders: show each item's location. */
  showPath: boolean;
  isManager: boolean;
  dropTarget: string;
  sort: SortKey;
  dir: SortDir;
  onSort: (key: SortKey) => void;
  onSelect: (f: FileResponse) => void;
  onOpen: (f: FileResponse) => void;
  onOpenFolder: (id: string) => void;
  fileMenu: (f: FileResponse) => MenuItems;
  folderMenu: (n: TreeNode) => MenuItems;
  onFileDragStart: (e: DragEvent, f: FileResponse) => void;
  onFolderDragStart: (e: DragEvent, n: TreeNode) => void;
  onFolderDragOver: (e: DragEvent, id: string) => void;
  onFolderDragLeave: () => void;
  onFolderDrop: (e: DragEvent, id: string) => void;
};

// ---- shared bits ----

const folderHue = (n: TreeNode) => (n.kind === "project" ? "violet" : n.kind === "module" ? "blue" : "amber");

export function FolderGlyph({ node, size = 28 }: { node: TreeNode; size?: number }) {
  return (
    <IconTile hue={folderHue(node)} size={size}>
      <IconFolder size={Math.round(size * 0.55)} />
    </IconTile>
  );
}

export function FileGlyph({ file, size = 28 }: { file: Pick<FileResponse, "name" | "contentType">; size?: number }) {
  const kind = fileKind(file);
  const icon = kind === "image" ? <IconImage size={Math.round(size * 0.55)} /> : kind === "video" ? <IconPlay size={Math.round(size * 0.5)} /> : <IconFile size={Math.round(size * 0.55)} />;
  return <IconTile hue={KIND_HUE[kind]} size={size}>{icon}</IconTile>;
}

function ExtBadge({ file }: { file: FileResponse }) {
  const ext = fileExt(file.name);
  if (!ext) return null;
  return <span className="rounded bg-surface/90 px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase text-ink-2 shadow-sm">{ext}</span>;
}

function PublicMark({ idx, file }: { idx: FileIndex; file: FileResponse }) {
  const t = useT().fx;
  if (!file.isPublic) return null;
  const eff = fileEffPublic(idx, file);
  return (
    <span title={eff ? t.public : t.publicHidden} aria-label={eff ? t.public : t.publicHidden} className={eff ? "text-(--green)" : "text-(--amber)"}>
      <IconGlobe size={13} />
    </span>
  );
}

function pathLabel(idx: FileIndex, file: FileResponse, root: string) {
  const info = file.folderId ? idx.folders.get(file.folderId) : undefined;
  return info ? info.path.map((p) => p.name).join(" / ") : root;
}

function ItemMenu({ items, label }: { items: MenuItems; label: string }) {
  if (!items.length) return null;
  return (
    <Menu
      align="end"
      trigger={({ ref, open, ...p }) => (
        <button
          ref={ref}
          type="button"
          {...p}
          aria-label={label}
          onClick={(e) => {
            e.stopPropagation();
            p.onClick?.();
          }}
          className={`grid size-7 place-items-center rounded-md text-ink-muted hover:bg-hover hover:text-ink ${open ? "bg-hover" : ""}`}
        >
          <IconMore size={15} />
        </button>
      )}
      items={items}
    />
  );
}

const onActivate = (open: () => void) => (e: KeyboardEvent) => {
  if (e.key === "Enter") {
    e.preventDefault();
    open();
  }
};

function SectionHeading({ children }: { children: ReactNode }) {
  return <h3 className="mb-2 mt-1 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-faint">{children}</h3>;
}

// ---- grid ----

export function GridView(p: ViewProps) {
  const t = useT().fx;
  return (
    <div className="flex flex-col gap-5">
      {p.folders.length > 0 && (
        <section>
          <SectionHeading>{t.foldersHeading}</SectionHeading>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-2.5">
            {p.folders.map((n) => {
              const id = n.id ?? "";
              return (
                <div
                  key={id}
                  role="button"
                  tabIndex={0}
                  aria-label={t.openFolder(n.name ?? "")}
                  draggable={p.isManager && n.kind === "custom"}
                  onDragStart={(e) => p.onFolderDragStart(e, n)}
                  onDragOver={(e) => p.onFolderDragOver(e, id)}
                  onDragLeave={p.onFolderDragLeave}
                  onDrop={(e) => p.onFolderDrop(e, id)}
                  onDoubleClick={() => p.onOpenFolder(id)}
                  onClick={() => p.onOpenFolder(id)}
                  onKeyDown={onActivate(() => p.onOpenFolder(id))}
                  className={`group flex h-14 cursor-pointer items-center gap-2.5 rounded-lg border bg-surface px-3 transition-colors hover:border-line-strong hover:bg-row-hover ${p.dropTarget === id ? "border-accent ring-2 ring-accent" : "border-line"}`}
                >
                  <FolderGlyph node={n} size={30} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[13px] font-medium text-ink">{n.name}</span>
                    <span className="block truncate text-[11.5px] text-ink-faint">
                      {n.kind === "project" ? t.projectFolder : n.kind === "module" ? t.moduleFolder : t.fileCount(countFiles(n))}
                    </span>
                  </span>
                  <span className="opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                    <ItemMenu items={p.folderMenu(n)} label={n.name ?? ""} />
                  </span>
                </div>
              );
            })}
          </div>
        </section>
      )}

      {p.files.length > 0 && (
        <section>
          <SectionHeading>{t.filesHeading}</SectionHeading>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(176px,1fr))] gap-3">
            {p.files.map((f) => {
              const kind = fileKind(f);
              const selected = p.selectedId === f.id;
              return (
                <div
                  key={f.id}
                  role="button"
                  tabIndex={0}
                  aria-label={t.selectFile(f.name)}
                  aria-pressed={selected}
                  draggable={p.isManager}
                  onDragStart={(e) => p.onFileDragStart(e, f)}
                  onClick={() => p.onSelect(f)}
                  onDoubleClick={() => p.onOpen(f)}
                  onKeyDown={onActivate(() => p.onOpen(f))}
                  className={`group flex cursor-pointer flex-col overflow-hidden rounded-lg border bg-surface transition-colors hover:border-line-strong ${selected ? "border-accent ring-2 ring-accent" : "border-line"}`}
                >
                  <div className="relative grid aspect-[4/3] place-items-center overflow-hidden bg-surface-sunken">
                    {kind === "image" ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={f.url} alt="" loading="lazy" className="size-full object-cover" />
                    ) : (
                      <FileGlyph file={f} size={52} />
                    )}
                    <span className="absolute left-2 top-2"><ExtBadge file={f} /></span>
                    <span className="absolute right-1.5 top-1.5 rounded-md bg-surface/90 opacity-0 shadow-sm transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                      <ItemMenu items={p.fileMenu(f)} label={f.name} />
                    </span>
                  </div>
                  <div className="flex flex-col gap-0.5 px-2.5 py-2">
                    <span className="flex items-center gap-1.5">
                      <span className="min-w-0 flex-1 truncate text-[12.5px] font-medium text-ink" title={f.name}>{f.name}</span>
                      <PublicMark idx={p.idx} file={f} />
                    </span>
                    <span className="truncate text-[11px] text-ink-faint">
                      {p.showPath ? pathLabel(p.idx, f, t.allFiles) : `${fmtBytes(f.sizeBytes)} · ${fmtRelative(f.updatedAt)}`}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}

// ---- list ----

function SortHeader({ k, label, p, className = "" }: { k: SortKey; label: string; p: ViewProps; className?: string }) {
  const active = p.sort === k;
  return (
    <th className={`px-2 ${className}`} aria-sort={active ? (p.dir === "asc" ? "ascending" : "descending") : "none"}>
      <button type="button" onClick={() => p.onSort(k)} className={`inline-flex items-center gap-1 uppercase tracking-wide hover:text-ink ${active ? "text-ink-2" : ""}`}>
        {label}
        {active && (p.dir === "asc" ? <IconChevronUp size={11} /> : <IconChevronDown size={11} />)}
      </button>
    </th>
  );
}

export function ListView(p: ViewProps) {
  const t = useT().fx;
  return (
    <table className="w-full min-w-[720px] border-collapse text-[13px]">
      <thead className="sticky top-0 z-10 bg-surface-sunken text-left text-[11px] font-medium text-ink-faint">
        <tr className="h-thead border-b border-line">
          <SortHeader k="name" label={t.colName} p={p} className="pl-3" />
          <SortHeader k="type" label={t.colType} p={p} className="w-32" />
          <SortHeader k="size" label={t.colSize} p={p} className="w-24 text-right" />
          <SortHeader k="updated" label={t.colUpdated} p={p} className="w-36" />
          <th className="w-28 px-2 uppercase tracking-wide">{t.colSharing}</th>
          <th className="w-10" />
        </tr>
      </thead>
      <tbody>
        {p.folders.map((n) => {
          const id = n.id ?? "";
          return (
            <tr
              key={id}
              tabIndex={0}
              aria-label={t.openFolder(n.name ?? "")}
              draggable={p.isManager && n.kind === "custom"}
              onDragStart={(e) => p.onFolderDragStart(e, n)}
              onDragOver={(e) => p.onFolderDragOver(e, id)}
              onDragLeave={p.onFolderDragLeave}
              onDrop={(e) => p.onFolderDrop(e, id)}
              onClick={() => p.onOpenFolder(id)}
              onKeyDown={onActivate(() => p.onOpenFolder(id))}
              className={`group h-row cursor-pointer border-b border-line-row hover:bg-row-hover ${p.dropTarget === id ? "bg-accent-soft" : ""}`}
            >
              <td className="px-3">
                <span className="flex items-center gap-2.5">
                  <FolderGlyph node={n} size={24} />
                  <span className="truncate font-medium text-ink">{n.name}</span>
                </span>
              </td>
              <td className="px-2 text-ink-2">{n.kind === "project" ? t.projectFolder : n.kind === "module" ? t.moduleFolder : t.foldersHeading}</td>
              <td className="px-2 text-right font-mono text-[12px] text-ink-faint">{t.fileCount(countFiles(n))}</td>
              <td className="px-2 text-ink-faint">—</td>
              <td className="px-2" />
              <td className="px-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100"><ItemMenu items={p.folderMenu(n)} label={n.name ?? ""} /></td>
            </tr>
          );
        })}
        {p.files.map((f) => {
          const selected = p.selectedId === f.id;
          const eff = fileEffPublic(p.idx, f);
          return (
            <tr
              key={f.id}
              tabIndex={0}
              aria-label={t.selectFile(f.name)}
              aria-selected={selected}
              draggable={p.isManager}
              onDragStart={(e) => p.onFileDragStart(e, f)}
              onClick={() => p.onSelect(f)}
              onDoubleClick={() => p.onOpen(f)}
              onKeyDown={onActivate(() => p.onOpen(f))}
              className={`group h-row cursor-pointer border-b border-line-row ${selected ? "bg-accent-soft" : "hover:bg-row-hover"}`}
            >
              <td className="px-3">
                <span className="flex min-w-0 items-center gap-2.5">
                  <FileGlyph file={f} size={24} />
                  <span className="min-w-0">
                    <span className="block truncate text-ink" title={f.name}>{f.name}</span>
                    {p.showPath && <span className="block truncate text-[11px] text-ink-faint">{t.inPath(pathLabel(p.idx, f, t.allFiles))}</span>}
                  </span>
                </span>
              </td>
              <td className="px-2 text-ink-2">{t.kind[fileKind(f)]}</td>
              <td className="px-2 text-right font-mono text-[12px] text-ink-2">{fmtBytes(f.sizeBytes)}</td>
              <td className="px-2 text-ink-2">{fmtRelative(f.updatedAt)}</td>
              <td className="px-2">
                {f.isPublic ? (
                  <span className={`inline-flex items-center gap-1 text-[12px] ${eff ? "text-(--green)" : "text-(--amber)"}`} title={eff ? undefined : t.publicHidden}>
                    <IconGlobe size={13} /> {t.public}
                  </span>
                ) : (
                  <span className="text-[12px] text-ink-faint">{t.private}</span>
                )}
              </td>
              <td className="px-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100"><ItemMenu items={p.fileMenu(f)} label={f.name} /></td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// ---- gallery ----

export function GalleryView(p: ViewProps) {
  const t = useT().fx;
  const media = p.files.filter((f) => {
    const k = fileKind(f);
    return k === "image" || k === "video";
  });
  if (media.length === 0) {
    return <p className="py-16 text-center text-[13px] text-ink-faint">{t.galleryEmpty}</p>;
  }
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-2">
      {media.map((f) => {
        const selected = p.selectedId === f.id;
        const video = fileKind(f) === "video";
        return (
          <div
            key={f.id}
            role="button"
            tabIndex={0}
            aria-label={t.selectFile(f.name)}
            aria-pressed={selected}
            draggable={p.isManager}
            onDragStart={(e) => p.onFileDragStart(e, f)}
            onClick={() => p.onSelect(f)}
            onDoubleClick={() => p.onOpen(f)}
            onKeyDown={onActivate(() => p.onOpen(f))}
            className={`group relative aspect-square cursor-pointer overflow-hidden rounded-lg bg-surface-sunken ${selected ? "ring-2 ring-accent" : ""}`}
          >
            {video ? (
              <video src={f.url} muted preload="metadata" className="size-full object-cover" />
            ) : (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={f.url} alt={f.name} loading="lazy" className="size-full object-cover transition-transform duration-300 group-hover:scale-[1.03]" />
            )}
            {video && (
              <span className="absolute inset-0 grid place-items-center">
                <span className="grid size-11 place-items-center rounded-full bg-surface/85 text-ink shadow-sm"><IconPlay size={18} /></span>
              </span>
            )}
            <span className="absolute inset-x-0 bottom-0 truncate bg-(--scrim) px-2.5 py-1.5 text-[12px] font-medium text-white opacity-0 transition-opacity group-hover:opacity-100">
              {f.name}
            </span>
            <span className="absolute right-1.5 top-1.5 rounded-md bg-surface/90 opacity-0 shadow-sm transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
              <ItemMenu items={p.fileMenu(f)} label={f.name} />
            </span>
          </div>
        );
      })}
    </div>
  );
}
