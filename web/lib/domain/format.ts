const VN = "vi-VN";

const toDate = (v: string | Date | null | undefined) => (v ? (v instanceof Date ? v : new Date(v)) : null);

/** 05/03/2026 */
export function fmtDate(v?: string | Date | null): string {
  const d = toDate(v);
  return d ? d.toLocaleDateString(VN, { day: "2-digit", month: "2-digit", year: "numeric" }) : "—";
}

/** 05/03 */
export function fmtDayMonth(v?: string | Date | null): string {
  const d = toDate(v);
  return d ? d.toLocaleDateString(VN, { day: "2-digit", month: "2-digit" }) : "—";
}

/** 09:30 */
export function fmtTime(v?: string | Date | null): string {
  const d = toDate(v);
  return d ? d.toLocaleTimeString(VN, { hour: "2-digit", minute: "2-digit", hour12: false }) : "—";
}

export const fmtDateTime = (v?: string | Date | null) => (toDate(v) ? `${fmtTime(v)} ${fmtDate(v)}` : "—");

/** "vừa xong", "5 phút trước", "3 giờ trước", "2 ngày trước", còn lại là ngày. */
export function fmtRelative(v?: string | Date | null): string {
  const d = toDate(v);
  if (!d) return "";
  const diff = (Date.now() - d.getTime()) / 1000;
  if (diff < 45) return "vừa xong";
  if (diff < 3600) return `${Math.round(diff / 60)} phút trước`;
  if (diff < 86400) return `${Math.round(diff / 3600)} giờ trước`;
  if (diff < 86400 * 7) return `${Math.round(diff / 86400)} ngày trước`;
  return fmtDate(d);
}

export function initials(name?: string | null): string {
  const parts = (name ?? "").trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

/** Băm chuỗi → chỉ số ổn định, dùng để chọn màu nhận diện cho avatar/dự án. */
function hashIndex(seed: string, mod: number): number {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) | 0;
  return Math.abs(h) % mod;
}

const AVATAR_COUNT = 8;
/** Nền avatar: gradient ổn định theo id/tên (chữ trắng, đã chọn cặp màu đủ đậm). */
export const avatarColor = (seed: string) => `var(--grad-${hashIndex(seed, AVATAR_COUNT) + 1})`;
export const projectColor = (seed: string) => `var(--avatar-${hashIndex(`p:${seed}`, AVATAR_COUNT) + 1})`;

/** Ngày đầu tuần (thứ Hai) chứa `d`. */
export function startOfWeek(d: Date): Date {
  const r = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const dow = (r.getDay() + 6) % 7;
  r.setDate(r.getDate() - dow);
  return r;
}

export const addDays = (d: Date, n: number) => {
  const r = new Date(d);
  r.setDate(r.getDate() + n);
  return r;
};

/** "2026-03" */
export const monthKey = (d: Date) => `${d.getFullYear()}-${`${d.getMonth() + 1}`.padStart(2, "0")}`;
