"use client";

import { useId, type ReactNode } from "react";
import type { HolidayId, ThemeId } from "@/lib/domain/season";

/**
 * More festive pieces woven into the layout: a garland hanging along the top of the page on
 * holidays, a small scene at the foot of the sidebar, and a snow cap on the hero banner in
 * winter. All decorative: aria-hidden, no pointer events, colours from tokens only.
 */

const c = (v: string) => `var(--${v})`;

// Static class names so Tailwind generates them: bulbs blink out of step.
const BLINK = [
  "motion-safe:animate-[fx-blink_2s_ease-in-out_infinite]",
  "motion-safe:animate-[fx-blink_2.7s_ease-in-out_infinite]",
  "motion-safe:animate-[fx-blink_3.4s_ease-in-out_infinite]",
];

// ---------- Garland ----------

/** One repeat of the garland: a sagging string with four hanging items (pattern is 256 × 22). */
const GARLAND: Partial<Record<HolidayId, (k: number) => ReactNode>> = {
  christmas: (k) => {
    const colors = ["rose-vivid", "amber-vivid", "green-vivid", "blue-vivid"];
    return (
      <>
        <path d="M0 3 Q32 12 64 3 T128 3 T192 3 T256 3" fill="none" stroke={c("ink-faint")} strokeWidth="1" opacity="0.7" />
        {colors.map((col, i) => (
          <g key={col} transform={`translate(${32 + i * 64} 7.5)`}>
            <rect x="-2" y="-1" width="4" height="3" rx="0.8" fill={c("ink-faint")} />
            <ellipse cx="0" cy="6" rx="3.6" ry="5" fill={c(col)} className={BLINK[(i + k) % BLINK.length]} />
          </g>
        ))}
      </>
    );
  },
  newyear: () => (
    <>
      <path d="M0 2 Q32 10 64 2 T128 2 T192 2 T256 2" fill="none" stroke={c("fx-gold")} strokeWidth="1.4" />
      {[16, 48, 80, 112, 144, 176, 208, 240].map((x, i) => (
        <path key={x} d={`M${x} ${4 + (i % 2) * 2} q3 5 0 9 q-3 4 0 8`} fill="none" stroke={c(["fx-gold", "rose-vivid", "blue-vivid", "green-vivid"][i % 4])} strokeWidth="1.6" strokeLinecap="round" />
      ))}
    </>
  ),
  tet: () => (
    <>
      <path d="M0 2 Q32 9 64 2 T128 2 T192 2 T256 2" fill="none" stroke={c("rose-vivid")} strokeWidth="1.2" />
      {[0, 1, 2, 3].map((i) => {
        const x = 32 + i * 64;
        return i % 2 === 0 ? (
          <g key={i} transform={`translate(${x} 6)`}>
            <line x1="0" y1="0" x2="0" y2="2" stroke={c("rose-vivid")} />
            <rect x="-3" y="2" width="6" height="1.6" fill={c("fx-gold")} />
            <ellipse cx="0" cy="8" rx="5.5" ry="5" fill={c("rose-vivid")} />
            <rect x="-3" y="12.4" width="6" height="1.6" fill={c("fx-gold")} />
            <line x1="0" y1="14" x2="0" y2="18" stroke={c("fx-gold")} strokeWidth="1.2" />
          </g>
        ) : (
          <polygon key={i} points={`${x - 7},5 ${x + 7},5 ${x},16`} fill={c("fx-gold")} />
        );
      })}
    </>
  ),
  halloween: () => (
    <>
      <path d="M0 2 Q32 8 64 2 T128 2 T192 2 T256 2" fill="none" stroke={c("ink-faint")} strokeWidth="1" />
      {[0, 1, 2, 3].map((i) => {
        const x = 32 + i * 64;
        return <polygon key={i} points={`${x - 8},4 ${x + 8},4 ${x},18`} fill={c(i % 2 ? "violet-vivid" : "orange-vivid")} opacity="0.9" />;
      })}
    </>
  ),
  valentine: () => (
    <>
      <path d="M0 2 Q32 9 64 2 T128 2 T192 2 T256 2" fill="none" stroke={c("fx-heart")} strokeWidth="1" opacity="0.7" />
      {[0, 1, 2, 3].map((i) => (
        <path key={i} transform={`translate(${32 + i * 64} 11) scale(${i % 2 ? 0.55 : 0.75})`} d="M0 7 C-10 -1 -6 -10 0 -5 C6 -10 10 -1 0 7 Z" fill={c(i % 2 ? "fx-petal-2" : "fx-heart")} />
      ))}
    </>
  ),
  midautumn: () => (
    <>
      <path d="M0 2 Q32 9 64 2 T128 2 T192 2 T256 2" fill="none" stroke={c("ink-faint")} strokeWidth="1" />
      {[0, 1, 2, 3].map((i) => (
        <g key={i} transform={`translate(${32 + i * 64} 7)`}>
          <ellipse cx="0" cy="5" rx="5" ry="4.5" fill={c(["rose-vivid", "amber-vivid", "orange-vivid", "fx-gold"][i])} opacity="0.95" />
          <line x1="0" y1="9.5" x2="0" y2="14" stroke={c("fx-gold")} strokeWidth="1.1" />
        </g>
      ))}
    </>
  ),
};

/** Garland across the top of the main area on holidays. */
export function Garland({ theme }: { theme: ThemeId }) {
  const id = useId().replace(/:/g, "");
  const art = GARLAND[theme as HolidayId];
  if (!art) return null;
  return (
    <svg aria-hidden="true" className="pointer-events-none absolute inset-x-0 top-0 z-[24] h-[22px] w-full print:hidden" preserveAspectRatio="none">
      <defs>
        <pattern id={`g-${id}`} width="256" height="22" patternUnits="userSpaceOnUse">
          {art(0)}
        </pattern>
      </defs>
      <rect width="100%" height="22" fill={`url(#g-${id})`} />
    </svg>
  );
}

// ---------- Sidebar scene ----------

const ground = (fill: string, opacity = 1) => <path d="M0 50 Q54 44 108 48 T216 46 V56 H0 Z" fill={c(fill)} opacity={opacity} />;

const SCENE: Record<ThemeId, ReactNode> = {
  christmas: (
    <>
      {ground("fx-snow", 0.8)}
      <polygon points="40,10 52,26 28,26" fill={c("green-vivid")} />
      <polygon points="40,18 56,36 24,36" fill={c("green-vivid")} />
      <polygon points="40,28 60,46 20,46" fill={c("green-vivid")} />
      <rect x="37" y="45" width="6" height="5" fill={c("orange")} />
      <circle cx="40" cy="9" r="2.6" fill={c("fx-gold")} />
      {[[34, 24], [46, 32], [30, 40], [50, 42]].map(([x, y]) => <circle key={`${x}${y}`} cx={x} cy={y} r="1.6" fill={c(x % 4 ? "rose-vivid" : "fx-gold")} />)}
      <rect x="70" y="38" width="12" height="10" rx="1" fill={c("rose-vivid")} />
      <rect x="75" y="38" width="2" height="10" fill={c("fx-gold")} />
      <rect x="86" y="42" width="9" height="6" rx="1" fill={c("blue-vivid")} />
      <rect x="89.5" y="42" width="2" height="6" fill={c("fx-gold")} />
    </>
  ),
  winter: (
    <>
      {ground("fx-snow", 0.8)}
      <circle cx="40" cy="40" r="9" fill={c("fx-white")} stroke={c("fx-snow")} strokeWidth="1.5" />
      <circle cx="40" cy="26" r="6.5" fill={c("fx-white")} stroke={c("fx-snow")} strokeWidth="1.5" />
      <rect x="34" y="16" width="12" height="3" rx="1" fill={c("ink-2")} />
      <rect x="36.5" y="10" width="7" height="7" rx="1" fill={c("ink-2")} />
      <circle cx="38" cy="25" r="0.9" fill={c("ink")} />
      <circle cx="42" cy="25" r="0.9" fill={c("ink")} />
      <polygon points="40,27 46,28 40,29" fill={c("orange-vivid")} />
      <path d="M33 32 Q40 35 47 32" stroke={c("rose-vivid")} strokeWidth="2.4" fill="none" />
      <path d="M31 38 L22 32 M49 38 L58 31" stroke={c("orange")} strokeWidth="1.3" />
      <polygon points="88,48 96,34 104,48" fill={c("green-vivid")} opacity="0.55" />
      <polygon points="100,48 110,30 120,48" fill={c("green-vivid")} opacity="0.75" />
    </>
  ),
  tet: (
    <>
      <path d="M6 50 Q30 30 52 22 Q70 16 84 6" stroke={c("orange")} strokeWidth="2" fill="none" strokeLinecap="round" />
      <path d="M40 27 Q46 36 44 44" stroke={c("orange")} strokeWidth="1.4" fill="none" />
      {[[20, 40], [34, 30], [52, 22], [70, 14], [44, 42], [84, 8]].map(([x, y]) => (
        <g key={`${x}${y}`} transform={`translate(${x} ${y})`}>
          {[0, 72, 144, 216, 288].map((a) => <ellipse key={a} cx="0" cy="-2.4" rx="1.7" ry="2.5" fill={c("fx-mai")} transform={`rotate(${a})`} />)}
          <circle r="1" fill={c("orange")} />
        </g>
      ))}
      <g transform="translate(118 8)">
        <line x1="0" y1="-8" x2="0" y2="2" stroke={c("rose-vivid")} />
        <ellipse cx="0" cy="9" rx="8" ry="7.5" fill={c("rose-vivid")} />
        <path d="M-8 9 H8" stroke={c("fx-gold")} />
        <line x1="0" y1="16" x2="0" y2="24" stroke={c("fx-gold")} strokeWidth="1.4" />
      </g>
      <g transform="translate(150 34) rotate(-8)">
        <rect x="-8" y="-6" width="16" height="20" rx="2" fill={c("rose-vivid")} />
        <circle cx="0" cy="4" r="3.5" fill={c("fx-gold")} />
      </g>
    </>
  ),
  midautumn: (
    <>
      <circle cx="170" cy="18" r="12" fill={c("fx-gold")} opacity="0.85" />
      <g transform="translate(44 26)">
        <line x1="0" y1="-26" x2="0" y2="-14" stroke={c("ink-faint")} />
        <polygon points="0,-14 3.5,-4 14,-4 5.5,2 9,12 0,6 -9,12 -5.5,2 -14,-4 -3.5,-4" fill={c("rose-vivid")} />
        <line x1="0" y1="10" x2="0" y2="20" stroke={c("fx-gold")} strokeWidth="1.4" />
      </g>
      <g transform="translate(96 42)">
        <circle r="9" fill={c("orange")} />
        <circle r="6" fill="none" stroke={c("fx-gold")} strokeWidth="1.2" />
        <path d="M-3 0 H3 M0 -3 V3" stroke={c("fx-gold")} strokeWidth="1.2" />
      </g>
    </>
  ),
  halloween: (
    <>
      {ground("orange", 0.25)}
      {[[40, 42, 10], [62, 45, 7]].map(([x, y, r]) => (
        <g key={x}>
          <ellipse cx={x} cy={y} rx={r * 1.25} ry={r} fill={c("orange-vivid")} />
          <ellipse cx={x} cy={y} rx={r * 0.5} ry={r} fill={c("orange")} opacity="0.35" />
          <rect x={x - 1} y={y - r - 4} width="2" height="5" fill={c("green-vivid")} />
        </g>
      ))}
      <path d="M110 46 V24 Q110 10 122 10 Q134 10 134 24 V46 L130 42 L126 46 L122 42 L118 46 L114 42 Z" fill={c("fx-white")} stroke={c("ink-faint")} strokeWidth="1.2" />
      <circle cx="118" cy="22" r="1.6" fill={c("ink")} />
      <circle cx="126" cy="22" r="1.6" fill={c("ink")} />
      <ellipse cx="122" cy="30" rx="2.2" ry="3" fill={c("ink")} />
    </>
  ),
  valentine: (
    <>
      {[[40, 30, 1.4, "fx-heart"], [70, 20, 0.9, "fx-petal-2"], [96, 34, 1.1, "fx-heart"], [130, 24, 0.7, "fx-petal"]].map(([x, y, s, col]) => (
        <path key={`${x}`} transform={`translate(${x} ${y}) scale(${s})`} d="M0 7 C-10 -1 -6 -10 0 -5 C6 -10 10 -1 0 7 Z" fill={c(col as string)} />
      ))}
      <g transform="translate(160 38)">
        <rect x="-14" y="-9" width="28" height="18" rx="2" fill={c("fx-white")} stroke={c("fx-heart")} />
        <path d="M-14 -9 L0 2 L14 -9" fill="none" stroke={c("fx-heart")} />
      </g>
    </>
  ),
  womensday: (
    <>
      <path d="M30 56 L34 40 H58 L62 56 Z" fill={c("orange")} opacity="0.8" />
      {[[38, 18, "fx-heart"], [46, 12, "fx-petal-2"], [54, 18, "rose-vivid"]].map(([x, y, col]) => (
        <g key={`${x}`}>
          <path d={`M${x} ${(y as number) + 8} Q46 30 46 40`} stroke={c("green-vivid")} strokeWidth="1.4" fill="none" />
          <path d={`M${(x as number) - 5} ${y} Q${(x as number) - 6} ${(y as number) + 9} ${x} ${(y as number) + 9} Q${(x as number) + 6} ${(y as number) + 9} ${(x as number) + 5} ${y} L${(x as number) + 2} ${(y as number) + 3} L${x} ${(y as number) - 1} L${(x as number) - 2} ${(y as number) + 3} Z`} fill={c(col as string)} />
        </g>
      ))}
    </>
  ),
  newyear: (
    <>
      {Array.from({ length: 14 }, (_, i) => (
        <rect key={i} x={12 + i * 14} y={8 + ((i * 7) % 30)} width="4" height="2" rx="0.5" fill={c(["fx-gold", "rose-vivid", "blue-vivid", "green-vivid"][i % 4])} transform={`rotate(${i * 37} ${14 + i * 14} ${9 + ((i * 7) % 30)})`} />
      ))}
    </>
  ),
  spring: (
    <>
      {ground("green-vivid", 0.35)}
      {[[30, 34, "fx-petal"], [56, 38, "fx-mai"], [84, 32, "fx-petal-2"], [150, 36, "fx-petal"]].map(([x, y, col]) => (
        <g key={`${x}`}>
          <path d={`M${x} ${y} V50`} stroke={c("green-vivid")} strokeWidth="1.4" />
          <g transform={`translate(${x} ${y})`}>
            {[0, 72, 144, 216, 288].map((a) => <ellipse key={a} cx="0" cy="-3" rx="2.2" ry="3.2" fill={c(col as string)} transform={`rotate(${a})`} />)}
            <circle r="1.4" fill={c("fx-gold")} />
          </g>
        </g>
      ))}
      <g transform="translate(118 18)" className="motion-safe:animate-[fx-sway_3s_ease-in-out_infinite]">
        <path d="M0 0 Q-8 -8 -10 0 Q-8 6 0 2 Q8 6 10 0 Q8 -8 0 0" fill={c("blue-vivid")} opacity="0.85" />
        <line x1="0" y1="-3" x2="0" y2="4" stroke={c("ink-2")} strokeWidth="1" />
      </g>
    </>
  ),
  summer: (
    <>
      <path d="M0 48 Q54 42 108 47 T216 45 V56 H0 Z" fill={c("amber-vivid")} opacity="0.3" />
      <path d="M0 52 q12 -4 24 0 t24 0 t24 0 t24 0 t24 0 t24 0 t24 0 t24 0 t24 0" stroke={c("cyan-vivid")} strokeWidth="1.6" fill="none" opacity="0.7" />
      <circle cx="180" cy="14" r="8" fill={c("fx-gold")} />
      <path d="M60 46 Q58 26 66 12" stroke={c("orange")} strokeWidth="2.4" fill="none" strokeLinecap="round" />
      {[-60, -20, 20, 60, 100].map((a) => (
        <path key={a} d="M66 12 q10 -2 18 6" stroke={c("green-vivid")} strokeWidth="3" fill="none" strokeLinecap="round" transform={`rotate(${a} 66 12)`} />
      ))}
    </>
  ),
  autumn: (
    <>
      {[[30, 48, "fx-leaf-1", 20], [44, 50, "fx-leaf-2", -30], [58, 47, "fx-leaf-3", 50], [38, 44, "fx-leaf-3", -60], [52, 44, "fx-leaf-1", 10], [168, 50, "fx-leaf-2", 35]].map(([x, y, col, r]) => (
        <ellipse key={`${x}${y}`} cx={x} cy={y} rx="7" ry="3.5" fill={c(col as string)} transform={`rotate(${r} ${x} ${y})`} />
      ))}
      <g transform="translate(110 40)">
        <path d="M-9 0 Q0 -14 9 0 Z" fill={c("rose-vivid")} />
        <rect x="-2.5" y="0" width="5" height="9" rx="1.5" fill={c("fx-white")} stroke={c("ink-faint")} strokeWidth="0.8" />
        <circle cx="-3" cy="-5" r="1.2" fill={c("fx-white")} />
        <circle cx="3" cy="-3" r="1" fill={c("fx-white")} />
      </g>
    </>
  ),
};

/** A small scene at the foot of the expanded sidebar. */
export function SidebarScene({ theme }: { theme: ThemeId }) {
  return (
    <svg viewBox="0 0 216 56" aria-hidden="true" className="pointer-events-none block h-14 w-full shrink-0 opacity-90">
      {SCENE[theme]}
    </svg>
  );
}

// ---------- Snow cap ----------

/** Snow resting on the top edge of a banner (winter and Christmas). A thin snow-blue rim and a
 *  soft shadow under the drifts keep it apart from a light page background. */
const SNOW_DRIFT = "Q388 12 376 7 T352 8 Q340 14 326 7 T300 6 Q288 13 272 7 T244 8 Q232 13 220 6 T196 7 Q182 14 168 7 T140 6 Q128 12 114 7 T88 8 Q74 14 60 7 T34 6 Q22 12 10 7 T0 7";

export function SnowCap() {
  return (
    <svg aria-hidden="true" viewBox="0 0 400 16" preserveAspectRatio="none" className="pointer-events-none absolute inset-x-0 top-0 h-4 w-full">
      {/* shadow under the drifts */}
      <path d={`M0 0 H400 V8 ${SNOW_DRIFT} Z`} transform="translate(0 1.6)" fill="var(--fx-snow)" opacity="0.55" />
      <path d={`M0 0 H400 V6 ${SNOW_DRIFT} Z`} fill="var(--fx-white)" />
      {/* rim: the banner's top edge and the lower edge of the drifts */}
      <path d={`M400 6 ${SNOW_DRIFT}`} fill="none" stroke="var(--fx-snow)" strokeWidth="1" vectorEffect="non-scaling-stroke" />
      <line x1="0" y1="0.5" x2="400" y2="0.5" stroke="var(--fx-snow)" strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}
