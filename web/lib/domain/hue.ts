import type { CSSProperties } from "react";

/** 9 màu ngữ nghĩa của giao diện (token `--blue`, `--blue-soft`… trong globals.css). */
export type Hue = "blue" | "violet" | "pink" | "rose" | "orange" | "amber" | "green" | "teal" | "cyan";

/**
 * Gán bộ biến `--h` (chữ/icon đậm), `--hs` (nền nhạt), `--hl` (viền), `--hv` (khối đặc) cho một phần tử;
 * bên trong dùng `text-(--h) bg-(--hs) border-(--hl) bg-(--hv)`. Nhờ vậy một component nhận `hue` mà
 * không phải liệt kê class của từng màu.
 */
export const hueVars = (h: Hue): CSSProperties =>
  ({ "--h": `var(--${h})`, "--hs": `var(--${h}-soft)`, "--hl": `var(--${h}-line)`, "--hv": `var(--${h}-vivid)` }) as CSSProperties;

/** Màu cố định cho từng loại mục trong điều hướng — người dùng nhận ra chỗ đang ở bằng màu, không chỉ bằng chữ. */
export const NAV_HUE = {
  home: "violet",
  inbox: "rose",
  myTasks: "blue",
  threads: "cyan",
  calendar: "orange",
  initiatives: "amber",
  analytics: "pink",
  files: "blue",
  settings: "green",
} as const satisfies Record<string, Hue>;
