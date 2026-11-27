/**
 * Which festive theme the app wears on a given day: a holiday when one is on (world holidays
 * plus the Vietnamese lunar ones), otherwise the season (northern hemisphere, as in Vietnam).
 * Pure functions on local dates, so they can be tested and previewed with ?season=<id>.
 */

export type Particle = "snow" | "leaf" | "petal" | "blossom" | "heart" | "confetti" | "bat" | "spark";

export type SeasonId = "spring" | "summer" | "autumn" | "winter";
export type HolidayId = "newyear" | "tet" | "valentine" | "womensday" | "midautumn" | "halloween" | "christmas";
export type ThemeId = SeasonId | HolidayId;

export interface FestiveTheme {
  id: ThemeId;
  holiday: boolean;
  /** What drifts across the screen; null for none. */
  particle: Particle | null;
}

// Lunar dates have no simple formula: Tết (1/1 âm lịch) and Trung thu (15/8 âm lịch), Vietnam time.
const TET: Record<number, string> = {
  2025: "01-29", 2026: "02-17", 2027: "02-06", 2028: "01-26", 2029: "02-13",
  2030: "02-03", 2031: "01-23", 2032: "02-11", 2033: "01-31", 2034: "02-19", 2035: "02-08",
};
const MID_AUTUMN: Record<number, string> = {
  2025: "10-06", 2026: "09-25", 2027: "09-15", 2028: "10-03", 2029: "09-22",
  2030: "09-12", 2031: "10-01", 2032: "09-19", 2033: "09-08", 2034: "09-27", 2035: "09-16",
};

const PARTICLE: Record<ThemeId, Particle | null> = {
  spring: "petal",
  summer: "spark",
  autumn: "leaf",
  winter: "snow",
  newyear: "confetti",
  tet: "blossom",
  valentine: "heart",
  womensday: "petal",
  midautumn: "spark",
  halloween: "bat",
  christmas: "snow",
};

const HOLIDAYS: HolidayId[] = ["newyear", "tet", "valentine", "womensday", "midautumn", "halloween", "christmas"];
export const THEME_IDS: ThemeId[] = [...HOLIDAYS, "spring", "summer", "autumn", "winter"];

const DAY = 86_400_000;
/** How many days before and after a holiday its theme shows. */
export const HOLIDAY_REACH = 5;

const at = (y: number, md: string) => {
  const [m, d] = md.split("-").map(Number);
  return new Date(y, m - 1, d);
};
const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate());
/** Whole days from `a` to `b` (local calendar days, DST-safe). */
const dayDiff = (a: Date, b: Date) => Math.round((startOfDay(b).getTime() - startOfDay(a).getTime()) / DAY);

/** The day each holiday falls on in year y (lunar ones only where known). */
function holidaysIn(y: number): [HolidayId, Date][] {
  const out: [HolidayId, Date][] = [
    ["newyear", new Date(y, 0, 1)],
    ["valentine", new Date(y, 1, 14)],
    ["womensday", new Date(y, 2, 8)],
    ["halloween", new Date(y, 9, 31)],
    ["christmas", new Date(y, 11, 25)],
  ];
  if (TET[y]) out.push(["tet", at(y, TET[y])]);
  if (MID_AUTUMN[y]) out.push(["midautumn", at(y, MID_AUTUMN[y])]);
  return out;
}

/**
 * The holiday whose theme shows on date d: the nearest holiday within HOLIDAY_REACH days.
 * When two holidays are closer than that, the days between them are split at the midpoint
 * (each day goes to the nearer one; an exact tie goes to the upcoming holiday).
 */
export function holidayOn(d: Date): HolidayId | null {
  const y = d.getFullYear();
  let best: { id: HolidayId; dist: number; ahead: boolean } | null = null;
  for (const year of [y - 1, y, y + 1]) {
    for (const [id, day] of holidaysIn(year)) {
      const diff = dayDiff(d, day); // > 0: holiday is ahead
      const dist = Math.abs(diff);
      if (dist > HOLIDAY_REACH) continue;
      const ahead = diff >= 0;
      if (!best || dist < best.dist || (dist === best.dist && ahead && !best.ahead)) best = { id, dist, ahead };
    }
  }
  return best?.id ?? null;
}

/** Meteorological seasons, northern hemisphere. */
export function seasonOn(d: Date): SeasonId {
  const m = d.getMonth() + 1;
  if (m >= 3 && m <= 5) return "spring";
  if (m >= 6 && m <= 8) return "summer";
  if (m >= 9 && m <= 11) return "autumn";
  return "winter";
}

export function themeById(id: ThemeId): FestiveTheme {
  return { id, holiday: (HOLIDAYS as string[]).includes(id), particle: PARTICLE[id] };
}

export function themeOn(d: Date): FestiveTheme {
  return themeById(holidayOn(d) ?? seasonOn(d));
}

export const isThemeId = (v: string | null | undefined): v is ThemeId => !!v && (THEME_IDS as string[]).includes(v);
