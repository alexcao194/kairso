import type { CalendarDayResponse, MemberCalendarDay } from "../api/schema";
import { localDateKey } from "./task";

interface Interval {
  start: number;
  end: number;
}

/**
 * Gộp các khoảng (ms epoch) chồng/liền nhau thành tập khoảng rời, đã sắp xếp.
 * Dùng để cộng thời gian mà KHÔNG tính trùng phần giờ chồng nhau — khi một ngày
 * có nhiều phiên (vd yêu cầu bổ sung công đè lên phiên đã có).
 */
function mergeIntervals(intervals: Interval[]): Interval[] {
  const valid = intervals.filter((iv) => iv.end > iv.start).sort((a, b) => a.start - b.start);
  const out: Interval[] = [];
  for (const iv of valid) {
    const last = out[out.length - 1];
    if (last && iv.start <= last.end) last.end = Math.max(last.end, iv.end);
    else out.push({ ...iv });
  }
  return out;
}

const intervalsHours = (intervals: Interval[]) => mergeIntervals(intervals).reduce((a, iv) => a + (iv.end - iv.start), 0) / 3600000;

/* ── Tính trạng thái ngày từ dữ liệu lịch (CalendarDayResponse) ──────────── */

export type DayState = "full" | "late" | "missed" | "off" | "extra" | "upcoming" | "pending";

interface DayInfo {
  state: DayState;
  /**
   * Giờ công: phần thời gian vào–ra nằm trong các ca ĐÃ DUYỆT — đúng cách backend tính
   * `attendanceHours` cho thống kê/KPI, nên cộng các ngày lại sẽ khớp ô "Giờ làm".
   */
  hours: number;
  /** Thời gian có mặt thực tế (tổng các cặp vào–ra), kể cả ngoài ca. */
  presence: number;
  isLate: boolean;
  isEarlyLeave: boolean;
  scheduled: boolean;
}

/**
 * Trạng thái một ngày từ dữ liệu lịch, cùng quy tắc với thống kê backend
 * (`attendanceHours`, `lateEarlyDays`): chỉ ca đã duyệt mới tính công; ngày < hôm nay
 * có ca đã duyệt mới bị coi là muộn/về sớm/thiếu công; chấm công ngoài lịch không được tính.
 */
export function dayInfo(day: CalendarDayResponse | undefined, key: string): DayInfo {
  const sch = day?.schedule;
  const approved = sch?.status === "approved" ? (sch.sessions ?? []) : [];
  const windows = mergeIntervals(approved.map((s) => ({ start: new Date(s.startAt).getTime(), end: new Date(s.endAt).getTime() })));
  let firstIn: number | undefined;
  let lastOut: number | undefined;
  const present: Interval[] = [];
  for (const s of day?.sessions ?? []) {
    const i = s.checkIn ? new Date(s.checkIn.recordedAt).getTime() : undefined;
    const o = s.checkOut ? new Date(s.checkOut.recordedAt).getTime() : undefined;
    if (i !== undefined) firstIn = firstIn === undefined ? i : Math.min(firstIn, i);
    if (o !== undefined) lastOut = lastOut === undefined ? o : Math.max(lastOut, o);
    if (i !== undefined && o !== undefined && o > i) present.push({ start: i, end: o });
  }
  // Hợp các phiên trước khi cộng để không tính trùng phần giờ chồng nhau.
  const merged = mergeIntervals(present);
  const presence = merged.reduce((a, iv) => a + (iv.end - iv.start), 0) / 3600000;
  let hours = 0;
  for (const p of merged) for (const w of windows) hours += Math.max(0, Math.min(p.end, w.end) - Math.max(p.start, w.start)) / 3600000;
  const scheduled = windows.length > 0;
  const base = { hours, presence, scheduled, isLate: false, isEarlyLeave: false };
  if (sch?.status === "pending") return { ...base, state: "pending" };
  if (key >= localDateKey()) return { ...base, state: scheduled ? "upcoming" : "off" };
  if (!scheduled) return { ...base, state: presence > 0 ? "extra" : "off" };
  if (firstIn === undefined && lastOut === undefined) return { ...base, state: "missed" };
  const start = Math.min(...windows.map((w) => w.start));
  const end = Math.max(...windows.map((w) => w.end));
  const isLate = firstIn !== undefined && firstIn > start;
  const isEarlyLeave = lastOut !== undefined && lastOut < end;
  return { ...base, isLate, isEarlyLeave, state: isLate || isEarlyLeave ? "late" : "full" };
}

export const DAY_STATE_LABEL: Record<DayState, string> = {
  full: "Đủ công",
  late: "Muộn / về sớm",
  missed: "Thiếu công",
  off: "Không có ca",
  extra: "Làm ngoài lịch",
  upcoming: "Sắp tới",
  pending: "Chờ duyệt lịch",
};

/* ── Timeline một ngày của từng thành viên (tab Đội nhóm → Ngày) ─────────── */

/** Trạng thái "trực tiếp" của một thành viên trong ngày đang xem. */
export type LiveState = "working" | "done" | "absent" | "waiting" | "off";

export const LIVE_STATE_ORDER: LiveState[] = ["working", "absent", "waiting", "done", "off"];

/** Một đoạn trên trục giờ, tính bằng phút kể từ 00:00 của ngày đang xem (0..1440). */
interface TimelineBar {
  start: number;
  end: number;
  label: string;
}

interface MemberDayTimeline {
  state: LiveState;
  label: string;
  /** Ca đã duyệt / đang chờ duyệt (ca bị từ chối không vẽ). */
  schedule: Array<TimelineBar & { pending: boolean }>;
  /** Phiên chấm công: `open` = đã vào chưa ra, `stray` = có giờ ra mà không có giờ vào. */
  actual: Array<TimelineBar & { open: boolean; stray: boolean }>;
  hours: number;
  /** Số phút vào muộn so với đầu ca đã duyệt sớm nhất (0 nếu đúng giờ/không có ca). */
  lateMinutes: number;
}

const hhmm = (d: Date) => `${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;

/**
 * Port `MemberTimelineRow` (Flutter): làn trên là ca đăng ký, làn dưới là giờ vào/ra thực tế.
 * Phiên chưa check-out của hôm nay kéo dài tới `now`; ngày khác thì chỉ là một vạch.
 */
export function memberDayTimeline(day: MemberCalendarDay | undefined, key: string, now: Date = new Date()): MemberDayTimeline {
  const [y, m, d] = key.split("-").map(Number);
  const midnight = new Date(y, m - 1, d).getTime();
  const mins = (v: string | Date) => Math.min(1440, Math.max(0, Math.round((new Date(v).getTime() - midnight) / 60000)));
  const today = localDateKey(now);
  const isToday = key === today;
  const nowMin = mins(now);

  const sch = day?.schedule;
  const schedule = sch && sch.status !== "rejected"
    ? (sch.sessions ?? [])
        .map((s) => ({ start: mins(s.startAt), end: mins(s.endAt), label: `${hhmm(new Date(s.startAt))}–${hhmm(new Date(s.endAt))}`, pending: sch.status === "pending" }))
        .filter((b) => b.end > b.start)
        .sort((a, b) => a.start - b.start)
    : [];

  let firstIn: number | undefined;
  const actual: MemberDayTimeline["actual"] = [];
  const present: Interval[] = [];
  for (const s of day?.sessions ?? []) {
    const i = s.checkIn ? new Date(s.checkIn.recordedAt) : undefined;
    const o = s.checkOut ? new Date(s.checkOut.recordedAt) : undefined;
    if (!i && !o) continue;
    if (i) firstIn = firstIn === undefined ? mins(i) : Math.min(firstIn, mins(i));
    if (i && o) {
      if (o > i) present.push({ start: i.getTime(), end: o.getTime() });
      actual.push({ start: mins(i), end: mins(o), label: `${hhmm(i)}–${hhmm(o)}`, open: false, stray: false });
    } else if (i) {
      const end = isToday ? Math.max(nowMin, mins(i)) : mins(i);
      if (isToday && now > i) present.push({ start: i.getTime(), end: now.getTime() });
      actual.push({ start: mins(i), end, label: `${hhmm(i)}–${isToday ? "nay" : "--:--"}`, open: true, stray: false });
    } else if (o) {
      actual.push({ start: mins(o), end: mins(o), label: `--:--–${hhmm(o)}`, open: false, stray: true });
    }
  }
  // Hợp các khoảng để nhiều phiên chồng giờ không bị cộng trùng.
  const hours = intervalsHours(present);
  actual.sort((a, b) => a.start - b.start);

  const approved = schedule.filter((b) => !b.pending);
  const lateMinutes = approved.length && firstIn !== undefined ? Math.max(0, firstIn - approved[0].start) : 0;
  const past = key < today;

  let state: LiveState;
  if (actual.some((a) => a.open) && isToday) state = "working";
  else if (actual.length) state = "done";
  else if (approved.length && (past || (isToday && nowMin > approved[0].start))) state = "absent";
  else if (schedule.length && !past) state = "waiting";
  else state = "off";

  const label = {
    working: "Đang làm",
    done: isToday ? "Đã ra ca" : "Đã chấm công",
    absent: isToday ? "Chưa vào ca" : "Thiếu công",
    waiting: isToday ? "Chưa tới ca" : "Có lịch",
    off: "Không có ca",
  extra: "Làm ngoài lịch",
  }[state];

  return { state, label, schedule, actual, hours, lateMinutes };
}

/** Khung giờ hiển thị: mặc định 07–19h, nới ra để chứa mọi đoạn (và vạch "bây giờ"), làm tròn theo giờ. */
export function timelineWindow(rows: MemberDayTimeline[], nowMin?: number): { from: number; to: number } {
  let lo = 7 * 60;
  let hi = 19 * 60;
  for (const r of rows) {
    for (const b of [...r.schedule, ...r.actual]) {
      lo = Math.min(lo, b.start);
      hi = Math.max(hi, b.end);
    }
  }
  if (nowMin !== undefined) {
    lo = Math.min(lo, nowMin);
    hi = Math.max(hi, nowMin);
  }
  return { from: Math.max(0, Math.floor(lo / 60) * 60 - 60), to: Math.min(1440, Math.ceil(hi / 60) * 60 + 60) };
}
