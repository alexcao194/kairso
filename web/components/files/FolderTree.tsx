"use client";

import { useEffect, useState, type DragEvent } from "react";
import type { TreeNode } from "@/lib/api/schema";
import type { FileIndex } from "@/lib/domain/files";
import { useT } from "@/lib/i18n";
import { IconChevronRight, IconFolder, IconLayers } from "@/components/icons";
import { hueVars } from "@/lib/domain/hue";

type Props = {
  folders: TreeNode[];
  idx: FileIndex;
  current: string;
  dropTarget: string;
  onOpen: (id: string) => void;
  onDragOver: (e: DragEvent, id: string) => void;
  onDragLeave: () => void;
  onDrop: (e: DragEvent, id: string) => void;
};

/** Sidebar folder tree. Expands the path to the current folder automatically. */
export function FolderTree(p: Props) {
  const t = useT().fx;
  const [open, setOpen] = useState<Set<string>>(new Set());

  useEffect(() => {
    const info = p.current ? p.idx.folders.get(p.current) : undefined;
    if (!info) return;
    setOpen((prev) => {
      const next = new Set(prev);
      info.path.forEach((n) => n.id && next.add(n.id));
      return next;
    });
  }, [p.current, p.idx]);

  const toggle = (id: string) =>
    setOpen((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  return (
    <nav aria-label={t.folderTree} className="flex flex-col gap-px text-[13px]">
      <button
        type="button"
        onClick={() => p.onOpen("")}
        onDragOver={(e) => p.onDragOver(e, "")}
        onDragLeave={p.onDragLeave}
        onDrop={(e) => p.onDrop(e, "")}
        aria-current={p.current === "" ? "page" : undefined}
        className={`flex h-8 items-center gap-2 rounded-md px-2 text-left ${p.current === "" ? "bg-accent-soft font-semibold text-accent-ink" : "text-ink-2 hover:bg-hover"} ${p.dropTarget === "__root__" ? "ring-2 ring-accent" : ""}`}
      >
        <IconLayers size={15} />
        {t.allFiles}
      </button>
      {p.folders.map((n) => (
        <Node key={n.id} node={n} depth={0} open={open} toggle={toggle} {...p} />
      ))}
    </nav>
  );
}

function Node({ node, depth, open, toggle, ...p }: Props & { node: TreeNode; depth: number; open: Set<string>; toggle: (id: string) => void }) {
  const all = useT();
  const t = all.fx;
  const id = node.id ?? "";
  const kids = node.children ?? [];
  const expanded = open.has(id);
  const active = p.current === id;
  const hue = node.kind === "project" ? "violet" : node.kind === "module" ? "blue" : "amber";
  return (
    <div>
      <div
        onDragOver={(e) => p.onDragOver(e, id)}
        onDragLeave={p.onDragLeave}
        onDrop={(e) => p.onDrop(e, id)}
        style={{ paddingLeft: depth * 14 }}
        className={`flex h-8 items-center rounded-md ${active ? "bg-accent-soft" : "hover:bg-hover"} ${p.dropTarget === id ? "ring-2 ring-accent" : ""}`}
      >
        <button
          type="button"
          aria-label={expanded ? all.fm.collapse : all.fm.expand}
          aria-expanded={kids.length ? expanded : undefined}
          onClick={() => kids.length && toggle(id)}
          className={`grid size-6 shrink-0 place-items-center text-ink-faint ${kids.length ? "" : "invisible"}`}
        >
          <IconChevronRight size={12} className={`transition-transform ${expanded ? "rotate-90" : ""}`} />
        </button>
        <button
          type="button"
          onClick={() => p.onOpen(id)}
          aria-current={active ? "page" : undefined}
          title={node.kind === "custom" ? node.name : `${node.name} — ${node.kind === "project" ? t.projectFolder : t.moduleFolder}`}
          className={`flex min-w-0 flex-1 items-center gap-1.5 pr-2 text-left ${active ? "font-semibold text-accent-ink" : "text-ink-2"}`}
        >
          <span style={hueVars(hue)} className="shrink-0 text-(--h)"><IconFolder size={15} /></span>
          <span className="truncate">{node.name}</span>
        </button>
      </div>
      {expanded && kids.map((c) => <Node key={c.id} node={c} depth={depth + 1} open={open} toggle={toggle} {...p} />)}
    </div>
  );
}
