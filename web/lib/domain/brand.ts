/**
 * Workspace brand colour → full theme. One #RRGGBB value drives every accent token in the UI
 * (light + dark) and the --doc-* tokens used by the PDF/print pages. Contrast is enforced here so
 * any colour — pale yellow, near-black, neon — still yields readable text (WCAG AA, ≥ 4.5:1).
 */

export const BRAND_HEX = /^#[0-9A-Fa-f]{6}$/;
export const BRAND_PRESETS = ["#4F46E5", "#2563EB", "#0891B2", "#059669", "#65A30D", "#D97706", "#EA580C", "#C8102E", "#DB2777", "#7C3AED", "#475569"];

type Rgb = [number, number, number];
interface Hsl { h: number; s: number; l: number }

const clamp = (v: number, lo = 0, hi = 1) => Math.min(hi, Math.max(lo, v));

function hexToRgb(hex: string): Rgb {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function rgbToHsl([r, g, b]: Rgb): Hsl {
  const R = r / 255, G = g / 255, B = b / 255;
  const max = Math.max(R, G, B), min = Math.min(R, G, B), d = max - min;
  const l = (max + min) / 2;
  if (d === 0) return { h: 0, s: 0, l };
  const s = d / (1 - Math.abs(2 * l - 1));
  let h = max === R ? ((G - B) / d) % 6 : max === G ? (B - R) / d + 2 : (R - G) / d + 4;
  h = (h * 60 + 360) % 360;
  return { h, s, l };
}

function hslToRgb({ h, s, l }: Hsl): Rgb {
  const c = (1 - Math.abs(2 * l - 1)) * s;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = l - c / 2;
  const [r, g, b] = h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x] : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x];
  return [Math.round((r + m) * 255), Math.round((g + m) * 255), Math.round((b + m) * 255)];
}

const toHex = (rgb: Rgb) => `#${rgb.map((v) => v.toString(16).padStart(2, "0")).join("").toUpperCase()}`;
const hsl = (h: number, s: number, l: number): Hsl => ({ h: (h + 360) % 360, s: clamp(s), l: clamp(l) });
const hex = (c: Hsl) => toHex(hslToRgb(c));

function luminance([r, g, b]: Rgb): number {
  const f = (v: number) => { const c = v / 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
}

export function contrast(a: string, b: string): number {
  const la = luminance(hexToRgb(a)), lb = luminance(hexToRgb(b));
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

/** Move lightness away from `bg` until `c` reaches `min` contrast against it. */
function ensure(c: Hsl, bg: string, min: number): Hsl {
  const darker = luminance(hexToRgb(bg)) > 0.4;
  let cur = c;
  for (let i = 0; i < 100 && contrast(hex(cur), bg) < min; i++) cur = { ...cur, l: clamp(cur.l + (darker ? -0.01 : 0.01)) };
  return cur;
}

const rgba = (c: string, a: number) => { const [r, g, b] = hexToRgb(c); return `rgba(${r}, ${g}, ${b}, ${a})`; };
const WHITE = "#FFFFFF";
const NEAR_BLACK = "#111827";

export function normalizeBrand(v?: string | null): string | null {
  return v && BRAND_HEX.test(v) ? v.toUpperCase() : null;
}

/** Accent tokens for the app UI (light theme). */
function lightTokens(base: Hsl): Record<string, string> {
  const baseHex = hex(base);
  // Keep the picked colour when white text is readable on it; otherwise prefer dark text on a
  // bright colour, and only darken the colour as a last resort.
  let accent = base;
  let on = WHITE;
  if (contrast(baseHex, WHITE) < 4.5) {
    if (contrast(baseHex, NEAR_BLACK) >= 4.5) on = NEAR_BLACK;
    else accent = ensure(base, WHITE, 4.5);
  }
  const accentHex = hex(accent);
  const hover = hsl(accent.h, accent.s, accent.l + (on === WHITE ? -0.07 : -0.06));
  const soft = hsl(accent.h, Math.min(accent.s, 0.9), 0.95);
  const ink = ensure(ensure(hsl(accent.h, accent.s, Math.min(accent.l, 0.45)), WHITE, 5), hex(soft), 4.6);
  const border = hsl(accent.h, Math.min(accent.s, 0.85), 0.84);
  const accent2 = hsl(accent.h + 28, accent.s, accent.l);
  return {
    "--accent": accentHex,
    "--accent-2": hex(accent2),
    "--accent-hover": hex(hover),
    "--accent-ink": hex(ink),
    "--accent-soft": hex(soft),
    "--accent-border": hex(border),
    "--accent-on": on,
    "--accent-grad": `linear-gradient(135deg, ${accentHex} 0%, ${hex(accent2)} 100%)`,
    "--accent-grad-hover": `linear-gradient(135deg, ${hex(hover)} 0%, ${hex(hsl(accent2.h, accent2.s, accent2.l - 0.07))} 100%)`,
    "--sh-accent": `0 4px 12px ${rgba(accentHex, 0.32)}`,
  };
}

/** Accent tokens for the dark theme: lighter accent, dark tinted surfaces. */
function darkTokens(base: Hsl): Record<string, string> {
  const s = clamp(base.s, 0.3, 0.95);
  const accent = hsl(base.h, s, clamp(Math.max(base.l, 0.68), 0.68, 0.8));
  const accentHex = hex(accent);
  const accent2 = hsl(accent.h + 28, s, accent.l);
  const hoverC = hsl(accent.h, s, accent.l + 0.07);
  return {
    "--accent": accentHex,
    "--accent-2": hex(accent2),
    "--accent-hover": hex(hoverC),
    "--accent-ink": hex(hsl(accent.h, s, 0.8)),
    "--accent-soft": hex(hsl(accent.h, s * 0.5, 0.22)),
    "--accent-border": hex(hsl(accent.h, s * 0.45, 0.38)),
    "--accent-on": hex(hsl(accent.h, 0.4, 0.12)),
    "--accent-grad": `linear-gradient(135deg, ${accentHex} 0%, ${hex(accent2)} 100%)`,
    "--accent-grad-hover": `linear-gradient(135deg, ${hex(hoverC)} 0%, ${hex(hsl(accent2.h, s, accent2.l + 0.06))} 100%)`,
    "--sh-accent": `0 4px 14px ${rgba(accentHex, 0.28)}`,
  };
}

/** Tokens for printable documents (white paper, brand-coloured header with white text). */
export function docTokens(color?: string | null): Record<string, string> | undefined {
  const c = normalizeBrand(color);
  if (!c) return undefined;
  const base = rgbToHsl(hexToRgb(c));
  const brand = ensure(base, WHITE, 4.5);
  const s = base.s;
  return {
    "--doc-brand": hex(brand),
    "--doc-brand-2": hex(hsl(brand.h + 6, brand.s, brand.l + 0.07)),
    "--doc-tint": hex(hsl(base.h, clamp(s, 0, 0.9), 0.97)),
    "--doc-tint-line": hex(hsl(base.h, clamp(s * 0.6, 0, 0.7), 0.88)),
    "--doc-row-alt": hex(hsl(base.h, clamp(s * 0.6, 0, 0.7), 0.975)),
    "--doc-line": hex(hsl(base.h, clamp(s * 0.2, 0, 0.2), 0.92)),
  };
}

const decl = (t: Record<string, string>) => Object.entries(t).map(([k, v]) => `${k}:${v}`).join(";");

/** Stylesheet text overriding the accent tokens for both themes. Empty string = default theme. */
export function brandCss(color?: string | null): string {
  const c = normalizeBrand(color);
  if (!c) return "";
  const base = rgbToHsl(hexToRgb(c));
  return `html:root{${decl(lightTokens(base))}}\nhtml:root[data-theme="dark"]{${decl(darkTokens(base))}}`;
}

/** Inline CSS variables to preview a colour (UI accent + document header) in a small box. */
export function previewTokens(color?: string | null): Record<string, string> {
  const c = normalizeBrand(color);
  if (!c) return {};
  return { ...lightTokens(rgbToHsl(hexToRgb(c))), ...(docTokens(c) ?? {}) };
}
