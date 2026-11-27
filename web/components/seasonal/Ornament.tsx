import type { ReactNode } from "react";
import type { ThemeId } from "@/lib/domain/season";

/**
 * Small illustrations for the home hero banner, one per festive theme. They sit on the
 * workspace's brand colour, so they are drawn in white (currentColor) with gold highlights
 * (--fx-gold), which reads on any brand colour. Gentle motion is opt-in via motion-safe.
 */

const W = "currentColor";
const GOLD = "var(--fx-gold)";

function Snowflake({ x, y, r, o = 1 }: { x: number; y: number; r: number; o?: number }) {
  const arms = [0, 60, 120, 180, 240, 300];
  return (
    <g transform={`translate(${x} ${y})`} stroke={W} strokeWidth={r / 7} strokeLinecap="round" opacity={o}>
      {arms.map((a) => (
        <g key={a} transform={`rotate(${a})`}>
          <line x1="0" y1="0" x2="0" y2={-r} />
          <line x1="0" y1={-r * 0.55} x2={-r * 0.25} y2={-r * 0.8} />
          <line x1="0" y1={-r * 0.55} x2={r * 0.25} y2={-r * 0.8} />
        </g>
      ))}
    </g>
  );
}

function Star({ x, y, r, fill }: { x: number; y: number; r: number; fill: string }) {
  const pts = Array.from({ length: 10 }, (_, i) => {
    const a = (Math.PI / 5) * i - Math.PI / 2;
    const rr = i % 2 === 0 ? r : r * 0.45;
    return `${(x + Math.cos(a) * rr).toFixed(1)},${(y + Math.sin(a) * rr).toFixed(1)}`;
  }).join(" ");
  return <polygon points={pts} fill={fill} />;
}

function Blossom({ x, y, r, fill, center = GOLD }: { x: number; y: number; r: number; fill: string; center?: string }) {
  return (
    <g transform={`translate(${x} ${y})`}>
      {[0, 72, 144, 216, 288].map((a) => (
        <ellipse key={a} cx="0" cy={-r * 0.55} rx={r * 0.38} ry={r * 0.58} fill={fill} transform={`rotate(${a})`} />
      ))}
      <circle r={r * 0.22} fill={center} />
    </g>
  );
}

function Heart({ x, y, s, fill, stroke }: { x: number; y: number; s: number; fill?: string; stroke?: string }) {
  return (
    <path
      transform={`translate(${x} ${y}) scale(${s})`}
      d="M0 7 C-10 -1 -6 -10 0 -5 C6 -10 10 -1 0 7 Z"
      fill={fill ?? "none"}
      stroke={stroke}
      strokeWidth={stroke ? 1.6 / s : undefined}
    />
  );
}

function Lantern({ x, y, o = 1 }: { x: number; y: number; o?: number }) {
  return (
    <g opacity={o} className="origin-top motion-safe:animate-[fx-sway_5s_ease-in-out_infinite]" style={{ transformOrigin: `${x}px 0px` }}>
      <line x1={x} y1="0" x2={x} y2={y - 14} stroke={W} strokeWidth="1.2" opacity="0.7" />
      <rect x={x - 7} y={y - 16} width="14" height="4" rx="1" fill={GOLD} />
      <ellipse cx={x} cy={y} rx="15" ry="13" fill={W} />
      <path d={`M${x - 15} ${y} H${x + 15} M${x - 13} ${y - 6} H${x + 13} M${x - 13} ${y + 6} H${x + 13}`} stroke={GOLD} strokeWidth="1.2" opacity="0.9" />
      <rect x={x - 7} y={y + 12} width="14" height="4" rx="1" fill={GOLD} />
      <path d={`M${x - 3} ${y + 16} L${x - 4} ${y + 28} M${x} ${y + 16} V${y + 30} M${x + 3} ${y + 16} L${x + 4} ${y + 28}`} stroke={GOLD} strokeWidth="1.4" strokeLinecap="round" />
    </g>
  );
}

const ART: Record<ThemeId, ReactNode> = {
  christmas: (
    <g>
      <ellipse cx="70" cy="92" rx="46" ry="5" fill={W} opacity="0.35" />
      <rect x="64" y="78" width="12" height="12" rx="2" fill={W} opacity="0.75" />
      <polygon points="70,14 92,44 48,44" fill={W} />
      <polygon points="70,30 98,62 42,62" fill={W} />
      <polygon points="70,46 104,80 36,80" fill={W} />
      <path d="M52 44 Q70 52 88 42 M47 62 Q70 72 94 60" stroke={GOLD} strokeWidth="1.6" fill="none" />
      {[[58, 38], [80, 54], [54, 70], [86, 74], [70, 60]].map(([cx, cy]) => (
        <circle key={`${cx}-${cy}`} cx={cx} cy={cy} r="2.8" fill={GOLD} />
      ))}
      <g className="origin-center motion-safe:animate-[fx-twinkle_2.4s_ease-in-out_infinite]" style={{ transformOrigin: "70px 12px" }}>
        <Star x={70} y={12} r={8} fill={GOLD} />
      </g>
      <Snowflake x={120} y={22} r={7} o={0.7} />
      <Snowflake x={22} y={34} r={5} o={0.55} />
    </g>
  ),
  winter: (
    <g>
      <Snowflake x={84} y={42} r={26} />
      <Snowflake x={34} y={30} r={13} o={0.75} />
      <Snowflake x={124} y={78} r={10} o={0.6} />
      <Snowflake x={40} y={80} r={7} o={0.5} />
    </g>
  ),
  autumn: (
    <g>
      <g transform="translate(78 46) rotate(-18)">
        <path
          d="M0 -38 L7 -22 L20 -28 L16 -12 L34 -12 L24 0 L30 6 L12 8 L14 20 L2 14 L0 34 L-2 14 L-14 20 L-12 8 L-30 6 L-24 0 L-34 -12 L-16 -12 L-20 -28 L-7 -22 Z"
          fill={W}
        />
        <path d="M0 34 V-30 M0 0 L20 -14 M0 0 L-20 -14 M0 10 L12 16 M0 10 L-12 16" stroke={GOLD} strokeWidth="1.4" opacity="0.8" fill="none" />
      </g>
      <ellipse cx="28" cy="72" rx="10" ry="5" fill={GOLD} transform="rotate(-35 28 72)" opacity="0.85" />
      <ellipse cx="126" cy="20" rx="8" ry="4" fill={W} transform="rotate(25 126 20)" opacity="0.6" />
    </g>
  ),
  spring: (
    <g>
      <path d="M6 22 Q50 6 92 26 T138 30" stroke={W} strokeWidth="3" fill="none" strokeLinecap="round" opacity="0.8" />
      <path d="M60 14 Q66 30 58 44" stroke={W} strokeWidth="2" fill="none" strokeLinecap="round" opacity="0.7" />
      <Blossom x={34} y={18} r={11} fill={W} />
      <Blossom x={76} y={20} r={14} fill={W} />
      <Blossom x={58} y={46} r={9} fill={W} />
      <Blossom x={118} y={30} r={10} fill={W} />
      <circle cx="98" cy="16" r="3" fill={W} opacity="0.8" />
      <ellipse cx="100" cy="70" rx="5" ry="3" fill={W} transform="rotate(30 100 70)" opacity="0.6" />
      <ellipse cx="44" cy="80" rx="4" ry="2.5" fill={W} transform="rotate(-20 44 80)" opacity="0.5" />
    </g>
  ),
  summer: (
    <g>
      <g className="motion-safe:animate-[spin_40s_linear_infinite]" style={{ transformOrigin: "86px 38px" }}>
        {Array.from({ length: 12 }, (_, i) => (
          <line key={i} x1="86" y1="10" x2="86" y2="2" stroke={GOLD} strokeWidth="2.6" strokeLinecap="round" transform={`rotate(${i * 30} 86 38)`} />
        ))}
      </g>
      <circle cx="86" cy="38" r="20" fill={GOLD} />
      <path d="M14 78 q8 -6 16 0 t16 0 t16 0 t16 0 t16 0 t16 0 t16 0" stroke={W} strokeWidth="2.2" fill="none" strokeLinecap="round" opacity="0.8" />
      <path d="M30 90 q8 -6 16 0 t16 0 t16 0 t16 0 t16 0" stroke={W} strokeWidth="2" fill="none" strokeLinecap="round" opacity="0.55" />
    </g>
  ),
  tet: (
    <g>
      <Lantern x={42} y={44} />
      <Lantern x={98} y={30} o={0.9} />
      <Blossom x={128} y={72} r={10} fill={GOLD} center={W} />
      <Blossom x={70} y={82} r={7} fill={GOLD} center={W} />
      <Blossom x={14} y={84} r={6} fill={W} />
    </g>
  ),
  midautumn: (
    <g>
      <circle cx="98" cy="36" r="26" fill={GOLD} opacity="0.95" />
      <circle cx="90" cy="30" r="4" fill={W} opacity="0.35" />
      <circle cx="106" cy="46" r="3" fill={W} opacity="0.3" />
      <g className="motion-safe:animate-[fx-sway_5s_ease-in-out_infinite]" style={{ transformOrigin: "40px 0px" }}>
        <line x1="40" y1="0" x2="40" y2="36" stroke={W} strokeWidth="1.2" opacity="0.7" />
        <Star x={40} y={54} r={20} fill={W} />
        <Star x={40} y={55} r={8} fill={GOLD} />
        <path d="M37 74 L36 86 M40 74 V88 M43 74 L44 86" stroke={GOLD} strokeWidth="1.4" strokeLinecap="round" />
      </g>
    </g>
  ),
  halloween: (
    <g>
      <ellipse cx="76" cy="66" rx="16" ry="20" fill={W} opacity="0.95" />
      <ellipse cx="62" cy="66" rx="14" ry="19" fill={W} opacity="0.85" />
      <ellipse cx="90" cy="66" rx="14" ry="19" fill={W} opacity="0.85" />
      <path d="M76 46 q2 -8 8 -10" stroke={GOLD} strokeWidth="3" fill="none" strokeLinecap="round" />
      <path d="M64 62 l6 -6 l6 6 z M80 62 l6 -6 l6 6 z" fill={GOLD} />
      <path d="M64 74 q12 9 24 0 l-4 3 l-4 -3 l-4 3 l-4 -3 l-4 3 z" fill={GOLD} />
      <g className="motion-safe:animate-[fx-sway_3s_ease-in-out_infinite]" style={{ transformOrigin: "30px 24px" }}>
        <path d="M30 24 q-6 -8 -16 -4 q4 2 4 6 q4 -2 6 2 q2 -4 6 -4 q4 0 6 4 q2 -4 6 -2 q0 -4 4 -6 q-10 -4 -16 4 z" fill={W} />
      </g>
      <circle cx="122" cy="20" r="9" fill={GOLD} opacity="0.9" />
    </g>
  ),
  valentine: (
    <g>
      <g className="motion-safe:animate-[fx-twinkle_2.6s_ease-in-out_infinite]" style={{ transformOrigin: "80px 46px" }}>
        <Heart x={80} y={46} s={3.2} fill={W} />
      </g>
      <Heart x={40} y={30} s={1.7} stroke={GOLD} />
      <Heart x={122} y={76} s={1.3} fill={GOLD} />
      <Heart x={30} y={78} s={1} fill={W} />
    </g>
  ),
  womensday: (
    <g>
      {[[58, 30, -14], [78, 22, 0], [98, 32, 14]].map(([x, y, a]) => (
        <g key={x} transform={`rotate(${a} ${x} 88)`}>
          <path d={`M${x} ${y + 12} V88`} stroke={W} strokeWidth="2.2" opacity="0.8" />
          <path d={`M${x - 9} ${y} Q${x - 10} ${y + 14} ${x} ${y + 14} Q${x + 10} ${y + 14} ${x + 9} ${y} L${x + 4} ${y + 5} L${x} ${y - 2} L${x - 4} ${y + 5} Z`} fill={W} />
        </g>
      ))}
      <path d="M66 74 q12 8 24 0 l-4 10 h-16 z" fill={GOLD} />
      <Heart x={124} y={24} s={1.1} fill={GOLD} />
    </g>
  ),
  newyear: (
    <g>
      {[[88, 34, 24, GOLD], [40, 26, 15, W], [124, 70, 12, W]].map(([cx, cy, r, c]) => (
        <g key={`${cx}`} className="motion-safe:animate-[fx-twinkle_2.2s_ease-in-out_infinite]" style={{ transformOrigin: `${cx}px ${cy}px` }}>
          {Array.from({ length: 12 }, (_, i) => (
            <line
              key={i}
              x1={cx as number}
              y1={(cy as number) - (r as number) * 0.35}
              x2={cx as number}
              y2={(cy as number) - (r as number)}
              stroke={c as string}
              strokeWidth="2"
              strokeLinecap="round"
              transform={`rotate(${i * 30} ${cx} ${cy})`}
            />
          ))}
        </g>
      ))}
      <Star x={56} y={70} r={6} fill={GOLD} />
      <Star x={20} y={60} r={4} fill={W} />
    </g>
  ),
};

export function Ornament({ theme, className = "" }: { theme: ThemeId; className?: string }) {
  return (
    <svg viewBox="0 0 140 100" width="140" height="100" aria-hidden="true" className={`text-white ${className}`}>
      {ART[theme]}
    </svg>
  );
}
