"use client";

import { useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode } from "react";
import type { FileResponse, TreeNode } from "@/lib/api/schema";
import { useWorkspace } from "@/lib/workspace/WorkspaceContext";
import { useToast } from "@/lib/toast/ToastProvider";
import { useConfirm } from "@/components/ui/Confirm";
import { presignAndUpload, useFileMutations, useFileSettings, useFileTree } from "@/lib/hooks/files";
import { useSetUrlParams, useUrlState } from "@/lib/hooks/useUrlState";
import {
  buildIndex, canPreview, FILTER_KINDS, fileKind, normalize, sortFiles, sortFolders,
  type FileKind, type SortDir, type SortKey,
} from "@/lib/domain/files";
import { useT } from "@/lib/i18n";
import { Button, IconButton } from "@/components/ui/Button";
import { EmptyState, ErrorState, Skeleton } from "@/components/ui/feedback";
import { SearchInput, Select } from "@/components/ui/form";
import {
  IconChevronDown, IconChevronRight, IconChevronUp, IconClock, IconColumns, IconDownload, IconEdit,
  IconFolder, IconGlobe, IconGrid, IconImage, IconList, IconMaximize, IconPlus, IconSettings, IconTrash, IconUpload,
} from "@/components/icons";
import { FolderTree } from "./FolderTree";
import { FileDetails } from "./FileDetails";
import { GalleryView, GridView, ListView, type MenuItems, type ViewProps } from "./FileViews";
import { NewFolderModal, PreviewModal, RenameModal, SettingsModal, VersionsModal, type RenameTarget } from "./FileModals";

const DRAG_MIME = "application/x-kairso-file";
const VIEWS = ["grid", "list", "gallery"] as const;
type View = (typeof VIEWS)[number];
const SORTS: SortKey[] = ["name", "updated", "size", "type"];
/** Kinds listed explicitly in the filter; anything else counts as "other". */
const NAMED_KINDS = new Set<FileKind>(FILTER_KINDS.filter((k) => k !== "other"));

const matchesKind = (f: FileResponse, filter: string) => {
  if (!filter) return true;
  const k = fileKind(f);
  return filter === "other" ? !NAMED_KINDS.has(k) : k === filter;
};

export function FileManager() {
  const { workspaceId: ws, isManager } = useWorkspace();
  const all = useT();
  const t = all.fx;
  const fm = all.fm;
  const toast = useToast();
  const confirm = useConfirm();
  const tree = useFileTree(ws);
  const settings = useFileSettings(ws);
  const m = useFileMutations(ws);

  // Navigation, view and filters live in the URL so reloads and shared links keep them.
  const setParams = useSetUrlParams();
  const [folderParam] = useUrlState("folder", "");
  const [viewParam] = useUrlState("view", "grid");
  const [sortParam] = useUrlState("sort", "name");
  const [dirParam] = useUrlState("dir", "asc");
  const [kindParam] = useUrlState("type", "");
  const [qParam] = useUrlState("q", "");
  const view: View = (VIEWS as readonly string[]).includes(viewParam) ? (viewParam as View) : "grid";
  const sort: SortKey = SORTS.includes(sortParam as SortKey) ? (sortParam as SortKey) : "name";
  const dir: SortDir = dirParam === "desc" ? "desc" : "asc";

  // Search box: local state, pushed to the URL after a short pause.
  const [query, setQuery] = useState(qParam);
  useEffect(() => setQuery(qParam), [qParam]);
  useEffect(() => {
    if (query === qParam) return;
    const h = setTimeout(() => setParams({ q: query.trim() ? query : null }), 250);
    return () => clearTimeout(h);
  }, [query, qParam, setParams]);

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detailsOpen, setDetailsOpen] = useState(true);
  const [dropTarget, setDropTarget] = useState("");
  const [contentDrag, setContentDrag] = useState(false);
  const dragDepth = useRef(0);
  const [busy, setBusy] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  const [preview, setPreview] = useState<FileResponse | null>(null);
  const [versionsFor, setVersionsFor] = useState<FileResponse | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [newFolderIn, setNewFolderIn] = useState<string | null>(null);
  const [rename, setRename] = useState<RenameTarget | null>(null);

  const idx = useMemo(() => buildIndex(tree.data), [tree.data]);
  const current = folderParam ? idx.folders.get(folderParam) : undefined;
  const folderId = current ? folderParam : "";
  const searching = qParam.trim() !== "";

  const { folders, files } = useMemo(() => {
    let fs: TreeNode[];
    let fl: FileResponse[];
    if (searching) {
      const nq = normalize(qParam.trim());
      fs = [...idx.folders.values()].filter((i) => normalize(i.node.name ?? "").includes(nq)).map((i) => i.node);
      fl = idx.files.filter((x) => normalize(x.file.name).includes(nq)).map((x) => x.file);
    } else if (current) {
      fs = current.node.children ?? [];
      fl = current.node.files ?? [];
    } else {
      fs = tree.data?.folders ?? [];
      fl = tree.data?.rootFiles ?? [];
    }
    return {
      folders: kindParam ? [] : sortFolders(fs, sort, dir),
      files: sortFiles(fl.filter((f) => matchesKind(f, kindParam)), sort, dir),
    };
  }, [searching, qParam, idx, current, tree.data, kindParam, sort, dir]);

  const selected = selectedId ? idx.files.find((x) => x.file.id === selectedId)?.file : undefined;

  // ---- actions ----

  const openFolder = (id: string) => {
    setSelectedId(null);
    setQuery("");
    setParams({ folder: id || null, q: null });
  };

  const openFile = (f: FileResponse) => {
    if (canPreview(f)) setPreview(f);
    else window.open(f.url, "_blank", "noopener");
  };

  const upload = async (list: FileList | File[], target: string) => {
    const arr = Array.from(list);
    if (!arr.length) return;
    setBusy(true);
    try {
      for (const f of arr) {
        const up = await presignAndUpload(ws, f);
        await m.createFile.mutateAsync({ folderId: target || "", name: f.name, objectName: up.objectName, contentType: up.contentType, sizeBytes: up.sizeBytes, isPublic: false });
      }
      toast.success(arr.length > 1 ? fm.uploadedMany : fm.uploadedOne);
    } catch (e) {
      toast.error(e);
    } finally {
      setBusy(false);
    }
  };

  const togglePublic = async (kind: "folder" | "file", id: string, val: boolean) => {
    try {
      if (kind === "file") await m.updateFile.mutateAsync({ id, body: { isPublic: val } });
      else await m.updateFolder.mutateAsync({ id, body: { isPublic: val } });
    } catch (e) {
      toast.error(e);
    }
  };

  const remove = async (kind: "folder" | "file", id: string, name: string) => {
    const ok = await confirm({ title: fm.confirmDelete(kind === "file", name), message: kind === "folder" ? fm.deleteFolderDesc : undefined, danger: true });
    if (!ok) return;
    try {
      if (kind === "file") await m.deleteFile.mutateAsync(id);
      else await m.deleteFolder.mutateAsync(id);
      if (kind === "file" && selectedId === id) setSelectedId(null);
      if (kind === "folder" && folderId === id) openFolder(current?.node.parentId ?? "");
      toast.success(fm.deleted);
    } catch (e) {
      toast.error(e);
    }
  };

  // ---- drag & drop: OS files upload, internal items move ----

  const isFileDrag = (e: DragEvent) => Array.from(e.dataTransfer.types).includes("Files");

  const dropInto = async (e: DragEvent, target: string) => {
    e.preventDefault();
    e.stopPropagation();
    setDropTarget("");
    setContentDrag(false);
    dragDepth.current = 0;
    if (!isManager) return;
    if (e.dataTransfer.files?.length) return upload(e.dataTransfer.files, target);
    const raw = e.dataTransfer.getData(DRAG_MIME);
    if (!raw) return;
    try {
      const { kind, id } = JSON.parse(raw) as { kind: "folder" | "file"; id: string };
      if (id === target) return;
      if (kind === "file") await m.updateFile.mutateAsync({ id, body: { folderId: target || "" } });
      else await m.updateFolder.mutateAsync({ id, body: { parentId: target || "" } });
      toast.success(fm.moved);
    } catch (err) {
      toast.error(err);
    }
  };

  const folderDragOver = (e: DragEvent, id: string) => {
    if (!isManager) return;
    e.preventDefault();
    e.stopPropagation();
    setDropTarget(id || "__root__");
  };

  const startDrag = (e: DragEvent, kind: "file" | "folder", id: string) => {
    if (!isManager) return;
    e.dataTransfer.setData(DRAG_MIME, JSON.stringify({ kind, id }));
    e.dataTransfer.effectAllowed = "move";
  };

  // ---- menus ----

  const fileMenu = (f: FileResponse): MenuItems => [
    { label: canPreview(f) ? fm.preview : t.open, icon: <IconMaximize size={14} />, onSelect: () => openFile(f) },
    { label: fm.download, icon: <IconDownload size={14} />, onSelect: () => window.open(f.url, "_blank", "noopener") },
    { label: fm.versions, icon: <IconClock size={14} />, onSelect: () => setVersionsFor(f) },
    ...(isManager
      ? [
          { label: f.isPublic ? t.private : t.public, icon: <IconGlobe size={14} />, onSelect: () => togglePublic("file", f.id, !f.isPublic) },
          { label: fm.rename, icon: <IconEdit size={14} />, onSelect: () => setRename({ kind: "file" as const, id: f.id, name: f.name }) },
          { label: fm.delete, icon: <IconTrash size={14} />, danger: true, separated: true, onSelect: () => remove("file", f.id, f.name) },
        ]
      : []),
  ];

  const folderMenu = (n: TreeNode): MenuItems => {
    const id = n.id ?? "";
    const custom = n.kind === "custom";
    return [
      { label: t.open, icon: <IconFolder size={14} />, onSelect: () => openFolder(id) },
      ...(isManager
        ? [
            { label: fm.newSubfolder, icon: <IconPlus size={14} />, onSelect: () => setNewFolderIn(id) },
            { label: n.isPublic ? t.private : t.public, icon: <IconGlobe size={14} />, onSelect: () => togglePublic("folder", id, !n.isPublic) },
            ...(custom
              ? [
                  { label: fm.rename, icon: <IconEdit size={14} />, onSelect: () => setRename({ kind: "folder" as const, id, name: n.name ?? "" }) },
                  { label: fm.delete, icon: <IconTrash size={14} />, danger: true, separated: true, onSelect: () => remove("folder", id, n.name ?? "") },
                ]
              : []),
          ]
        : []),
    ];
  };

  // ---- render ----

  if (tree.isLoading) {
    return (
      <div className="flex flex-col gap-2 p-4">
        {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-row w-full" />)}
      </div>
    );
  }
  if (tree.isError) return <ErrorState error={tree.error} onRetry={() => tree.refetch()} />;

  const viewProps: ViewProps = {
    folders, files, idx, selectedId, showPath: searching, isManager, dropTarget, sort, dir,
    onSort: (k) => setParams(k === sort ? { dir: dir === "asc" ? "desc" : null } : { sort: k === "name" ? null : k, dir: null }),
    onSelect: (f) => { setSelectedId(f.id); setDetailsOpen(true); },
    onOpen: openFile,
    onOpenFolder: openFolder,
    fileMenu,
    folderMenu,
    onFileDragStart: (e, f) => startDrag(e, "file", f.id),
    onFolderDragStart: (e, n) => n.kind === "custom" && startDrag(e, "folder", n.id ?? ""),
    onFolderDragOver: folderDragOver,
    onFolderDragLeave: () => setDropTarget(""),
    onFolderDrop: dropInto,
  };

  const crumbs = current?.path ?? [];
  const currentName = current?.node.name ?? t.allFiles;
  const nothingHere = folders.length === 0 && files.length === 0;
  const showDetails = detailsOpen && !!selected;

  return (
    <div className="flex h-full min-h-0">
      {/* Folder tree */}
      <div className="w-[240px] shrink-0 overflow-y-auto border-r border-line bg-surface p-2 max-md:hidden">
        <FolderTree
          folders={tree.data?.folders ?? []}
          idx={idx}
          current={folderId}
          dropTarget={dropTarget}
          onOpen={openFolder}
          onDragOver={folderDragOver}
          onDragLeave={() => setDropTarget("")}
          onDrop={dropInto}
        />
      </div>

      <div className="flex min-w-0 flex-1 flex-col bg-surface">
        {/* Header: breadcrumb + primary actions */}
        <div className="flex min-h-toolbar flex-wrap items-center gap-2 border-b border-line px-4 py-1.5">
          <nav aria-label={t.location} className="flex min-w-0 flex-1 items-center gap-1 text-[13px]">
            <CrumbButton onClick={() => openFolder("")} current={!searching && crumbs.length === 0}>{t.allFiles}</CrumbButton>
            {crumbs.map((n, i) => (
              <span key={n.id} className="flex min-w-0 items-center gap-1">
                <IconChevronRight size={12} className="shrink-0 text-ink-faint" />
                <CrumbButton onClick={() => openFolder(n.id ?? "")} current={!searching && i === crumbs.length - 1}>{n.name}</CrumbButton>
              </span>
            ))}
          </nav>
          {isManager && (
            <div className="flex items-center gap-1.5">
              <input ref={fileInput} type="file" multiple hidden onChange={(e) => { if (e.target.files) upload(e.target.files, folderId); e.target.value = ""; }} />
              <Button size="sm" variant="ghost" onClick={() => setNewFolderIn(folderId)}><IconPlus size={14} /> {fm.folder}</Button>
              <Button size="sm" variant="primary" loading={busy} onClick={() => fileInput.current?.click()}><IconUpload size={14} /> {fm.upload}</Button>
              <IconButton label={fm.uploadSettings} variant="ghost" size="sm" onClick={() => setSettingsOpen(true)}><IconSettings size={15} /></IconButton>
            </div>
          )}
        </div>

        {/* Toolbar: search, type, sort, view */}
        <div className="flex min-h-toolbar flex-wrap items-center gap-2 border-b border-line px-4 py-1.5">
          <SearchInput value={query} onChange={setQuery} placeholder={t.search} label={t.search} className="w-64 max-sm:w-full" />
          <Select small aria-label={t.type} value={kindParam} onChange={(e) => setParams({ type: e.target.value || null })} className="w-40">
            <option value="">{t.typeAll}</option>
            {FILTER_KINDS.map((k) => <option key={k} value={k}>{t.kind[k]}</option>)}
          </Select>
          <div className="flex items-center gap-1">
            <Select small aria-label={t.sort} value={sort} onChange={(e) => setParams({ sort: e.target.value === "name" ? null : e.target.value })} className="w-40">
              <option value="name">{t.sortName}</option>
              <option value="updated">{t.sortUpdated}</option>
              <option value="size">{t.sortSize}</option>
              <option value="type">{t.sortType}</option>
            </Select>
            <IconButton label={dir === "asc" ? t.asc : t.desc} variant="ghost" size="sm" onClick={() => setParams({ dir: dir === "asc" ? "desc" : null })}>
              {dir === "asc" ? <IconChevronUp size={14} /> : <IconChevronDown size={14} />}
            </IconButton>
          </div>
          <span className="flex-1" />
          <span className="text-[12px] text-ink-faint max-sm:hidden">
            {searching ? t.searchResults(folders.length + files.length, qParam.trim()) : t.items(folders.length, files.length)}
          </span>
          <div role="group" aria-label={t.view} className="flex items-center rounded-md border border-line-strong p-0.5">
            <ViewButton active={view === "grid"} label={t.viewGrid} onClick={() => setParams({ view: null })}><IconGrid size={14} /></ViewButton>
            <ViewButton active={view === "list"} label={t.viewList} onClick={() => setParams({ view: "list" })}><IconList size={14} /></ViewButton>
            <ViewButton active={view === "gallery"} label={t.viewGallery} onClick={() => setParams({ view: "gallery" })}><IconImage size={14} /></ViewButton>
          </div>
          {selected && !detailsOpen && (
            <IconButton label={t.showDetails} variant="ghost" size="sm" onClick={() => setDetailsOpen(true)}><IconColumns size={15} /></IconButton>
          )}
        </div>

        {/* Content: drop OS files anywhere to upload into the current folder */}
        <div
          className="relative min-h-0 flex-1 overflow-auto"
          onDragEnter={(e) => {
            if (!isManager || !isFileDrag(e)) return;
            dragDepth.current += 1;
            setContentDrag(true);
          }}
          onDragLeave={(e) => {
            if (!isManager || !isFileDrag(e)) return;
            dragDepth.current = Math.max(0, dragDepth.current - 1);
            if (dragDepth.current === 0) setContentDrag(false);
          }}
          onDragOver={(e) => {
            if (isManager && isFileDrag(e)) e.preventDefault();
          }}
          onDrop={(e) => dropInto(e, folderId)}
          onClick={(e) => {
            if (e.target === e.currentTarget) setSelectedId(null);
          }}
        >
          <div className={view === "list" ? "" : "p-4"}>
            {nothingHere ? (
              <EmptyState
                className="py-16"
                icon={<IconFolder size={28} />}
                title={searching ? t.emptySearch : kindParam ? t.emptyFiltered : t.emptyFolder}
                description={searching || kindParam ? undefined : isManager ? t.emptyFolderManager : t.emptyFolderMember}
                action={isManager && !searching && !kindParam ? <Button variant="primary" onClick={() => fileInput.current?.click()}><IconUpload size={14} /> {fm.upload}</Button> : undefined}
              />
            ) : view === "list" ? (
              <ListView {...viewProps} />
            ) : view === "gallery" ? (
              <div className="flex flex-col gap-5">
                {folders.length > 0 && <GridView {...viewProps} files={[]} />}
                <GalleryView {...viewProps} />
              </div>
            ) : (
              <GridView {...viewProps} />
            )}
          </div>

          {contentDrag && (
            <div className="pointer-events-none absolute inset-2 grid place-items-center rounded-xl border-2 border-dashed border-accent bg-accent-soft/80">
              <p className="flex items-center gap-2 text-[14px] font-semibold text-accent-ink">
                <IconUpload size={18} /> {t.dropHere(currentName)}
              </p>
            </div>
          )}
          {busy && (
            <div className="pointer-events-none absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full border border-line bg-surface px-3 py-1.5 text-[12px] text-ink-2 shadow-overlay">
              {t.uploading}
            </div>
          )}
        </div>
      </div>

      {showDetails && selected && (
        <FileDetails
          file={selected}
          idx={idx}
          isManager={isManager}
          onClose={() => setDetailsOpen(false)}
          onOpen={() => openFile(selected)}
          onVersions={() => setVersionsFor(selected)}
          onRename={() => setRename({ kind: "file", id: selected.id, name: selected.name })}
          onDelete={() => remove("file", selected.id, selected.name)}
          onTogglePublic={(v) => togglePublic("file", selected.id, v)}
          onOpenFolder={openFolder}
        />
      )}

      {newFolderIn !== null && <NewFolderModal ws={ws} parentId={newFolderIn} onClose={() => setNewFolderIn(null)} />}
      {rename && <RenameModal target={rename} onClose={() => setRename(null)} />}
      {preview && <PreviewModal file={preview} onClose={() => setPreview(null)} />}
      {versionsFor && <VersionsModal ws={ws} file={versionsFor} isManager={isManager} onClose={() => setVersionsFor(null)} />}
      {settingsOpen && <SettingsModal ws={ws} current={settings.data} onClose={() => setSettingsOpen(false)} />}
    </div>
  );
}

function CrumbButton({ children, onClick, current }: { children: ReactNode; onClick: () => void; current: boolean }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-current={current ? "page" : undefined}
      className={`max-w-56 truncate rounded px-1 py-0.5 ${current ? "font-semibold text-ink" : "text-ink-muted hover:bg-hover hover:text-ink"}`}
    >
      {children}
    </button>
  );
}

function ViewButton({ active, label, onClick, children }: { active: boolean; label: string; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      aria-label={label}
      title={label}
      onClick={onClick}
      className={`grid h-6 w-7 place-items-center rounded ${active ? "bg-accent-soft text-accent-ink" : "text-ink-muted hover:bg-hover hover:text-ink"}`}
    >
      {children}
    </button>
  );
}
