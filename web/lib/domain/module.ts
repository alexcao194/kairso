import type { ModuleResponse, PublicModuleResponse } from "../api/schema";
import { localDateKey } from "./task";

/* ── Scrum ────────────────────────────────────────────────────────────────
   Module theo Scrum: trạng thái vòng đời + các sprint riêng của module. */

export type ModuleStatus = ModuleResponse["status"];
export type ModuleSprint = ModuleResponse["sprints"][number];
export type SprintStatus = ModuleSprint["status"];

export const MODULE_STATUS_ORDER: ModuleStatus[] = ["backlog", "in_progress", "done", "released"];
export const MODULE_STATUS_LABEL: Record<ModuleStatus, string> = {
  backlog: "Product Backlog",
  in_progress: "Đang phát triển",
  done: "Done",
  released: "Đã phát hành",
};
export const MODULE_STATUS_COLOR: Record<ModuleStatus, string> = {
  backlog: "var(--ink-faint)",
  in_progress: "var(--blue-vivid)",
  done: "var(--green-vivid)",
  released: "var(--violet-vivid)",
};

export const SPRINT_STATUS_LABEL: Record<SprintStatus, string> = { planned: "Lên kế hoạch", active: "Đang chạy", completed: "Đã xong" };
export const SPRINT_STATUS_COLOR: Record<SprintStatus, string> = {
  planned: "var(--ink-faint)",
  active: "var(--blue-vivid)",
  completed: "var(--green-vivid)",
};

/** Sprint tối đa 1 tháng theo Scrum Guide — khớp backend (maxSprintDays). */
const MAX_SPRINT_DAYS = 31;
const DEFAULT_SPRINT_DAYS = 14;

export const nextModuleStatus = (s: ModuleStatus): ModuleStatus | null => MODULE_STATUS_ORDER[MODULE_STATUS_ORDER.indexOf(s) + 1] ?? null;

/** "2026-10-02" → Date lúc 00:00 giờ máy (không lệch múi giờ như new Date("2026-10-02")). */
const dateOnly = (v: string) => {
  const [y, m, d] = v.split("-").map(Number);
  return new Date(y, m - 1, d);
};
const addDays = (key: string, n: number) => {
  const d = dateOnly(key);
  d.setDate(d.getDate() + n);
  return localDateKey(d);
};
/** Số ngày của sprint, tính cả hai đầu. */
export const sprintDays = (s: Pick<ModuleSprint, "startDate" | "endDate">) => Math.round((dateOnly(s.endDate).getTime() - dateOnly(s.startDate).getTime()) / 86_400_000) + 1;

export const activeSprint = (m: Pick<ModuleResponse, "sprints">) => (m.sprints ?? []).find((s) => s.status === "active");

/** Sprint gợi ý tiếp theo: ngay sau sprint cuối (hoặc hôm nay), dài bằng sprint cuối (mặc định 2 tuần). */
export function nextSprintDraft(m: Pick<ModuleResponse, "sprints">): { name: string; startDate: string; endDate: string } {
  const list = m.sprints ?? [];
  const last = list[list.length - 1];
  const start = last ? addDays(last.endDate, 1) : localDateKey();
  const len = last ? Math.min(sprintDays(last), MAX_SPRINT_DAYS) : DEFAULT_SPRINT_DAYS;
  return { name: `Sprint ${list.length + 1}`, startDate: start, endDate: addDays(start, len - 1) };
}

/** Lỗi time-box của một sprint so với các sprint khác — khớp kiểm tra ở backend để báo sớm. */
export function sprintProblem(s: Pick<ModuleSprint, "id" | "startDate" | "endDate">, all: ModuleSprint[]): string | null {
  if (!s.startDate || !s.endDate) return "Chọn ngày bắt đầu và kết thúc.";
  if (s.endDate < s.startDate) return "Ngày kết thúc phải từ ngày bắt đầu trở đi.";
  const days = sprintDays(s);
  if (days > MAX_SPRINT_DAYS) return `Sprint dài ${days} ngày — tối đa 1 tháng (${MAX_SPRINT_DAYS} ngày).`;
  const clash = all.find((o) => o.id !== s.id && o.startDate <= s.endDate && s.startDate <= o.endDate);
  return clash ? `Trùng thời gian với ${clash.name}.` : null;
}

/** "Sprint 2 · còn 5 ngày" cho sprint đang chạy; "Sprint 3 bắt đầu 13/10" nếu chưa chạy sprint nào. */
export function sprintSummary(m: Pick<ModuleResponse, "sprints">, today = localDateKey()): string {
  const a = activeSprint(m);
  if (a) {
    const left = Math.round((dateOnly(a.endDate).getTime() - dateOnly(today).getTime()) / 86_400_000);
    return `${a.name} · ${left > 0 ? `còn ${left} ngày` : left === 0 ? "hôm nay kết thúc" : `quá ${-left} ngày`}`;
  }
  const next = (m.sprints ?? []).find((s) => s.status === "planned");
  if (next) return `${next.name} bắt đầu ${next.startDate.slice(8, 10)}/${next.startDate.slice(5, 7)}`;
  const n = (m.sprints ?? []).length;
  return n ? `${n} sprint đã xong` : "Chưa có sprint";
}

/** Sprint đang chạy đã qua ngày kết thúc mà chưa đóng. */
export const isSprintOverdue = (s: ModuleSprint, today = localDateKey()) => s.status === "active" && s.endDate < today;

/** Quá hạn phát hành: có ngày phát hành mục tiêu đã qua mà chưa Done/Đã phát hành. */
export function isReleaseOverdue(m: Pick<ModuleResponse, "endDate" | "status">): boolean {
  return !!m.endDate && m.status !== "done" && m.status !== "released" && localDateKey(new Date(m.endDate)) < localDateKey();
}

/** Một thanh trên timeline module (cùng hình dạng `GanttRow["bars"][number]`). */
interface TimelineBar {
  start: Date;
  end: Date;
  color: string;
  label: string;
  hint?: string;
}

type TimelineModule = Pick<PublicModuleResponse, "name" | "status" | "startDate" | "endDate" | "releasedAt"> & {
  sprints: Array<{ name: string; startDate: string; endDate: string; status: SprintStatus; goal?: string }>;
};

/**
 * Timeline theo Scrum, dùng chung cho trang Module và trang phản hồi công khai: mỗi sprint một thanh
 * (màu theo trạng thái sprint); module chưa chia sprint thì một thanh từ ngày bắt đầu tới ngày phát hành;
 * cộng một mốc 1 ngày cho ngày phát hành (thực tế nếu đã phát hành, không thì mục tiêu).
 */
export function moduleTimelineBars(m: TimelineModule, fmt: (d: Date) => string): TimelineBar[] {
  const bars: TimelineBar[] = m.sprints.length
    ? m.sprints.map((s) => ({
        start: dateOnly(s.startDate),
        end: dateOnly(s.endDate),
        color: SPRINT_STATUS_COLOR[s.status],
        label: s.name,
        hint: `${m.name} · ${s.name} (${SPRINT_STATUS_LABEL[s.status]}): ${fmt(dateOnly(s.startDate))} → ${fmt(dateOnly(s.endDate))}${s.goal ? ` — ${s.goal}` : ""}`,
      }))
    : m.startDate
      ? [{ start: new Date(m.startDate), end: new Date(m.endDate ?? m.startDate), color: MODULE_STATUS_COLOR[m.status as ModuleStatus], label: MODULE_STATUS_LABEL[m.status as ModuleStatus] }]
      : [];
  const release = m.status === "released" && m.releasedAt ? m.releasedAt : m.endDate;
  if (release) {
    const d = new Date(release);
    bars.push({ start: d, end: d, color: MODULE_STATUS_COLOR.released, label: "Phát hành", hint: `${m.name} · ${m.status === "released" ? "đã phát hành" : "phát hành dự kiến"} ${fmt(d)}` });
  }
  return bars;
}
