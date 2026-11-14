"use client";

import { useRef, useState } from "react";
import type { FileResponse } from "@/lib/api/schema";
import { useWorkspace } from "@/lib/workspace/WorkspaceContext";
import { useToast } from "@/lib/toast/ToastProvider";
import { presignAndUpload, useFileMutations, useFileVersions } from "@/lib/hooks/files";
import { fileKind } from "@/lib/domain/files";
import { fmtBytes } from "@/lib/upload";
import { dateLocale, tr } from "@/lib/i18n";
import { Button, IconButton } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Field, Input } from "@/components/ui/form";
import { Skeleton } from "@/components/ui/feedback";
import { IconClock, IconDownload, IconUpload } from "@/components/icons";

export type RenameTarget = { kind: "folder" | "file"; id: string; name: string };

export function NewFolderModal({ ws, parentId, onClose }: { ws: string; parentId: string; onClose: () => void }) {
  const m = useFileMutations(ws);
  const toast = useToast();
  const [name, setName] = useState("");
  const save = async () => {
    if (!name.trim()) return;
    try {
      await m.createFolder.mutateAsync({ parentId: parentId || "", name: name.trim(), isPublic: false });
      toast.success(tr().fm.folderCreated);
      onClose();
    } catch (e) {
      toast.error(e);
    }
  };
  return (
    <Modal open onClose={onClose} title={tr().fm.newFolder} footer={<><Button variant="ghost" onClick={onClose}>{tr().fm.cancel}</Button><Button variant="primary" loading={m.createFolder.isPending} onClick={save} disabled={!name.trim()}>{tr().fm.create}</Button></>}>
      <Field label={tr().fm.folderName}>{(p) => <Input {...p} autoFocus value={name} onChange={(e) => setName(e.target.value)} onKeyDown={(e) => e.key === "Enter" && save()} placeholder={tr().fm.folderPh} />}</Field>
    </Modal>
  );
}

export function RenameModal({ target, onClose }: { target: RenameTarget; onClose: () => void }) {
  const { workspaceId: ws } = useWorkspace();
  const m = useFileMutations(ws);
  const toast = useToast();
  const [name, setName] = useState(target.name);
  const save = async () => {
    if (!name.trim()) return;
    try {
      if (target.kind === "file") await m.updateFile.mutateAsync({ id: target.id, body: { name: name.trim() } });
      else await m.updateFolder.mutateAsync({ id: target.id, body: { name: name.trim() } });
      toast.success(tr().fm.renamed);
      onClose();
    } catch (e) {
      toast.error(e);
    }
  };
  return (
    <Modal open onClose={onClose} title={tr().fm.rename} footer={<><Button variant="ghost" onClick={onClose}>{tr().fm.cancel}</Button><Button variant="primary" onClick={save} disabled={!name.trim()}>{tr().fm.save}</Button></>}>
      <Field label={tr().fm.newName}>{(p) => <Input {...p} autoFocus value={name} onChange={(e) => setName(e.target.value)} onKeyDown={(e) => e.key === "Enter" && save()} />}</Field>
    </Modal>
  );
}

export function PreviewModal({ file, onClose }: { file: FileResponse; onClose: () => void }) {
  const kind = fileKind(file);
  const name = file.name || tr().fm.file;
  const url = file.url;
  return (
    <Modal
      open
      onClose={onClose}
      title={name}
      width={900}
      footer={
        <a href={url} target="_blank" rel="noreferrer" download>
          <Button variant="ghost"><IconDownload size={14} /> {tr().fm.download}</Button>
        </a>
      }
    >
      {kind === "image" ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={url} alt={name} className="mx-auto max-h-[72vh] rounded-md" />
      ) : kind === "video" ? (
        <video src={url} controls className="mx-auto max-h-[72vh] w-full rounded-md bg-surface-sunken" />
      ) : kind === "audio" ? (
        <audio src={url} controls className="w-full" />
      ) : (
        <iframe src={url} title={name} className="h-[72vh] w-full rounded-md border border-line" />
      )}
    </Modal>
  );
}

export function VersionsModal({ ws, file, isManager, onClose }: { ws: string; file: FileResponse; isManager: boolean; onClose: () => void }) {
  const versions = useFileVersions(ws, file.id);
  const m = useFileMutations(ws);
  const toast = useToast();
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

  const addVersion = async (f: File) => {
    setBusy(true);
    try {
      const up = await presignAndUpload(ws, f);
      await m.addVersion.mutateAsync({ id: file.id, body: { objectName: up.objectName, contentType: up.contentType, sizeBytes: up.sizeBytes } });
      toast.success(tr().fm.versionUploaded);
    } catch (e) {
      toast.error(e);
    } finally {
      setBusy(false);
    }
  };
  const restore = async (versionId: string) => {
    try {
      await m.restoreVersion.mutateAsync({ id: file.id, versionId });
      toast.success(tr().fm.versionRestored);
    } catch (e) {
      toast.error(e);
    }
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={tr().fm.versionsOf(file.name)}
      width={560}
      footer={
        isManager ? (
          <>
            <input ref={input} type="file" hidden onChange={(e) => { if (e.target.files?.[0]) addVersion(e.target.files[0]); e.target.value = ""; }} />
            <Button variant="primary" loading={busy} onClick={() => input.current?.click()}><IconUpload size={14} /> {tr().fm.uploadVersion}</Button>
          </>
        ) : undefined
      }
    >
      {versions.isLoading && <Skeleton className="h-16 w-full" />}
      {versions.data && versions.data.length === 0 && <p className="text-[12.5px] text-ink-muted">{tr().fm.noVersions}</p>}
      <ul className="flex flex-col gap-1">
        {versions.data?.map((v, i) => (
          <li key={v.id} className="flex items-center gap-2 rounded-md border border-line px-2.5 py-1.5">
            <IconClock size={14} className="text-ink-faint" />
            <div className="min-w-0 flex-1">
              <p className="text-[12.5px] text-ink">
                {i === 0 ? tr().fm.current : tr().fm.versionN(versions.data!.length - i)} · <span className="font-mono text-[11px] text-ink-faint">{fmtBytes(v.sizeBytes)}</span>
              </p>
              <p className="text-[11px] text-ink-faint">{new Date(v.createdAt).toLocaleString(dateLocale())}</p>
            </div>
            <a href={v.url} target="_blank" rel="noreferrer" download>
              <IconButton label={tr().fm.download} variant="ghost" size="sm"><IconDownload size={14} /></IconButton>
            </a>
            {isManager && i !== 0 && <Button size="sm" variant="ghost" onClick={() => restore(v.id)}>{tr().fm.restore}</Button>}
          </li>
        ))}
      </ul>
    </Modal>
  );
}

export function SettingsModal({ ws, current, onClose }: { ws: string; current?: { maxFileSizeMb: number; allowedTypes: string }; onClose: () => void }) {
  const m = useFileMutations(ws);
  const toast = useToast();
  const [maxMB, setMaxMB] = useState(String(current?.maxFileSizeMb ?? 50));
  const [types, setTypes] = useState(current?.allowedTypes ?? "");
  const save = async () => {
    const n = parseInt(maxMB, 10);
    if (!n || n < 1 || n > 5000) return toast.error(tr().fm.maxSizeRange);
    try {
      await m.saveSettings.mutateAsync({ maxFileSizeMb: n, allowedTypes: types.trim() });
      toast.success(tr().fm.settingsSaved);
      onClose();
    } catch (e) {
      toast.error(e);
    }
  };
  return (
    <Modal
      open
      onClose={onClose}
      title={tr().fm.uploadSettings}
      description={tr().fm.settingsDesc}
      footer={<><Button variant="ghost" onClick={onClose}>{tr().fm.cancel}</Button><Button variant="primary" loading={m.saveSettings.isPending} onClick={save}>{tr().fm.save}</Button></>}
    >
      <div className="flex flex-col gap-3">
        <Field label={tr().fm.maxSize}>{(p) => <Input {...p} type="number" min={1} max={5000} value={maxMB} onChange={(e) => setMaxMB(e.target.value)} />}</Field>
        <Field label={tr().fm.allowedTypes} hint={tr().fm.allowedHint}>{(p) => <Input {...p} value={types} onChange={(e) => setTypes(e.target.value)} placeholder="pdf,png,jpg,docx" />}</Field>
      </div>
    </Modal>
  );
}
