"use client";

import type { ReactNode } from "react";
import type { FileResponse } from "@/lib/api/schema";
import { canPreview, fileEffPublic, fileExt, fileKind, type FileIndex } from "@/lib/domain/files";
import { fmtDateTime } from "@/lib/domain/format";
import { fmtBytes } from "@/lib/upload";
import { useT } from "@/lib/i18n";
import { Button, IconButton } from "@/components/ui/Button";
import { Switch } from "@/components/ui/form";
import { IconClock, IconClose, IconDownload, IconEdit, IconMaximize, IconTrash } from "@/components/icons";
import { FileGlyph } from "./FileViews";

type Props = {
  file: FileResponse;
  idx: FileIndex;
  isManager: boolean;
  onClose: () => void;
  onOpen: () => void;
  onVersions: () => void;
  onRename: () => void;
  onDelete: () => void;
  onTogglePublic: (v: boolean) => void;
  onOpenFolder: (id: string) => void;
};

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[92px_1fr] gap-2 py-1.5 text-[12.5px]">
      <dt className="text-ink-faint">{label}</dt>
      <dd className="min-w-0 break-words text-ink">{children}</dd>
    </div>
  );
}

/** Right-hand panel describing the selected file. */
export function FileDetails(p: Props) {
  const all = useT();
  const t = all.fx;
  const f = p.file;
  const kind = fileKind(f);
  const folder = f.folderId ? p.idx.folders.get(f.folderId) : undefined;
  const eff = fileEffPublic(p.idx, f);

  return (
    <aside aria-label={t.details} className="flex h-full w-[300px] shrink-0 flex-col border-l border-line bg-surface max-lg:hidden">
      <div className="flex h-toolbar shrink-0 items-center gap-2 border-b border-line px-3">
        <span className="text-[12px] font-semibold uppercase tracking-[0.08em] text-ink-faint">{t.details}</span>
        <span className="flex-1" />
        <IconButton label={t.hideDetails} variant="ghost" size="sm" onClick={p.onClose}><IconClose size={14} /></IconButton>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        <button
          type="button"
          onClick={p.onOpen}
          className="grid aspect-[4/3] w-full place-items-center overflow-hidden rounded-lg border border-line bg-surface-sunken"
          aria-label={canPreview(f) ? all.fm.preview : t.open}
        >
          {kind === "image" ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={f.url} alt={f.name} className="size-full object-contain" />
          ) : (
            <FileGlyph file={f} size={64} />
          )}
        </button>

        <h2 className="mt-3 break-words text-[14px] font-semibold text-ink">{f.name}</h2>

        <div className="mt-3 flex flex-wrap gap-1.5">
          <Button size="sm" variant="primary" onClick={p.onOpen}>
            <IconMaximize size={13} /> {canPreview(f) ? all.fm.preview : t.open}
          </Button>
          <a href={f.url} target="_blank" rel="noreferrer" download>
            <Button size="sm"><IconDownload size={13} /> {all.fm.download}</Button>
          </a>
        </div>

        <dl className="mt-4 divide-y divide-line-row border-y border-line-row">
          <Row label={t.type}>{t.kind[kind]}{fileExt(f.name) ? ` (.${fileExt(f.name)})` : ""}</Row>
          <Row label={t.size}>{fmtBytes(f.sizeBytes)}</Row>
          <Row label={t.location}>
            {folder ? (
              <span className="flex flex-wrap items-center gap-x-1">
                {folder.path.map((n, i) => (
                  <span key={n.id} className="inline-flex items-center gap-1">
                    {i > 0 && <span className="text-ink-faint">/</span>}
                    <button type="button" onClick={() => p.onOpenFolder(n.id ?? "")} className="text-accent-ink hover:underline">{n.name}</button>
                  </span>
                ))}
              </span>
            ) : (
              <button type="button" onClick={() => p.onOpenFolder("")} className="text-accent-ink hover:underline">{t.allFiles}</button>
            )}
          </Row>
          <Row label={t.uploadedAt}>{fmtDateTime(f.createdAt)}</Row>
          <Row label={t.updatedAt}>{fmtDateTime(f.updatedAt)}</Row>
        </dl>

        <div className="mt-4">
          <p className="mb-1.5 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-faint">{t.sharing}</p>
          {p.isManager ? (
            <Switch label={f.isPublic ? t.public : t.private} checked={f.isPublic} onChange={p.onTogglePublic} />
          ) : (
            <span className="text-[12.5px] text-ink">{f.isPublic ? t.public : t.private}</span>
          )}
          {f.isPublic && !eff && <p className="mt-1.5 text-[12px] text-(--amber)">{t.publicHidden}</p>}
          <p className="mt-1.5 text-[11.5px] text-ink-faint">{t.publicHint}</p>
        </div>

        {p.isManager && (
          <div className="mt-4 flex flex-col gap-1">
            <Button size="sm" variant="ghost" className="justify-start" onClick={p.onVersions}><IconClock size={13} /> {all.fm.versions}</Button>
            <Button size="sm" variant="ghost" className="justify-start" onClick={p.onRename}><IconEdit size={13} /> {all.fm.rename}</Button>
            <Button size="sm" variant="ghost" className="justify-start text-danger" onClick={p.onDelete}><IconTrash size={13} /> {all.fm.delete}</Button>
          </div>
        )}
        {!p.isManager && (
          <Button size="sm" variant="ghost" className="mt-4 justify-start" onClick={p.onVersions}><IconClock size={13} /> {all.fm.versions}</Button>
        )}
      </div>
    </aside>
  );
}
